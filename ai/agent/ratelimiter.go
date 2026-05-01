// Package agent Agent 层
// 负责 Agent 工厂和初始化，所有 Agent 实例管理
package agent

import (
	"Keydd/ai/config"
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RateLimiter LLM 请求速率限制器
// 实现了 Sliding Window（时间窗口限流）+ Semaphore（并发控制）+ Retry（429 重试）
type RateLimiter struct {
	cfg config.RateLimitConfig

	// 并发控制信号量（限制同时 in-flight 的请求数）
	semaphore chan struct{}

	// 滑动窗口：保护 recentRequests 访问的锁
	mu             sync.Mutex
	recentRequests []time.Time // 最近请求的时间戳列表（自动过期）

	// 指标
	metrics *RateLimitMetrics

	// 注入的 tracer（可选）
	tracer trace.Tracer
}

// RateLimitMetrics 速率限制相关指标
type RateLimitMetrics struct {
	// RateLimitedCount: 遇到 429 的总次数（含重试后成功的）
	RateLimitedCount int64
	// Retried429Count: 429 触发重试的总次数（含最终成功和失败）
	Retried429Count int64
	// RetrySuccessCount: 重试后成功的次数
	RetrySuccessCount int64
	// RetryFailedCount: 重试次数耗尽后仍然失败的次数
	RetryFailedCount int64
	// AvgQueueWaitMs: 请求在速率限制器中的平均等待时间（毫秒）
	AvgQueueWaitMs int64
	// QueueWaitSamples: 采样次数（用于计算平均值）
	QueueWaitSamples int64
	// QueueWaitSumMs: 等待时间总和（毫秒）
	QueueWaitSumMs int64
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(cfg config.RateLimitConfig, tracer trace.Tracer) *RateLimiter {
	// 设置默认值
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	windowSecs := cfg.WindowSecs
	if windowSecs <= 0 {
		windowSecs = 30
	}
	maxInWindow := cfg.MaxInWindow
	if maxInWindow <= 0 {
		maxInWindow = 50
	}
	retryMaxAttempts := cfg.RetryMaxAttempts
	if retryMaxAttempts <= 0 {
		retryMaxAttempts = 3
	}
	retryBaseDelayMs := cfg.RetryBaseDelayMs
	if retryBaseDelayMs <= 0 {
		retryBaseDelayMs = 2000
	}

	rl := &RateLimiter{
		cfg: config.RateLimitConfig{
			MaxConcurrent:    maxConcurrent,
			WindowSecs:       windowSecs,
			MaxInWindow:      maxInWindow,
			RetryOn429:       cfg.RetryOn429,
			RetryMaxAttempts: retryMaxAttempts,
			RetryBaseDelayMs: retryBaseDelayMs,
		},
		semaphore:       make(chan struct{}, maxConcurrent),
		recentRequests: make([]time.Time, 0, maxInWindow),
		metrics:        &RateLimitMetrics{},
		tracer:         tracer,
	}

	return rl
}

// acquire 阻塞获取一个并发槽位（可响应 context 取消）
// 滑动窗口满时会等待前面的请求过期（轮询清理过期记录），直到有空间才继续
// 返回等待耗时（毫秒），ctx 取消时返回 -1
func (rl *RateLimiter) acquire(ctx context.Context) int64 {
	waitStart := time.Now()

	for {
		// Step 1: 滑动窗口检查（持锁，快速）
		rl.mu.Lock()
		cutoff := time.Now().Add(-time.Duration(rl.cfg.WindowSecs) * time.Second)
		j := 0
		for i := range rl.recentRequests {
			if rl.recentRequests[i].After(cutoff) {
				rl.recentRequests[j] = rl.recentRequests[i]
				j++
			}
		}
		rl.recentRequests = rl.recentRequests[:j]

		if len(rl.recentRequests) < rl.cfg.MaxInWindow {
			// 窗口有空间，记录本次请求时间
			rl.recentRequests = append(rl.recentRequests, time.Now())
			rl.mu.Unlock()

			// Step 2: 等待并发槽位（可响应 ctx 取消）
			select {
			case rl.semaphore <- struct{}{}:
			case <-ctx.Done():
				// 回滚滑动窗口记录
				rl.mu.Lock()
				if len(rl.recentRequests) > 0 {
					rl.recentRequests = rl.recentRequests[:len(rl.recentRequests)-1]
				}
				rl.mu.Unlock()
				return -1
			}

			waitMs := time.Since(waitStart).Milliseconds()
			// 更新平均等待时间
			rl.mu.Lock()
			rl.metrics.QueueWaitSumMs += waitMs
			rl.metrics.QueueWaitSamples++
			rl.mu.Unlock()
			return waitMs
		}

		// 窗口已满，释放锁，短暂等待让其他 goroutine 有机会释放
		rl.mu.Unlock()

		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			return -1
		case <-time.After(100 * time.Millisecond):
			// 继续下一次循环，检查窗口是否有空间
		}
	}
}

// Release 释放并发槽位（滑动窗口自动过期，无需手动清理）
func (rl *RateLimiter) Release() {
	<-rl.semaphore
}

// Execute 执行带速率限制的 LLM 调用
// llmCall 是实际的 LLM 调用函数，返回结果和错误
// 返回值包含：result、error、isRateLimited（是否为速率限制相关错误）
func (rl *RateLimiter) Execute(ctx context.Context, sessionID string, llmCall func(ctx context.Context) (string, error)) (string, error, bool) {
	waitMs := rl.acquire(ctx)
	if waitMs < 0 {
		// context 已取消
		return "", ctx.Err(), false
	}
	defer rl.Release()

	// 创建带 tracer 的子上下文
	var span trace.Span
	if rl.tracer != nil {
		_, span = rl.tracer.Start(ctx, "rate-limited-llm-call",
			trace.WithAttributes(
				attribute.Int64("rate_limit.queue_wait_ms", waitMs),
				attribute.Int("rate_limit.max_concurrent", rl.cfg.MaxConcurrent),
				attribute.Int("rate_limit.max_in_window", rl.cfg.MaxInWindow),
				attribute.Int("rate_limit.window_secs", rl.cfg.WindowSecs),
			),
		)
		if span != nil {
			ctx = trace.ContextWithSpan(ctx, span)
		}
	}
	if span != nil {
		defer span.End()
	}

	// 执行调用，支持重试
	return rl.executeWithRetry(ctx, sessionID, llmCall)
}

// executeWithRetry 执行带指数退避重试的 LLM 调用
func (rl *RateLimiter) executeWithRetry(ctx context.Context, sessionID string, llmCall func(ctx context.Context) (string, error)) (string, error, bool) {
	var lastErr error
	isRateLimited := false

	for attempt := 1; attempt <= rl.cfg.RetryMaxAttempts; attempt++ {
		result, err := llmCall(ctx)
		if err == nil {
			// 成功
			if attempt > 1 {
				rl.mu.Lock()
				rl.metrics.RetrySuccessCount++
				rl.mu.Unlock()
			}
			return result, nil, false
		}

		errMsg := err.Error()
		is429 := isRateLimitError(errMsg)

		if is429 {
			isRateLimited = true
			// 遇到 429 总是计入 RateLimitedCount（无论是否重试）
			rl.mu.Lock()
			rl.metrics.RateLimitedCount++
			rl.mu.Unlock()

			if !rl.cfg.RetryOn429 {
				// 不重试，直接返回
				rl.mu.Lock()
				rl.metrics.RetryFailedCount++
				rl.mu.Unlock()
				return "", err, true
			}

			if attempt < rl.cfg.RetryMaxAttempts {
				// 指数退避 + jitter
				delayMs := int64(rl.cfg.RetryBaseDelayMs * (1 << (attempt - 1)))
				// 防止 delayMs=0 时 rand.Int63n(0) panic
				if delayMs <= 0 {
					delayMs = 100
				}
				jitter := rand.Int63n(delayMs / 2)
				totalDelay := time.Duration(delayMs+jitter) * time.Millisecond

				rl.mu.Lock()
				rl.metrics.Retried429Count++
				rl.mu.Unlock()

				// 创建带超时的新上下文
				select {
				case <-ctx.Done():
					rl.mu.Lock()
					rl.metrics.RetryFailedCount++
					rl.mu.Unlock()
					return "", ctx.Err(), true
				case <-time.After(totalDelay):
					// 继续下一次重试
				}
				continue
			} else {
				// 重试次数耗尽
				rl.mu.Lock()
				rl.metrics.RetryFailedCount++
				rl.mu.Unlock()
				return "", err, true
			}
		} else {
			// 非 429 错误，不重试
			lastErr = err
			break
		}
	}

	return "", lastErr, isRateLimited
}

// GetMetrics 返回速率限制器指标快照
func (rl *RateLimiter) GetMetrics() RateLimitMetricsSnapshot {
	rl.mu.Lock()
	m := *rl.metrics
	windowCount := len(rl.recentRequests)
	rl.mu.Unlock()

	snap := RateLimitMetricsSnapshot{
		RateLimitedCount:  m.RateLimitedCount,
		Retried429Count:   m.Retried429Count,
		RetrySuccessCount: m.RetrySuccessCount,
		RetryFailedCount:  m.RetryFailedCount,
		AvgQueueWaitMs:    m.AvgQueueWaitMs,
		WindowCount:       int64(windowCount),
	}
	if m.QueueWaitSamples > 0 {
		snap.AvgQueueWaitMs = m.QueueWaitSumMs / m.QueueWaitSamples
	}
	return snap
}

// isRateLimitError 判断错误是否为速率限制错误
func isRateLimitError(errMsg string) bool {
	patterns := []string{
		"429",
		"rate limit",
		"rate_limit",
		"too many requests",
		"rate limit exceeded",
		"rate_limit_exceeded",
		"throttl",
		"Too Many Requests",
	}
	lower := strings.ToLower(errMsg)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// RateLimitMetricsSnapshot 速率限制器指标快照
type RateLimitMetricsSnapshot struct {
	// RateLimitedCount: 遇到 429 的总次数（含重试后成功的）
	RateLimitedCount int64 `json:"rate_limited_count"`
	// Retried429Count: 触发重试的次数
	Retried429Count int64 `json:"retried_429_count"`
	// RetrySuccessCount: 重试后成功
	RetrySuccessCount int64 `json:"retry_success_count"`
	// RetryFailedCount: 重试耗尽
	RetryFailedCount int64 `json:"retry_failed_count"`
	// AvgQueueWaitMs: 平均等待时间
	AvgQueueWaitMs int64 `json:"avg_queue_wait_ms"`
	// WindowCount: 当前滑动窗口中的请求数（用于调试）
	WindowCount int64 `json:"window_count"`
}
