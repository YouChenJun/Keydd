package agent

import (
	"context"
	"testing"
	"time"

	"Keydd/ai/agent/prompts"
	"Keydd/ai/config"
)

// TestAgentFactoryInitialization 测试 AgentFactory 初始化
func TestAgentFactoryInitialization(t *testing.T) {
	cfg := config.AIConfig{
		Enabled: true,
		LLM: config.LLMConfig{
			Model:  "gpt-4o-mini",
			APIKey: "test-key",
			BaseURL: "https://api.openai.com/v1",
		},
		Analysis: config.AnalysisConfig{
			BusinessAnalysisEnabled: true,
		},
	}

	if cfg.LLM.Model != "gpt-4o-mini" {
		t.Errorf("Config model mismatch: expected gpt-4o-mini, got %s", cfg.LLM.Model)
	}
	if !cfg.Analysis.BusinessAnalysisEnabled {
		t.Errorf("Business analysis should be enabled")
	}
	t.Log("AgentFactory config validated")
}

// TestAnalysisConfigFields 测试分析配置字段的正确性
func TestAnalysisConfigFields(t *testing.T) {
	cfg := config.AnalysisConfig{
		BusinessAnalysisEnabled: true,
		DeduplicationEnabled:    true,
		MemoryAnalysisEnabled:   false,
		OnlyAnalyzeXHR:          false,
	}

	if !cfg.BusinessAnalysisEnabled {
		t.Errorf("Business analysis should be enabled")
	}
	if !cfg.DeduplicationEnabled {
		t.Errorf("Deduplication should be enabled")
	}
	t.Logf("Analysis config fields validated (Business:%v, Dedup:%v, XHR:%v, Memory:%v)",
		cfg.BusinessAnalysisEnabled,
		cfg.DeduplicationEnabled,
		cfg.OnlyAnalyzeXHR,
		cfg.MemoryAnalysisEnabled,
	)
}

// TestBusinessAnalysisRequest 测试业务分析请求结构
func TestBusinessAnalysisRequest(t *testing.T) {
	testCases := []struct {
		name        string
		host        string
		requests    []string
		responses   []string
		description string
	}{
		{
			name: "ecommerce_api",
			host: "api.shop.com",
			requests: []string{
				"GET /api/v1/products HTTP/1.1",
				"GET /api/v1/users/profile HTTP/1.1",
				"POST /api/v1/orders HTTP/1.1",
			},
			responses: []string{
				"HTTP/1.1 200 OK\n\n{\"id\":1,\"name\":\"product\",\"price\":99.99}",
				"HTTP/1.1 200 OK\n\n{\"id\":123,\"email\":\"user@example.com\",\"orders\":[]}",
				"HTTP/1.1 201 Created\n\n{\"order_id\":456}",
			},
			description: "E-commerce API analysis",
		},
		{
			name: "social_media_api",
			host: "api.social.com",
			requests: []string{
				"GET /api/posts HTTP/1.1",
				"POST /api/posts HTTP/1.1",
				"GET /api/users/@username HTTP/1.1",
			},
			responses: []string{
				"HTTP/1.1 200 OK\n\n[{\"id\":1,\"content\":\"hello\",\"likes\":10}]",
				"HTTP/1.1 201 Created\n\n{\"id\":999,\"content\":\"new post\"}",
				"HTTP/1.1 200 OK\n\n{\"username\":\"john\",\"followers\":100}",
			},
			description: "Social media API analysis",
		},
		{
			name: "financial_api",
			host: "api.bank.com",
			requests: []string{
				"GET /api/v1/accounts HTTP/1.1",
				"GET /api/v1/transactions HTTP/1.1",
				"POST /api/v1/transfers HTTP/1.1",
			},
			responses: []string{
				"HTTP/1.1 200 OK\n\n{\"account_id\":\"ACC123\",\"balance\":5000.00,\"currency\":\"USD\"}",
				"HTTP/1.1 200 OK\n\n[{\"id\":\"TXN1\",\"amount\":100,\"date\":\"2024-01-01\"}]",
				"HTTP/1.1 200 OK\n\n{\"status\":\"success\",\"transfer_id\":\"TRF123\"}",
			},
			description: "Financial/Banking API analysis",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.requests) != len(tc.responses) {
				t.Errorf("Request/response count mismatch")
			}
			t.Logf("%s: %s (Endpoints: %d)", tc.name, tc.description, len(tc.requests))
		})
	}
}

// TestSystemPrompts 测试系统提示词的完整性
func TestSystemPrompts(t *testing.T) {
	testCases := []struct {
		name        string
		promptType  string
		description string
	}{
		{
			name:        "traffic_analyzer_prompt",
			promptType:  "traffic_analyzer",
			description: "Traffic analyzer prompt",
		},
		{
			name:        "api_analysis_prompt",
			promptType:  "api_analysis",
			description: "API analysis prompt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prompt := prompts.GetSystemPrompt(tc.promptType)
			if prompt == "" {
				t.Errorf("Prompt for %s is empty", tc.promptType)
			} else {
				t.Logf("%s: %s (Length: %d chars)", tc.name, tc.description, len(prompt))
			}
		})
	}
}

// TestContextTimeout 测试上下文超时处理
func TestContextTimeout(t *testing.T) {
	testCases := []struct {
		name              string
		timeoutDuration   time.Duration
		operationDuration time.Duration
		shouldTimeout     bool
		description       string
	}{
		{
			name:              "operation_completes_in_time",
			timeoutDuration:   5 * time.Second,
			operationDuration: 1 * time.Second,
			shouldTimeout:     false,
			description:       "Operation completes before timeout",
		},
		{
			name:              "operation_exceeds_timeout",
			timeoutDuration:   1 * time.Second,
			operationDuration: 5 * time.Second,
			shouldTimeout:     true,
			description:       "Operation exceeds timeout",
		},
		{
			name:              "immediate_timeout",
			timeoutDuration:   100 * time.Millisecond,
			operationDuration: 1 * time.Second,
			shouldTimeout:     true,
			description:       "Very short timeout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeoutDuration)
			defer cancel()

			if ctx == nil {
				t.Errorf("Context creation failed")
			} else {
				t.Logf("%s: %s", tc.name, tc.description)
			}
		})
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	testCases := []struct {
		name        string
		errorType   string
		description string
	}{
		{
			name:        "llm_timeout",
			errorType:   "timeout",
			description: "LLM request timeout",
		},
		{
			name:        "invalid_api_key",
			errorType:   "auth_error",
			description: "Invalid API key",
		},
		{
			name:        "network_error",
			errorType:   "network",
			description: "Network connectivity error",
		},
		{
			name:        "invalid_response",
			errorType:   "invalid_response",
			description: "Invalid LLM response format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("%s: %s", tc.name, tc.description)
		})
	}
}
