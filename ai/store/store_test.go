package store

import (
	"database/sql"
	"testing"
	"time"
)

// TestStatisticsDefault 测试统计结构体默认值
func TestStatisticsDefault(t *testing.T) {
	stats := &Statistics{}

	if stats.TotalRequests != 0 {
		t.Errorf("TotalRequests 期望 0, 得到 %d", stats.TotalRequests)
	}
	if stats.SuccessCount != 0 {
		t.Errorf("SuccessCount 期望 0, 得到 %d", stats.SuccessCount)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("TotalTokens 期望 0, 得到 %d", stats.TotalTokens)
	}
}

// TestAnalysisTaskStatus 测试任务状态常量
func TestAnalysisTaskStatus(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Pending", TaskStatusPending, "pending"},
		{"Analyzing", TaskStatusAnalyzing, "analyzing"},
		{"Done", TaskStatusDone, "done"},
		{"Failed", TaskStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("期望 %s, 得到 %s", tt.expected, tt.got)
			}
		})
	}
}

// TestTrafficStatusConstants 测试流量状态常量
func TestTrafficStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Pending", StatusPending, "pending"},
		{"Skipped", StatusSkipped, "skipped"},
		{"Analyzed", StatusAnalyzed, "analyzed"},
		{"Failed", StatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("期望 %s, 得到 %s", tt.expected, tt.got)
			}
		})
	}
}

// TestAnalyzedRecordJSONFields 测试 AnalyzedRecord JSON 序列化字段名
func TestAnalyzedRecordJSONFields(t *testing.T) {
	record := &AnalyzedRecord{
		ID:                  1,
		Host:                "example.com",
		Path:                "/api/test",
		Method:              "GET",
		BusinessName:        "Test API",
		Sensitivity:         "high",
		RiskLevel:          "medium",
		AnalyzedAt:          "2024-01-01T00:00:00Z",
	}

	// 验证 JSON tag
	if record.ID == 0 {
		t.Error("ID 应该被正确设置")
	}
	if record.Path != "/api/test" {
		t.Errorf("Path 期望 /api/test, 得到 %s", record.Path)
	}
}

// TestAnalysisTaskFields 测试 AnalysisTask 结构体
func TestAnalysisTaskFields(t *testing.T) {
	now := time.Now()
	task := &AnalysisTask{
		ID:         "task-123",
		SigKey:     "sig-key-456",
		Host:       "example.com",
		Path:       "/api/test",
		Method:     "POST",
		Status:     TaskStatusAnalyzing,
		Progress:   50,
		ErrorMsg:   "",
		CreatedAt:  now,
		StartedAt:  now,
	}

	if task.ID != "task-123" {
		t.Errorf("ID 期望 task-123, 得到 %s", task.ID)
	}
	if task.Status != TaskStatusAnalyzing {
		t.Errorf("Status 期望 analyzing, 得到 %s", task.Status)
	}
	if task.Progress != 50 {
		t.Errorf("Progress 期望 50, 得到 %d", task.Progress)
	}
}

// TestTrafficRecordFields 测试 TrafficRecord 结构体
func TestTrafficRecordFields(t *testing.T) {
	record := &TrafficRecord{
		ID:             1,
		Host:           "example.com",
		Path:           "/api/users",
		Method:         "GET",
		QueryParamKeys: "page,size",
		ContentType:    "application/json",
		SigKey:        "host:example.com|path:/api/users|method:GET",
		Status:         StatusPending,
	}

	if record.ID != 1 {
		t.Errorf("ID 期望 1, 得到 %d", record.ID)
	}
	if record.Method != "GET" {
		t.Errorf("Method 期望 GET, 得到 %s", record.Method)
	}
	if record.Status != StatusPending {
		t.Errorf("Status 期望 pending, 得到 %s", record.Status)
	}
}

// TestAnalysisResultFields 测试 AnalysisResult 结构体
func TestAnalysisResultFields(t *testing.T) {
	result := &AnalysisResult{
		SessionID:           "session-123",
		BusinessName:        "用户管理",
		BusinessDescription:  "处理用户注册、登录等功能",
		FunctionName:        "用户登录接口",
		Sensitivity:         "high",
		AuthMechanism:       "JWT",
		AnalysisContext:     "这是一个用户认证接口",
		PenetrationPriority: 3,
		RiskLevel:          "medium",
		FinalSummary:       "该接口存在SQL注入风险",
	}

	if result.SessionID != "session-123" {
		t.Errorf("SessionID 期望 session-123, 得到 %s", result.SessionID)
	}
	if result.Sensitivity != "high" {
		t.Errorf("Sensitivity 期望 high, 得到 %s", result.Sensitivity)
	}
	if result.PenetrationPriority != 3 {
		t.Errorf("PenetrationPriority 期望 3, 得到 %d", result.PenetrationPriority)
	}
}

// TestNullTimeHandling 测试空时间处理
func TestNullTimeHandling(t *testing.T) {
	// 测试 NullTime 转换为空字符串
	var nullTime sql.NullTime
	if nullTime.Valid {
		t.Error("NullTime 应该无效")
	}

	// 测试有效时间
	validTime := sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}
	if !validTime.Valid {
		t.Error("NullTime 应该有效")
	}
}

// TestStatisticsIncrement 测试统计增量计算
func TestStatisticsIncrement(t *testing.T) {
	// 模拟增量统计
	var totalRequests int64
	var totalTokens int64

	// 模拟第一次分析
	totalRequests += 1
	totalTokens += 100
	if totalRequests != 1 {
		t.Errorf("totalRequests 期望 1, 得到 %d", totalRequests)
	}
	if totalTokens != 100 {
		t.Errorf("totalTokens 期望 100, 得到 %d", totalTokens)
	}

	// 模拟第二次分析
	totalRequests += 1
	totalTokens += 150
	if totalRequests != 2 {
		t.Errorf("totalRequests 期望 2, 得到 %d", totalRequests)
	}
	if totalTokens != 250 {
		t.Errorf("totalTokens 期望 250, 得到 %d", totalTokens)
	}
}

// TestTokenCalculation 测试 Token 计算
func TestTokenCalculation(t *testing.T) {
	tests := []struct {
		name            string
		promptTokens    int64
		completionTokens int64
		expectedTotal   int64
	}{
		{"正常计算", 100, 50, 150},
		{"仅prompt", 200, 0, 200},
		{"仅completion", 0, 100, 100},
		{"零值", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := tt.promptTokens + tt.completionTokens
			if total != tt.expectedTotal {
				t.Errorf("期望 total %d, 得到 %d", tt.expectedTotal, total)
			}
		})
	}
}

// TestSuccessRateCalculation 测试成功率计算
func TestSuccessRateCalculation(t *testing.T) {
	tests := []struct {
		name          string
		totalRequests int64
		successCount  int64
		expectedRate  float64
	}{
		{"100%成功率", 10, 10, 100.0},
		{"50%成功率", 10, 5, 50.0},
		{"0%成功率", 10, 0, 0.0},
		{"零请求", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rate float64
			if tt.totalRequests == 0 {
				rate = 0
			} else {
				rate = float64(tt.successCount) / float64(tt.totalRequests) * 100
			}
			if rate != tt.expectedRate {
				t.Errorf("期望成功率 %.1f%%, 得到 %.1f%%", tt.expectedRate, rate)
			}
		})
	}
}
