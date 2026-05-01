package proxy

import (
	"Keydd/ai/config"
	"Keydd/ai/store"
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockDB 模拟数据库适配器
type mockDB struct {
	insertCount   int
	shouldSkip    bool
	shouldReinsert bool
	mu            sync.Mutex
}

func (m *mockDB) ShouldAnalyze(key string) (bool, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shouldSkip, m.shouldReinsert, nil
}

func (m *mockDB) InsertSignature(rec *store.TrafficRecord) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertCount++
	return int64(m.insertCount), false, nil
}

func (m *mockDB) UpdateAnalysisResult(key string, result *store.AnalysisResult) error {
	return nil
}

// mockAISystem 模拟 AISystem
type mockAISystem struct {
	analysisCount int32
}

// mockFactory 模拟 AgentFactory
type mockFactory struct {
	analysisDelay time.Duration
	analysisError error
	callCount     int32
}

func (f *mockFactory) RunFullAnalysis(ctx context.Context, sessionID string, sigID int64, host, method, path, sampleReq, sampleResp string) (interface{}, error) {
	atomic.AddInt32(&f.callCount, 1)
	if f.analysisDelay > 0 {
		time.Sleep(f.analysisDelay)
	}
	return nil, f.analysisError
}

func (f *mockFactory) GetMetrics() interface{} {
	return nil
}

func newMockHandler(pendingLimit int) *Handler {
	aiConfig := config.AIConfig{
		LLM: config.LLMConfig{
			RateLimit: config.RateLimitConfig{
				MaxConcurrent: 10,
				WindowSecs:    30,
				MaxInWindow:   50,
			},
		},
		Analysis: config.AnalysisConfig{
			DeduplicationEnabled: false, // 测试背压时关闭去重，避免干扰
		},
	}
	h := &Handler{
		aiConfig:        aiConfig,
		pendingChan:     make(chan struct{}, pendingLimit),
		analyzedSig:     make(map[string]time.Time),
		lastCleanup:     time.Now(),
		dedupCleanInterval: 5 * time.Minute,
	}
	return h
}

// TestHandler_TryAcquirePending tests backpressure control
func TestHandler_TryAcquirePending(t *testing.T) {
	h := newMockHandler(3)

	// 获取 3 个 slot（达到上限）
	for i := 0; i < 3; i++ {
		if !h.tryAcquirePending() {
			t.Errorf("slot %d should be acquired", i)
		}
	}

	// 第 4 个应该失败（背压）
	if h.tryAcquirePending() {
		t.Errorf("slot should be rejected due to backpressure")
	}

	// 释放 1 个
	h.releasePending()

	// 现在可以再获取 1 个
	if !h.tryAcquirePending() {
		t.Errorf("slot should be available after release")
	}
}

// TestHandler_TTLExpire tests TTL cleanup of analyzedSig map
func TestHandler_TTLExpire(t *testing.T) {
	h := newMockHandler(10)
	h.dedupCleanInterval = 10 * time.Millisecond // 测试用短间隔

	// 手动添加 5 个"过期"的记录（时间设为 1 小时前）
	h.analyzedMu.Lock()
	for i := 0; i < 5; i++ {
		h.analyzedSig[string(rune('a'+i))] = time.Now().Add(-1 * time.Hour)
	}
	// 添加 3 个"新鲜"的记录
	for i := 0; i < 3; i++ {
		h.analyzedSig[string(rune('f'+i))] = time.Now()
	}
	h.lastCleanup = time.Now().Add(-10 * time.Minute) // 强制下次触发清理
	h.analyzedMu.Unlock()

	// 触发清理（通过 getAnalyzedSig）
	_, _ = h.getAnalyzedSig("dummy")

	h.analyzedMu.Lock()
	defer h.analyzedMu.Unlock()

	// 过期记录应被清理
	if len(h.analyzedSig) != 3 {
		t.Errorf("expected 3 entries after cleanup, got %d", len(h.analyzedSig))
	}
	// 新鲜记录应保留
	for i := 0; i < 3; i++ {
		key := string(rune('f' + i))
		if _, ok := h.analyzedSig[key]; !ok {
			t.Errorf("fresh entry %c should be preserved", 'f'+i)
		}
	}
	t.Logf("TTL清理测试: 清理前 8 条，清理后 %d 条", len(h.analyzedSig))
}

// TestHandler_AnalyzedSigMapTypes tests that analyzedSig map stores time.Time
func TestHandler_AnalyzedSigMapTypes(t *testing.T) {
	h := newMockHandler(10)

	h.markAnalyzedSig("test-key")

	h.analyzedMu.RLock()
	defer h.analyzedMu.RUnlock()

	v, ok := h.analyzedSig["test-key"]
	if !ok {
		t.Errorf("expected key to exist")
	}
	if v.IsZero() {
		t.Errorf("time should not be zero after markAnalyzedSig")
	}
	t.Logf("markAnalyzedSig 正确存储 time.Time: %v", v)
}

// TestHandler_PendingChanCapacity tests that pendingChan has correct capacity
func TestHandler_PendingChanCapacity(t *testing.T) {
	maxConcurrent := 3
	pendingLimit := maxConcurrent * 5 // 15
	h := newMockHandler(pendingLimit)

	if cap(h.pendingChan) != pendingLimit {
		t.Errorf("pendingChan capacity = %d, want %d", cap(h.pendingChan), pendingLimit)
	}
	if len(h.pendingChan) != 0 {
		t.Errorf("pendingChan initial length = %d, want 0", len(h.pendingChan))
	}

	// 填充到容量
	for i := 0; i < pendingLimit; i++ {
		h.tryAcquirePending()
	}

	if len(h.pendingChan) != pendingLimit {
		t.Errorf("pendingChan length = %d, want %d", len(h.pendingChan), pendingLimit)
	}

	// 再获取应该失败
	if h.tryAcquirePending() {
		t.Errorf("should reject when at capacity")
	}
}

// TestHandler_MultipleAcquireRelease tests concurrent acquire and release
func TestHandler_MultipleAcquireRelease(t *testing.T) {
	h := newMockHandler(5)
	var successCount int32
	var wg sync.WaitGroup

	// 10 个 goroutine 竞争 5 个 slot，每个 goroutine 重试直到成功
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if h.tryAcquirePending() {
					atomic.AddInt32(&successCount, 1)
					time.Sleep(5 * time.Millisecond)
					h.releasePending()
					return
				}
				// 没有 slot 时短暂让出，循环重试
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	// 10 个 goroutine 都应该成功获取并释放 slot
	if successCount != 10 {
		t.Errorf("expected 10 successful acquires, got %d", successCount)
	}
	t.Logf("10 个 goroutine 全部成功获取并释放 slot，successCount=%d", successCount)
}

// TestHandler_CleanupInterval tests that cleanup only runs every interval
func TestHandler_CleanupInterval(t *testing.T) {
	h := newMockHandler(10)
	h.dedupCleanInterval = 100 * time.Millisecond

	// 添加过期记录
	h.analyzedMu.Lock()
	h.analyzedSig["old"] = time.Now().Add(-1 * time.Hour)
	h.lastCleanup = time.Now() // 上次清理刚执行
	h.analyzedMu.Unlock()

	// 第一次调用不应清理（间隔未到）
	_, _ = h.getAnalyzedSig("dummy")

	h.analyzedMu.RLock()
	_, hasOld := h.analyzedSig["old"]
	h.analyzedMu.RUnlock()

	if !hasOld {
		t.Errorf("old entry should still exist (cleanup interval not passed)")
	}

	// 等待间隔过去
	time.Sleep(150 * time.Millisecond)

	// 第二次调用应该清理
	_, _ = h.getAnalyzedSig("dummy2")

	h.analyzedMu.RLock()
	_, hasOld = h.analyzedSig["old"]
	h.analyzedMu.RUnlock()

	if hasOld {
		t.Errorf("old entry should be cleaned up after interval passed")
	}
}

// TestHandler_NoLeakAfterRelease tests no goroutine leak after release
func TestHandler_NoLeakAfterRelease(t *testing.T) {
	h := newMockHandler(5)

	for i := 0; i < 5; i++ {
		if h.tryAcquirePending() {
			h.releasePending()
		}
	}

	// 验证 channel 已清空
	if len(h.pendingChan) != 0 {
		t.Errorf("pendingChan should be empty after release, got len=%d", len(h.pendingChan))
	}

	// 验证可以继续获取
	if !h.tryAcquirePending() {
		t.Errorf("should be able to acquire after release")
	}
}
