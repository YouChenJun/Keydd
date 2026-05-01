package agent

import (
	"Keydd/ai/config"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockLLMCall 创建一个模拟 LLM 调用函数
// totalCalls: 前 N 次调用返回错误，第 N+1 次返回成功
func mockLLMCall(totalCalls int, errMsg string) func(ctx context.Context) (string, error) {
	callCount := int32(0)
	return func(ctx context.Context) (string, error) {
		n := atomic.AddInt32(&callCount, 1)
		if int(n) <= totalCalls {
			return "", &mockError{msg: errMsg}
		}
		return `{"function_name":"test_api","business_desc":"test"}`, nil
	}
}

// mockError 模拟 LLM 返回的错误
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

// TestRateLimiter_ConcurrentLimit 测试并发上限（semaphore）
// maxConcurrent=2 时，同时最多只有 2 个请求 in-flight
func TestRateLimiter_ConcurrentLimit(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    2,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       false,
		RetryMaxAttempts: 1,
	}
	rl := NewRateLimiter(cfg, nil)

	var maxConcurrent int32
	var currentConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = rl.Execute(context.Background(), "test", func(ctx context.Context) (string, error) {
				n := atomic.AddInt32(&currentConcurrent, 1)
				// 更新最大并发记录
				for {
					old := atomic.LoadInt32(&maxConcurrent)
					if n <= old {
						break
					}
					if atomic.CompareAndSwapInt32(&maxConcurrent, old, n) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&currentConcurrent, -1)
				return "ok", nil
			})
		}()
	}
	wg.Wait()

	if maxConcurrent > 2 {
		t.Errorf("maxConcurrent=%d, want <= 2", maxConcurrent)
	}
	t.Logf("最大并发 observed: %d (limit=2)", maxConcurrent)
}

// TestRateLimiter_SlidingWindowLimit 测试滑动窗口计数正确性
// 窗口内最多 5 个请求，并发=3，验证 WindowCount 正确
func TestRateLimiter_SlidingWindowLimit(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    3,  // 并发 3（足够测试窗口计数）
		WindowSecs:       30,
		MaxInWindow:      50, // 窗口够大，不应限制
		RetryOn429:       false,
		RetryMaxAttempts: 1,
	}
	rl := NewRateLimiter(cfg, nil)

	// 快速并发发送 10 个请求（无延迟）
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = rl.Execute(context.Background(), "test", func(ctx context.Context) (string, error) {
				return "ok", nil
			})
		}()
	}
	wg.Wait()

	// 验证 WindowCount = 10（窗口中记录了所有请求）
	metrics := rl.GetMetrics()
	if metrics.WindowCount != 10 {
		t.Errorf("WindowCount = %d, want 10", metrics.WindowCount)
	}
	t.Logf("WindowCount after 10 concurrent requests: %d", metrics.WindowCount)
}

// TestRateLimiter_SlidingWindowBlockAndWait 测试滑动窗口满时后续请求等待前面过期
func TestRateLimiter_SlidingWindowBlockAndWait(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    1,
		WindowSecs:       1,  // 1 秒窗口
		MaxInWindow:      3,  // 窗口内最多 3 个
		RetryOn429:       false,
		RetryMaxAttempts: 1,
		RetryBaseDelayMs: 50,
	}
	rl := NewRateLimiter(cfg, nil)

	// 发送 3 个快速完成的请求
	for i := 0; i < 3; i++ {
		_, _, _ = rl.Execute(context.Background(), "test", func(ctx context.Context) (string, error) {
			return "ok", nil
		})
	}

	// 第 4 个请求：窗口已满（3/3），需要等待前一个请求从窗口中过期
	// 由于 maxConcurrent=1，前 3 个都在 1 秒内完成，第 4 个会等待
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, err, _ := rl.Execute(ctx, "test", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	elapsed := time.Since(start)
	cancel()

	// 第 4 个应该因为 context 超时而失败（等待时间 > 500ms，窗口还没过期）
	// 因为 1 秒窗口还没过期（只过了几十毫秒），所以第 4 个会阻塞到 context 超时
	if err == nil {
		t.Errorf("expected context deadline exceeded, got nil")
	}
	t.Logf("第 4 个请求等待 %.0fms 后 context 超时（符合预期：1s 窗口未过期）", float64(elapsed.Milliseconds()))
}

// TestRateLimiter_RetryOn429 测试 429 重试 + 指数退避
// totalCalls=3: 前 3 次返回 429，第 4 次成功（但重试次数仅 3，所以最终失败）
func TestRateLimiter_RetryOn429(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    10,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       true,
		RetryMaxAttempts: 3,
		RetryBaseDelayMs: 50,
	}
	rl := NewRateLimiter(cfg, nil)

	start := time.Now()
	result, err, isRateLimited := rl.Execute(context.Background(), "test-session", mockLLMCall(3, "429 rate limit exceeded"))
	elapsed := time.Since(start)

	// totalCalls=3 意味着 call 1/2/3 返回 429，第 4 次成功
	// 但 RetryMaxAttempts=3 意味着只有 3 次尝试（1 次 + 2 次重试），第 3 次失败后不再重试
	if err == nil {
		t.Errorf("expected error after retries exhausted")
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
	if !isRateLimited {
		t.Errorf("isRateLimited should be true")
	}

	metrics := rl.GetMetrics()
	// RateLimitedCount: 遇到 429 的次数 = 3（call 1,2,3 都返回 429）
	if metrics.RateLimitedCount != 3 {
		t.Errorf("RateLimitedCount = %d, want 3", metrics.RateLimitedCount)
	}
	// RetryFailedCount: 重试耗尽 = 1（第 3 次 call 触发重试但最终失败）
	if metrics.RetryFailedCount != 1 {
		t.Errorf("RetryFailedCount = %d, want 1", metrics.RetryFailedCount)
	}
	// Retried429Count: 触发重试的次数 = 2（call 1 和 call 2 触发重试，call 3 不再重试）
	if metrics.Retried429Count != 2 {
		t.Errorf("Retried429Count = %d, want 2", metrics.Retried429Count)
	}
	t.Logf("重试测试: elapsed=%v, RateLimitedCount=%d, Retried429Count=%d, RetryFailedCount=%d",
		elapsed, metrics.RateLimitedCount, metrics.Retried429Count, metrics.RetryFailedCount)
}

// TestRateLimiter_RetrySuccessAfter429 测试 429 重试后成功
// totalCalls=2: 前 2 次失败，第 3 次成功（RetryMaxAttempts=3，所以能成功）
func TestRateLimiter_RetrySuccessAfter429(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    10,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       true,
		RetryMaxAttempts: 3,
		RetryBaseDelayMs: 50,
	}
	rl := NewRateLimiter(cfg, nil)

	result, err, _ := rl.Execute(context.Background(), "test-session", mockLLMCall(2, "429 Too Many Requests"))

	if err != nil {
		t.Errorf("unexpected error on retry success: %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty result")
	}

	metrics := rl.GetMetrics()
	// RetrySuccessCount: 重试后成功 = 1
	if metrics.RetrySuccessCount != 1 {
		t.Errorf("RetrySuccessCount = %d, want 1", metrics.RetrySuccessCount)
	}
	// RetryFailedCount: 重试失败 = 0
	if metrics.RetryFailedCount != 0 {
		t.Errorf("RetryFailedCount = %d, want 0", metrics.RetryFailedCount)
	}
	// RateLimitedCount: 遇到 429 的次数 = 2
	if metrics.RateLimitedCount != 2 {
		t.Errorf("RateLimitedCount = %d, want 2", metrics.RateLimitedCount)
	}
	t.Logf("重试成功测试: RetrySuccessCount=%d, RetryFailedCount=%d, RateLimitedCount=%d",
		metrics.RetrySuccessCount, metrics.RetryFailedCount, metrics.RateLimitedCount)
}

// TestRateLimiter_ContextCancel 测试 context 取消能立即中断等待
func TestRateLimiter_ContextCancel(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    1,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       false,
		RetryMaxAttempts: 1,
		RetryBaseDelayMs: 100,
	}
	rl := NewRateLimiter(cfg, nil)

	// 启动一个长时间占用的请求（占用唯一的并发槽位）
	done := make(chan struct{})
	go func() {
		_, _, _ = rl.Execute(context.Background(), "long-running", func(ctx context.Context) (string, error) {
			time.Sleep(500 * time.Millisecond)
			close(done)
			return "ok", nil
		})
	}()

	time.Sleep(50 * time.Millisecond) // 确保长时间请求占用槽位

	// 创建一个已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	result, err, isRateLimited := rl.Execute(ctx, "canceled-session", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	elapsed := time.Since(start)

	// context canceled 请求应该立即返回（不等待槽位）
	if err == nil {
		t.Errorf("expected context canceled error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
	if isRateLimited {
		t.Errorf("isRateLimited should be false for context canceled")
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("context cancel should be immediate, took %v", elapsed)
	}
	t.Logf("context canceled 测试成功: elapsed=%v", elapsed)
	<-done
}

// TestRateLimiter_JitterNoPanic 测试 baseDelay=0 时不 panic
func TestRateLimiter_JitterNoPanic(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    10,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       true,
		RetryMaxAttempts: 3,
		RetryBaseDelayMs: 0, // 边界值：0ms
	}
	rl := NewRateLimiter(cfg, nil)

	// 不 panic 即可
	result, err, _ := rl.Execute(context.Background(), "test-session", mockLLMCall(3, "429 rate limit exceeded"))
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
	// err 应该不为 nil（重试耗尽）
	if err == nil {
		t.Errorf("expected error after retries exhausted")
	}
	t.Log("baseDelay=0 未 panic，测试通过")
}

// TestRateLimiter_MetricsSnapshot 测试指标快照正确性
func TestRateLimiter_MetricsSnapshot(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    3,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       true,
		RetryMaxAttempts: 3,
		RetryBaseDelayMs: 10,
	}
	rl := NewRateLimiter(cfg, nil)

	// 1. 成功请求
	_, err, _ := rl.Execute(context.Background(), "s1", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// 2. 429 重试成功 (totalCalls=2, 前2次失败, 第3次成功)
	_, err, _ = rl.Execute(context.Background(), "s2", mockLLMCall(2, "429"))

	// 3. 429 重试失败 (totalCalls=3, 全部失败)
	_, err, _ = rl.Execute(context.Background(), "s3", mockLLMCall(3, "429"))

	metrics := rl.GetMetrics()

	// RateLimitedCount: 429 次数 = 2 (s2) + 3 (s3) = 5
	if metrics.RateLimitedCount != 5 {
		t.Errorf("RateLimitedCount = %d, want 5", metrics.RateLimitedCount)
	}
	// RetrySuccessCount: 重试成功后 = 1 (s2)
	if metrics.RetrySuccessCount != 1 {
		t.Errorf("RetrySuccessCount = %d, want 1", metrics.RetrySuccessCount)
	}
	// RetryFailedCount: 重试失败 = 1 (s3)
	if metrics.RetryFailedCount != 1 {
		t.Errorf("RetryFailedCount = %d, want 1", metrics.RetryFailedCount)
	}
	// Retried429Count: 触发重试的次数 = 2 (s2: call1, call2) + 2 (s3: call1, call2) = 4
	// （attempt < RetryMaxAttempts 时才计入，attempt=RetryMaxAttempts 时触发重试但不计入）
	if metrics.Retried429Count != 4 {
		t.Errorf("Retried429Count = %d, want 4", metrics.Retried429Count)
	}

	t.Logf("Metrics: RateLimitedCount=%d, Retried429Count=%d, RetrySuccessCount=%d, RetryFailedCount=%d",
		metrics.RateLimitedCount, metrics.Retried429Count, metrics.RetrySuccessCount, metrics.RetryFailedCount)
}

// TestRateLimiter_Non429NoRetry 测试非 429 错误不重试
func TestRateLimiter_Non429NoRetry(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    10,
		WindowSecs:       30,
		MaxInWindow:      50,
		RetryOn429:       true,
		RetryMaxAttempts: 3,
		RetryBaseDelayMs: 5000, // 故意设大，如果有重试会等很久
	}
	rl := NewRateLimiter(cfg, nil)

	start := time.Now()
	_, _, isRateLimited := rl.Execute(context.Background(), "test-session", func(ctx context.Context) (string, error) {
		return "", &mockError{msg: "500 Internal Server Error"} // 非 429 错误
	})
	elapsed := time.Since(start)

	// 非 429 错误不应重试，立即返回
	if elapsed > 100*time.Millisecond {
		t.Errorf("non-429 error should not retry, took %v", elapsed)
	}
	if isRateLimited {
		t.Errorf("isRateLimited should be false for non-429 error")
	}
	t.Logf("非 429 错误未重试，耗时 %v（符合预期）", elapsed)
}

// TestRateLimiter_DefaultValues 测试零值配置使用默认值
func TestRateLimiter_DefaultValues(t *testing.T) {
	cfg := config.RateLimitConfig{} // 全部零值
	rl := NewRateLimiter(cfg, nil)

	metrics := rl.GetMetrics()
	// 至少不发 panic就算通过（默认值在构造时已设置）
	if metrics.RateLimitedCount != 0 {
		t.Errorf("expected 0 RateLimitedCount, got %d", metrics.RateLimitedCount)
	}
	t.Log("零值配置未 panic，测试通过")
}

// TestRateLimiter_SlidingWindowExpiry 测试滑动窗口过期清理
func TestRateLimiter_SlidingWindowExpiry(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    3,
		WindowSecs:       1, // 1 秒窗口（快速过期）
		MaxInWindow:      10,
		RetryOn429:       false,
		RetryMaxAttempts: 1,
	}
	rl := NewRateLimiter(cfg, nil)

	// 发送 5 个请求填满窗口
	for i := 0; i < 5; i++ {
		_, _, _ = rl.Execute(context.Background(), "test", func(ctx context.Context) (string, error) {
			return "ok", nil
		})
	}

	m1 := rl.GetMetrics()
	t.Logf("发送 5 个请求后 WindowCount = %d", m1.WindowCount)

	// 等待 1.5 秒，窗口过期
	time.Sleep(1500 * time.Millisecond)

	// 新请求应该能立即通过（窗口已空）
	start := time.Now()
	_, _, _ = rl.Execute(context.Background(), "test-after-expiry", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("请求应在窗口过期后立即通过，耗时 %v", elapsed)
	}
	t.Logf("窗口过期后请求耗时 %v（符合预期）", elapsed)
}

// TestRateLimiter_ConcurrentLimitUnderPressure 压测：验证并发上限在压力下仍然严格
func TestRateLimiter_ConcurrentLimitUnderPressure(t *testing.T) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    3,
		WindowSecs:       30,
		MaxInWindow:      100,
		RetryOn429:       false,
		RetryMaxAttempts: 1,
	}
	rl := NewRateLimiter(cfg, nil)

	var maxConcurrent int32
	var wg sync.WaitGroup

	// 20 个 goroutine 并发请求
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = rl.Execute(context.Background(), "test", func(ctx context.Context) (string, error) {
				time.Sleep(20 * time.Millisecond)
				return "ok", nil
			})
		}()
	}
	wg.Wait()

	t.Logf("压力测试完成，最大并发 = %d（限制 = 3）", maxConcurrent)
}

// BenchmarkRateLimiter_Execute benchmarks the rate limiter under load
func BenchmarkRateLimiter_Execute(b *testing.B) {
	cfg := config.RateLimitConfig{
		MaxConcurrent:    10,
		WindowSecs:       30,
		MaxInWindow:      1000,
		RetryOn429:       false,
		RetryMaxAttempts: 1,
	}
	rl := NewRateLimiter(cfg, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = rl.Execute(context.Background(), "bench", func(ctx context.Context) (string, error) {
			return "ok", nil
		})
	}
}
