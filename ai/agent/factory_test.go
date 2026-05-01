package agent

import (
	"Keydd/ai/agent/prompts"
	"Keydd/ai/tools"
	"Keydd/ai/config"
	"os"
	"testing"
)

// TestNewAgentFactory_MissingAPIKey 测试 API Key 缺失
func TestNewAgentFactory_MissingAPIKey(t *testing.T) {
	// 保存原始环境变量
	oldAPIKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", oldAPIKey)

	// 清除环境变量
	os.Unsetenv("OPENAI_API_KEY")

	cfg := config.AIConfig{
		Enabled: true,
		LLM: config.LLMConfig{
			Model:  "gpt-4",
			APIKey: "", // 空的 API Key
		},
	}

	factory, err := NewAgentFactory(cfg)
	if err == nil {
		t.Errorf("期望返回错误，但得到成功")
	}
	if factory != nil {
		t.Errorf("工厂应该为 nil")
	}
}

// TestGetSystemPrompt_AllPrompts 测试所有系统提示词
func TestGetSystemPrompt_AllPrompts(t *testing.T) {
	promptTypes := []string{
		"traffic_analyzer",
		"api_analysis",
	}

	for _, pt := range promptTypes {
		result := prompts.GetSystemPrompt(pt)
		if result == "" {
			t.Errorf("提示词 %q 返回空字符串", pt)
		}
		if len(result) < 20 {
			t.Errorf("提示词 %q 长度过短: %d", pt, len(result))
		}
	}
}

// TestGetSystemPrompt_Unknown 测试未知的提示词
func TestGetSystemPrompt_Unknown(t *testing.T) {
	result := prompts.GetSystemPrompt("unknown_prompt_type")
	if result == "" {
		t.Errorf("未知提示词应该返回默认提示词而不是空字符串")
	}
}

// TestExtractRiskLevel 测试风险等级提取
func TestExtractRiskLevel(t *testing.T) {
	testCases := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "High Risk",
			text:     "This endpoint has HIGH risk of SQL injection",
			expected: "high",
		},
		{
			name:     "CRITICAL in uppercase",
			text:     "CRITICAL vulnerability found in authentication",
			expected: "critical",
		},
		{
			name:     "Medium with other words",
			text:     "The Medium level security issue should be addressed",
			expected: "medium",
		},
		{
			name:     "Low risk",
			text:     "LOW possibility of XSS attack",
			expected: "low",
		},
		{
			name:     "No risk level",
			text:     "Some regular text without risk level",
			expected: "",
		},
		{
			name:     "Multiple risk levels - first wins",
			text:     "HIGH risk and CRITICAL issue detected",
			expected: "high",
		},
	}

	for _, tc := range testCases {
		result := tools.ExtractRiskLevel(tc.text)
		if result != tc.expected {
			t.Errorf("[%s] 期望 %q，实际 %q", tc.name, tc.expected, result)
		}
	}
}
