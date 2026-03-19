package cmd

import (
	"Keydd/consts"
	"testing"
)

// makeRule 辅助函数：快速构造测试用规则
func makeRule(id, pattern string, enabled bool, testCases ...string) consts.Rule {
	return consts.Rule{
		Id:        id,
		Enabled:   enabled,
		Pattern:   pattern,
		TestCases: testCases,
	}
}

// TestValidateRules_AllPass 验证正常规则全部通过
func TestValidateRules_AllPass(t *testing.T) {
	rules := []consts.Rule{
		makeRule("phone", `\b1[3-9]\d{9}\b`, true, "13812345678", "19988887777"),
		makeRule("jwt_token",
			`eyJ[A-Za-z0-9_/+\-]{10,}={0,2}\.[A-Za-z0-9_/+\-\\]{15,}={0,2}\.[A-Za-z0-9_/+\-\\]{10,}={0,2}`,
			true,
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		),
	}

	errs := ValidateRules(rules)
	if len(errs) != 0 {
		t.Errorf("期望 0 个错误，实际得到 %d 个：%v", len(errs), errs)
	}
}

// TestValidateRules_DisabledRuleSkipped 禁用规则不参与验证
func TestValidateRules_DisabledRuleSkipped(t *testing.T) {
	rules := []consts.Rule{
		// enabled=false，即使 pattern 为空、没有 test_cases 也不应报错
		makeRule("bad_rule", "", false),
	}

	errs := ValidateRules(rules)
	if len(errs) != 0 {
		t.Errorf("禁用规则不应产生验证错误，实际得到 %d 个", len(errs))
	}
}

// TestValidateRules_InvalidRegex 坏正则被捕获
func TestValidateRules_InvalidRegex(t *testing.T) {
	rules := []consts.Rule{
		makeRule("bad_regex", `[invalid(regex`, true, "some input"),
	}

	errs := ValidateRules(rules)
	if len(errs) == 0 {
		t.Fatal("期望捕获到正则编译错误，但未捕获")
	}
	if errs[0].ErrorType != ErrCompile {
		t.Errorf("期望错误类型 %s，实际得到 %s", ErrCompile, errs[0].ErrorType)
	}
}

// TestValidateRules_NoTestCase 缺少测试样例被捕获
func TestValidateRules_NoTestCase(t *testing.T) {
	rules := []consts.Rule{
		makeRule("no_cases", `\b1[3-9]\d{9}\b`, true), // 无 test_cases
	}

	errs := ValidateRules(rules)
	if len(errs) == 0 {
		t.Fatal("期望捕获到缺少测试样例的错误，但未捕获")
	}
	if errs[0].ErrorType != ErrNoTestCase {
		t.Errorf("期望错误类型 %s，实际得到 %s", ErrNoTestCase, errs[0].ErrorType)
	}
}

// TestValidateRules_MatchFailed 测试样例无法匹配被捕获
func TestValidateRules_MatchFailed(t *testing.T) {
	rules := []consts.Rule{
		// 提供了不匹配的测试样例
		makeRule("phone", `\b1[3-9]\d{9}\b`, true, "not-a-phone-number"),
	}

	errs := ValidateRules(rules)
	if len(errs) == 0 {
		t.Fatal("期望捕获到测试样例不匹配的错误，但未捕获")
	}
	if errs[0].ErrorType != ErrMatchFailed {
		t.Errorf("期望错误类型 %s，实际得到 %s", ErrMatchFailed, errs[0].ErrorType)
	}
}

// TestValidateRules_BuiltinSkip 内置规则跳过 pattern/test_cases 检查
func TestValidateRules_BuiltinSkip(t *testing.T) {
	rules := []consts.Rule{
		// 内置规则：无 pattern，无 test_cases，但不应报错
		{Id: "domain", Enabled: true, Pattern: ""},
		{Id: "path", Enabled: true, Pattern: ""},
		{Id: "secret_key", Enabled: true, Pattern: ""},
		{Id: "ip", Enabled: true, Pattern: ""},
	}

	errs := ValidateRules(rules)
	if len(errs) != 0 {
		t.Errorf("内置规则不应产生验证错误，实际得到 %d 个：%v", len(errs), errs)
	}
}

// TestValidateRules_NoPattern 启用规则缺少 pattern 被捕获
func TestValidateRules_NoPattern(t *testing.T) {
	rules := []consts.Rule{
		makeRule("missing_pattern", "", true, "some input"),
	}

	errs := ValidateRules(rules)
	if len(errs) == 0 {
		t.Fatal("期望捕获到缺少 pattern 的错误，但未捕获")
	}
	if errs[0].ErrorType != ErrNoPattern {
		t.Errorf("期望错误类型 %s，实际得到 %s", ErrNoPattern, errs[0].ErrorType)
	}
}

// TestValidateRules_MultipleErrors 多条错误同时捕获
func TestValidateRules_MultipleErrors(t *testing.T) {
	rules := []consts.Rule{
		makeRule("r1", `[bad`, true, "input"),          // 坏正则
		makeRule("r2", `\b\d+\b`, true),                // 无测试样例
		makeRule("r3", `\b[a-z]+\b`, true, "12345678"), // 样例不匹配
	}

	errs := ValidateRules(rules)
	if len(errs) != 3 {
		t.Errorf("期望 3 个错误，实际得到 %d 个：%v", len(errs), errs)
	}
}
