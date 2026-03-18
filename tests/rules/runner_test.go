package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// TestCase 单条测试用例
type TestCase struct {
	Input       string `yaml:"input"`
	ShouldMatch bool   `yaml:"should_match"`
}

// TestSuite 单个规则的测试套件
type TestSuite struct {
	RuleId string     `yaml:"rule_id"`
	Cases  []TestCase `yaml:"cases"`
}

// TestCasesFile testcases.yaml 文件结构
type TestCasesFile struct {
	TestSuites []TestSuite `yaml:"test_suites"`
}

// RuleYAML rule.yaml 中单条规则的结构
type RuleYAML struct {
	Id      string `yaml:"id"`
	Enabled bool   `yaml:"enabled"`
	Pattern string `yaml:"pattern"`
}

// RulesConfig rule.yaml 顶层结构
type RulesConfig struct {
	Rules []RuleYAML `yaml:"rules"`
}

// projectRoot 返回项目根目录（tests/rules/ 上两级）
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// loadTestCases 加载 testcases.yaml
func loadTestCases(t *testing.T) *TestCasesFile {
	t.Helper()
	path := filepath.Join(projectRoot(), "tests", "rules", "testcases.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("无法读取 testcases.yaml: %v", err)
	}
	var f TestCasesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		t.Fatalf("解析 testcases.yaml 失败: %v", err)
	}
	return &f
}

// loadRules 加载 config/rule.yaml
func loadRules(t *testing.T) map[string]string {
	t.Helper()
	path := filepath.Join(projectRoot(), "config", "rule.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("无法读取 config/rule.yaml: %v", err)
	}
	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("解析 config/rule.yaml 失败: %v", err)
	}
	patterns := make(map[string]string)
	for _, r := range config.Rules {
		if r.Enabled && r.Pattern != "" {
			patterns[r.Id] = r.Pattern
		}
	}
	return patterns
}

// TestAllRulesFromYAML 加载 testcases.yaml 和 rule.yaml，对所有规则运行匹配测试
func TestAllRulesFromYAML(t *testing.T) {
	tcFile := loadTestCases(t)
	patterns := loadRules(t)

	if len(tcFile.TestSuites) == 0 {
		t.Fatal("testcases.yaml 中没有任何测试套件")
	}

	passCount := 0
	failCount := 0

	for _, suite := range tcFile.TestSuites {
		suite := suite // capture loop variable
		t.Run(suite.RuleId, func(t *testing.T) {
			pattern, ok := patterns[suite.RuleId]
			if !ok {
				t.Skipf("规则 %q 在 rule.yaml 中未启用或无 pattern，跳过", suite.RuleId)
				return
			}

			compiled, err := regexp.Compile(pattern)
			if err != nil {
				t.Errorf("规则 %q 正则编译失败: %v", suite.RuleId, err)
				failCount++
				return
			}

			for i, tc := range suite.Cases {
				matched := compiled.MatchString(tc.Input)
				if matched != tc.ShouldMatch {
					desc := "应匹配但未匹配"
					if !tc.ShouldMatch {
						desc = "不应匹配但却匹配了"
					}
					t.Errorf(
						"用例 #%d %s\n  规则:  %s\n  输入:  %q\n  模式:  %s",
						i+1, desc, suite.RuleId, truncate(tc.Input, 100), pattern,
					)
					failCount++
				} else {
					passCount++
				}
			}
		})
	}

	fmt.Printf("\n规则测试结果: %d 通过 / %d 失败\n", passCount, failCount)
}

// TestRuleYAMLSyntax 验证 rule.yaml 文件语法是否正确可解析
func TestRuleYAMLSyntax(t *testing.T) {
	path := filepath.Join(projectRoot(), "config", "rule.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("无法读取 config/rule.yaml: %v", err)
	}
	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("config/rule.yaml 语法错误: %v", err)
	}
	if len(config.Rules) == 0 {
		t.Fatal("config/rule.yaml 中没有任何规则")
	}
	t.Logf("成功解析 %d 条规则", len(config.Rules))
}

// TestAllPatternCompilable 验证所有启用规则的正则表达式可编译
func TestAllPatternCompilable(t *testing.T) {
	patterns := loadRules(t)
	failCount := 0
	for id, pattern := range patterns {
		if _, err := regexp.Compile(pattern); err != nil {
			t.Errorf("规则 %q 正则编译失败: %v", id, err)
			failCount++
		}
	}
	if failCount == 0 {
		t.Logf("所有 %d 条规则的正则表达式均可编译", len(patterns))
	}
}

// truncate 截断过长字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
