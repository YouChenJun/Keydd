// Package agent Agent 层
// 负责 Agent 工厂和初始化，所有 Agent 实例管理
package agent

import (
	"sync"
	"sync/atomic"
	"time"
)

// AnalysisMetrics 记录 AI 流量分析的运行指标
type AnalysisMetrics struct {
	// 计数（原子操作）
	totalRequests   atomic.Int64
	successCount    atomic.Int64
	failureCount    atomic.Int64
	timeoutCount    atomic.Int64
	llmErrorCount   atomic.Int64
	parseErrorCount atomic.Int64
	rateLimitedCount atomic.Int64

	// 延迟采样（滑动窗口，保留最近 100 次）
	mu           sync.Mutex
	latencies    []time.Duration // 最近 N 次分析的耗时
	latencyIndex int
	latencyCap   int

	// 最后一次分析时间
	lastAnalysisAt atomic.Int64 // UnixNano
	lastSuccessAt  atomic.Int64 // UnixNano
	lastFailureAt  atomic.Int64 // UnixNano
}

// NewAnalysisMetrics 创建指标收集器
func NewAnalysisMetrics() *AnalysisMetrics {
	return &AnalysisMetrics{
		latencies: make([]time.Duration, 100),
		latencyCap: 100,
	}
}

// RecordStart 记录一次分析开始，返回开始时间戳
func (m *AnalysisMetrics) RecordStart() time.Time {
	m.totalRequests.Add(1)
	m.lastAnalysisAt.Store(time.Now().UnixNano())
	return time.Now()
}

// RecordSuccess 记录一次成功分析
func (m *AnalysisMetrics) RecordSuccess(start time.Time) {
	m.successCount.Add(1)
	m.lastSuccessAt.Store(time.Now().UnixNano())
	m.recordLatency(time.Since(start))
}

// RecordFailure 记录一次失败分析，errorType 可选值:
//   - "timeout": LLM 调用超时（60s 内未返回）
//   - "llm_error": LLM 返回了非 429 错误（如认证失败、参数错误、服务端错误）
//   - "parse_error": LLM 调用成功但结构化 JSON 解析失败
//   - "rate_limited": 收到 429 响应（包含重试后仍失败的场景）
func (m *AnalysisMetrics) RecordFailure(start time.Time, errorType string) {
	m.failureCount.Add(1)
	m.lastFailureAt.Store(time.Now().UnixNano())
	m.recordLatency(time.Since(start))

	switch errorType {
	case "timeout":
		m.timeoutCount.Add(1)
	case "llm_error":
		m.llmErrorCount.Add(1)
	case "parse_error":
		m.parseErrorCount.Add(1)
	case "rate_limited":
		m.rateLimitedCount.Add(1)
	}
}

func (m *AnalysisMetrics) recordLatency(d time.Duration) {
	m.mu.Lock()
	m.latencies[m.latencyIndex] = d
	m.latencyIndex = (m.latencyIndex + 1) % m.latencyCap
	m.mu.Unlock()
}

// GetLatencyP95 返回最近 N 次分析耗时的 P95（毫秒）
func (m *AnalysisMetrics) GetLatencyP95() int64 {
	m.mu.Lock()
	n := 0
	for _, lat := range m.latencies {
		if lat > 0 {
			n++
		}
	}
	if n == 0 {
		m.mu.Unlock()
		return 0
	}
	// 复制一份用于排序
	sorted := make([]time.Duration, n)
	j := 0
	for _, lat := range m.latencies {
		if lat > 0 {
			sorted[j] = lat
			j++
		}
	}
	m.mu.Unlock()

	// 简单选择排序（数据量小，100 个元素够用）
	for i := 0; i < n-1; i++ {
		minIdx := i
		for j := i + 1; j < n; j++ {
			if sorted[j] < sorted[minIdx] {
				minIdx = j
			}
		}
		sorted[i], sorted[minIdx] = sorted[minIdx], sorted[i]
	}

	p95Idx := int(float64(n) * 0.95)
	if p95Idx >= n {
		p95Idx = n - 1
	}
	return sorted[p95Idx].Milliseconds()
}

// Snapshot 快照当前所有指标
type MetricsSnapshot struct {
	TotalRequests    int64                    `json:"total_requests"`
	SuccessCount     int64                    `json:"success_count"`
	FailureCount     int64                    `json:"failure_count"`
	TimeoutCount     int64                    `json:"timeout_count"`
	LLMErrorCount    int64                    `json:"llm_error_count"`
	ParseErrorCount  int64                    `json:"parse_error_count"`
	RateLimitedCount int64                    `json:"rate_limited_count"`
	SuccessRate      float64                  `json:"success_rate"`
	LatencyP95Ms     int64                    `json:"latency_p95_ms"`
	LastSuccessAt    string                   `json:"last_success_at"`
	LastFailureAt    string                   `json:"last_failure_at"`
	RateLimiter      RateLimitMetricsSnapshot `json:"rate_limiter"`
}

// Snapshot 返回当前指标的快照，rateLimiterMetrics 为可选的速率限制器指标
func (m *AnalysisMetrics) Snapshot(rateLimiterMetrics ...RateLimitMetricsSnapshot) MetricsSnapshot {
	total := m.totalRequests.Load()
	success := m.successCount.Load()
	failure := m.failureCount.Load()
	var rate float64
	if total > 0 {
		rate = float64(success) / float64(total) * 100
	}

	snap := MetricsSnapshot{
		TotalRequests:    total,
		SuccessCount:     success,
		FailureCount:     failure,
		TimeoutCount:     m.timeoutCount.Load(),
		LLMErrorCount:    m.llmErrorCount.Load(),
		ParseErrorCount:  m.parseErrorCount.Load(),
		RateLimitedCount: m.rateLimitedCount.Load(),
		SuccessRate:      rate,
		LatencyP95Ms:     m.GetLatencyP95(),
	}

	if t := m.lastSuccessAt.Load(); t > 0 {
		snap.LastSuccessAt = time.Unix(0, t).Format(time.RFC3339)
	}
	if t := m.lastFailureAt.Load(); t > 0 {
		snap.LastFailureAt = time.Unix(0, t).Format(time.RFC3339)
	}

	// 填充速率限制器指标
	if len(rateLimiterMetrics) > 0 {
		snap.RateLimiter = rateLimiterMetrics[0]
	}

	return snap
}
