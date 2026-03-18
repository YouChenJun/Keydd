package cmd

import (
	"Keydd/consts"
	"fmt"
	"regexp"
)

// 内置规则 ID 列表（无 pattern，跳过正则和测试样例检查）
var builtinRuleIDs = map[string]bool{
	"domain":     true,
	"path":       true,
	"domain_url": true,
	"ip":         true,
	"ip_url":     true,
	"secret_key": true,
}

// ValidationErrorType 验证错误类型
type ValidationErrorType string

const (
	// ErrCompile 正则表达式编译失败
	ErrCompile ValidationErrorType = "COMPILE_ERROR"
	// ErrNoTestCase 规则缺少测试样例
	ErrNoTestCase ValidationErrorType = "NO_TEST_CASE"
	// ErrMatchFailed 测试样例无法被规则匹配
	ErrMatchFailed ValidationErrorType = "MATCH_FAILED"
	// ErrNoPattern 启用的规则缺少 pattern
	ErrNoPattern ValidationErrorType = "NO_PATTERN"
)

// ValidationError 单条验证错误
type ValidationError struct {
	RuleId    string
	ErrorType ValidationErrorType
	Message   string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("[WARN] rule %-35s | %-15s | %s", e.RuleId, e.ErrorType, e.Message)
}

// ValidateRules 验证规则列表，返回所有验证错误
// 仅对 enabled=true 的规则进行验证，内置规则跳过 pattern/test_cases 检查
func ValidateRules(rules []consts.Rule) []ValidationError {
	var errs []ValidationError

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 内置规则：跳过 pattern 和 test_cases 检查
		if builtinRuleIDs[rule.Id] {
			continue
		}

		// 检查 pattern 是否为空
		if rule.Pattern == "" {
			errs = append(errs, ValidationError{
				RuleId:    rule.Id,
				ErrorType: ErrNoPattern,
				Message:   "规则已启用但缺少 pattern 字段",
			})
			continue
		}

		// 检查 pattern 是否能编译
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			errs = append(errs, ValidationError{
				RuleId:    rule.Id,
				ErrorType: ErrCompile,
				Message:   fmt.Sprintf("正则表达式编译失败: %v", err),
			})
			continue
		}

		// 检查是否有测试样例
		if len(rule.TestCases) == 0 {
			errs = append(errs, ValidationError{
				RuleId:    rule.Id,
				ErrorType: ErrNoTestCase,
				Message:   "规则缺少 test_cases，请至少添加一个测试样例",
			})
			continue
		}

		// 验证每个测试样例是否能被匹配
		for i, tc := range rule.TestCases {
			if !compiled.MatchString(tc) {
				errs = append(errs, ValidationError{
					RuleId:    rule.Id,
					ErrorType: ErrMatchFailed,
					Message:   fmt.Sprintf("第 %d 个测试样例未能匹配: %q", i+1, truncate(tc, 80)),
				})
			}
		}
	}

	return errs
}

// PrintValidationReport 打印验证报告到终端
func PrintValidationReport(rules []consts.Rule, errs []ValidationError) {
	enabledCount := 0
	for _, r := range rules {
		if r.Enabled && !builtinRuleIDs[r.Id] {
			enabledCount++
		}
	}

	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println("  Keydd Rule Validation Report")
	fmt.Println("─────────────────────────────────────────────────────")

	if len(errs) == 0 {
		fmt.Printf("  [OK] All %d rules passed validation\n", enabledCount)
		fmt.Println("─────────────────────────────────────────────────────")
		return
	}

	fmt.Printf("  Checked %d rules, found %d issue(s):\n\n", enabledCount, len(errs))
	for _, e := range errs {
		fmt.Printf("  %s\n", e.String())
	}
	fmt.Println("─────────────────────────────────────────────────────")
}

// truncate 截断过长字符串，用于日志展示
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
