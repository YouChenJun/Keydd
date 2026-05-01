package agent

import (
	"sync"
	"testing"
)

func TestTokenTracker_NewTokenTracker(t *testing.T) {
	tracker := NewTokenTracker()
	if tracker == nil {
		t.Fatal("Expected NewTokenTracker to return non-nil")
	}
}

func TestTokenTracker_Record(t *testing.T) {
	tracker := NewTokenTracker()

	tracker.Record(100, 50, 10)

	snapshot := tracker.GetSnapshot()
	if snapshot.TotalPromptTokens != 100 {
		t.Errorf("Expected TotalPromptTokens=100, got %d", snapshot.TotalPromptTokens)
	}
	if snapshot.TotalCompletionTokens != 50 {
		t.Errorf("Expected TotalCompletionTokens=50, got %d", snapshot.TotalCompletionTokens)
	}
	if snapshot.TotalTokens != 150 {
		t.Errorf("Expected TotalTokens=150, got %d", snapshot.TotalTokens)
	}
	if snapshot.PromptCachedTokens != 10 {
		t.Errorf("Expected PromptCachedTokens=10, got %d", snapshot.PromptCachedTokens)
	}
	if snapshot.TurnCount != 1 {
		t.Errorf("Expected TurnCount=1, got %d", snapshot.TurnCount)
	}
}

func TestTokenTracker_MultipleRecords(t *testing.T) {
	tracker := NewTokenTracker()

	tracker.Record(100, 50, 10)
	tracker.Record(200, 100, 20)
	tracker.Record(150, 75, 15)

	snapshot := tracker.GetSnapshot()
	if snapshot.TotalPromptTokens != 450 {
		t.Errorf("Expected TotalPromptTokens=450, got %d", snapshot.TotalPromptTokens)
	}
	if snapshot.TotalCompletionTokens != 225 {
		t.Errorf("Expected TotalCompletionTokens=225, got %d", snapshot.TotalCompletionTokens)
	}
	if snapshot.TotalTokens != 675 {
		t.Errorf("Expected TotalTokens=675, got %d", snapshot.TotalTokens)
	}
	if snapshot.TurnCount != 3 {
		t.Errorf("Expected TurnCount=3, got %d", snapshot.TurnCount)
	}
}

func TestTokenTracker_Concurrent(t *testing.T) {
	tracker := NewTokenTracker()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.Record(10, 5, 1)
		}(i)
	}

	wg.Wait()

	snapshot := tracker.GetSnapshot()
	expectedTokens := int64(numGoroutines * 10)
	expectedCompletion := int64(numGoroutines * 5)
	expectedTotal := int64(numGoroutines * 15)
	expectedCount := int64(numGoroutines)

	if snapshot.TotalPromptTokens != expectedTokens {
		t.Errorf("Expected TotalPromptTokens=%d, got %d", expectedTokens, snapshot.TotalPromptTokens)
	}
	if snapshot.TotalCompletionTokens != expectedCompletion {
		t.Errorf("Expected TotalCompletionTokens=%d, got %d", expectedCompletion, snapshot.TotalCompletionTokens)
	}
	if snapshot.TotalTokens != expectedTotal {
		t.Errorf("Expected TotalTokens=%d, got %d", expectedTotal, snapshot.TotalTokens)
	}
	if snapshot.TurnCount != expectedCount {
		t.Errorf("Expected TurnCount=%d, got %d", expectedCount, snapshot.TurnCount)
	}
}

func TestTokenUsageSnapshot_Fields(t *testing.T) {
	snapshot := TokenUsageSnapshot{
		TotalPromptTokens:     1000,
		TotalCompletionTokens: 500,
		TotalTokens:          1500,
		PromptCachedTokens:   100,
		TurnCount:            10,
	}

	if snapshot.TotalPromptTokens != 1000 {
		t.Errorf("Expected TotalPromptTokens=1000, got %d", snapshot.TotalPromptTokens)
	}
	if snapshot.TotalCompletionTokens != 500 {
		t.Errorf("Expected TotalCompletionTokens=500, got %d", snapshot.TotalCompletionTokens)
	}
	if snapshot.TotalTokens != 1500 {
		t.Errorf("Expected TotalTokens=1500, got %d", snapshot.TotalTokens)
	}
	if snapshot.PromptCachedTokens != 100 {
		t.Errorf("Expected PromptCachedTokens=100, got %d", snapshot.PromptCachedTokens)
	}
	if snapshot.TurnCount != 10 {
		t.Errorf("Expected TurnCount=10, got %d", snapshot.TurnCount)
	}
}

func TestTokenTracker_ZeroValues(t *testing.T) {
	tracker := NewTokenTracker()

	snapshot := tracker.GetSnapshot()
	if snapshot.TotalPromptTokens != 0 {
		t.Errorf("Expected TotalPromptTokens=0, got %d", snapshot.TotalPromptTokens)
	}
	if snapshot.TotalCompletionTokens != 0 {
		t.Errorf("Expected TotalCompletionTokens=0, got %d", snapshot.TotalCompletionTokens)
	}
	if snapshot.TotalTokens != 0 {
		t.Errorf("Expected TotalTokens=0, got %d", snapshot.TotalTokens)
	}
	if snapshot.TurnCount != 0 {
		t.Errorf("Expected TurnCount=0, got %d", snapshot.TurnCount)
	}
}

func TestAgentResult_Fields(t *testing.T) {
	result := AgentResult{
		Content:      "test content",
		PromptTokens: 100,
		OutputTokens: 50,
		CachedTokens: 10,
	}

	if result.Content != "test content" {
		t.Errorf("Expected Content='test content', got '%s'", result.Content)
	}
	if result.PromptTokens != 100 {
		t.Errorf("Expected PromptTokens=100, got %d", result.PromptTokens)
	}
	if result.OutputTokens != 50 {
		t.Errorf("Expected OutputTokens=50, got %d", result.OutputTokens)
	}
	if result.CachedTokens != 10 {
		t.Errorf("Expected CachedTokens=10, got %d", result.CachedTokens)
	}
}
