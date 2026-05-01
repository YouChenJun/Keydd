package cmd

import (
	"Keydd/consts"
	"regexp"
	"testing"
)

// TestMatchRulesPattern_JWTToken 测试 JWT Token 规则匹配
func TestMatchRulesPattern_JWTToken(t *testing.T) {
	// JWT Token 规则
	jwtPattern := regexp.MustCompile(`(eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[-_a-zA-Z0-9+/=]*)`)

	jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	matches := jwtPattern.FindAllStringSubmatch(jwtToken, -1)
	if len(matches) == 0 {
		t.Errorf("JWT Token 规则匹配失败")
	}
}

// TestMatchRulesPattern_APIKey 测试 API Key 规则匹配
func TestMatchRulesPattern_APIKey(t *testing.T) {
	// API Key 规则
	apiKeyPattern := regexp.MustCompile(`(?i)(api[_-]?key|apikey)[\s]*=[\s]*['""]?([a-zA-Z0-9_-]{20,})['""]?`)

	body := `api_key = "sk_test_abcdefghijklmnopqrstuvwxyz123456"`
	matches := apiKeyPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Errorf("API Key 规则匹配失败")
	}
}

// TestMatchRulesPattern_Password 测试密码规则匹配
func TestMatchRulesPattern_Password(t *testing.T) {
	// 简化的密码规则
	passwordPattern := regexp.MustCompile(`password["\']?\s*:\s*["']?([^"'}\s,]+)`)

	body := `{"password": "MySecurePass123"}`
	matches := passwordPattern.FindAllStringSubmatch(body, -1)
	if len(matches) < 1 {
		t.Errorf("密码规则匹配失败，期望至少 1 个匹配，实际 %d 个", len(matches))
	}
}

// TestMatchRulesPattern_AWSKey 测试 AWS Key 规则匹配
func TestMatchRulesPattern_AWSKey(t *testing.T) {
	// AWS Key 规则
	awsKeyPattern := regexp.MustCompile(`AKIA[0-9A-Z]{16}`)

	body := `credentials: AKIA2Z7K8Q9R5X2YBCDE`
	matches := awsKeyPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Errorf("AWS Key 规则匹配失败")
	}
}

// TestMatchRulesPattern_PrivateKey 测试私钥规则匹配
func TestMatchRulesPattern_PrivateKey(t *testing.T) {
	// 简化的私钥规则（Go regexp 不支持反向引用）
	privateKeyPattern := regexp.MustCompile(`-----BEGIN.*PRIVATE KEY-----`)

	key := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds0gHPE0+1f/dPvLs9gABCDEF
GHIJKLMNOPQRSTUVWXYZ123456
-----END RSA PRIVATE KEY-----`

	matches := privateKeyPattern.FindAllStringSubmatch(key, -1)
	if len(matches) == 0 {
		t.Errorf("私钥规则匹配失败")
	}
}

// TestMatchRulesPattern_DatabaseConnectionString 测试数据库连接字符串规则匹配
func TestMatchRulesPattern_DatabaseConnectionString(t *testing.T) {
	// 数据库连接字符串规则
	dbPattern := regexp.MustCompile(`(mysql|postgres|mongodb)://[^@]+:([^@]+)@[^/]+/\w+`)

	connStr := `postgres://user:SecurePass123@db.internal:5432/mydb`
	matches := dbPattern.FindAllStringSubmatch(connStr, -1)
	if len(matches) == 0 {
		t.Errorf("数据库连接字符串规则匹配失败")
	}
}

// TestMatchRulesPattern_WebhookURL 测试 Webhook URL 规则匹配
func TestMatchRulesPattern_WebhookURL(t *testing.T) {
	// Webhook URL 规则
	webhookPattern := regexp.MustCompile(`(https?://hooks\.(slack|discord|feishu)\.com/[^"'\s]+)`)

	url := `https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX`
	matches := webhookPattern.FindAllStringSubmatch(url, -1)
	if len(matches) == 0 {
		t.Errorf("Webhook URL 规则匹配失败")
	}
}

// TestMatchRulesPattern_NoMatch 测试未匹配的内容
func TestMatchRulesPattern_NoMatch(t *testing.T) {
	jwtPattern := regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[-_a-zA-Z0-9+/=]*`)

	body := `This is just normal text without any sensitive data`
	matches := jwtPattern.FindAllStringSubmatch(body, -1)
	if len(matches) != 0 {
		t.Errorf("规则不应该匹配，但找到 %d 个匹配", len(matches))
	}
}

// TestMatchRulesPattern_MultipleMatches 测试多个匹配
func TestMatchRulesPattern_MultipleMatches(t *testing.T) {
	passwordPattern := regexp.MustCompile(`password["\']?\s*:\s*["']?([^"'}\s,]+)`)

	body := `{
		"password": "Pass1",
		"db_password": "Pass2",
		"api_password": "Pass3"
	}`

	matches := passwordPattern.FindAllStringSubmatch(body, -1)
	if len(matches) != 3 {
		t.Errorf("期望 3 个匹配，实际 %d 个", len(matches))
	}
}

// TestMatchRulesPattern_SpecialCharacters 测试特殊字符
func TestMatchRulesPattern_SpecialCharacters(t *testing.T) {
	keyPattern := regexp.MustCompile(`key=([^&\s]+)`)

	body := `key=abc!@#$%^&*()`
	matches := keyPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Errorf("特殊字符匹配失败")
	}
}

// TestMatchRulesPattern_Unicode 测试 Unicode 字符
func TestMatchRulesPattern_Unicode(t *testing.T) {
	rulePattern := regexp.MustCompile(`secret[=:]\s*(.+)`)

	body := `secret: 密钥信息🔐`
	matches := rulePattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Errorf("Unicode 匹配失败")
	}
}

// TestMatchRulesPattern_CaseInsensitive 测试不区分大小写
func TestMatchRulesPattern_CaseInsensitive(t *testing.T) {
	passwordPattern := regexp.MustCompile(`(?i)(password|passwd|pwd)`)

	testCases := []string{
		"Password",
		"PASSWORD",
		"password",
		"PaSsWoRd",
		"PASSWD",
		"pwd",
		"PWD",
	}

	for _, testCase := range testCases {
		matches := passwordPattern.FindAllStringSubmatch(testCase, -1)
		if len(matches) == 0 {
			t.Errorf("不区分大小写匹配失败: %s", testCase)
		}
	}
}

// TestMatchRulesPattern_LongContent 测试长内容
func TestMatchRulesPattern_LongContent(t *testing.T) {
	apiKeyPattern := regexp.MustCompile(`sk_live_[a-zA-Z0-9]{20,}`)

	// 创建包含 API Key 的长字符串
	longBody := `This is a very long string with lots of content. `
	for i := 0; i < 100; i++ {
		longBody += `Some more content without sensitive data. `
	}
	longBody += `Here is the API key: sk_live_1234567890abcdefghij123`

	matches := apiKeyPattern.FindAllStringSubmatch(longBody, -1)
	if len(matches) == 0 {
		t.Errorf("长内容中的 API Key 匹配失败")
	}
}

// TestRuleKeyinfo 测试 Keyinfo 结构体
func TestRuleKeyinfo(t *testing.T) {
	keyinfo := &consts.Keyinfo{
		RuleName:     "test_rule",
		Host:         "example.com",
		Req_Path:     "/api/test",
		Req_Body:     []byte(`{"data":"value"}`),
		Res_Body:     []byte(`{"result":"ok"}`),
		Key_text:     "secret_key_12345",
		Content_Type: "application/json",
	}

	// 验证结构体字段
	if keyinfo.RuleName == "" {
		t.Errorf("RuleName 为空")
	}
	if keyinfo.Host == "" {
		t.Errorf("Host 为空")
	}
	if len(keyinfo.Req_Body) == 0 {
		t.Errorf("Req_Body 为空")
	}
	if keyinfo.Key_text == "" {
		t.Errorf("Key_text 为空")
	}
}

// TestRulesLoading 测试规则加载
func TestRulesLoading(t *testing.T) {
	// 测试规则是否能加载到 consts.LodaRules
	consts.LodaRules = make(map[string]*regexp.Regexp)

	// 添加一些规则
	jwtPattern, err := regexp.Compile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[-_a-zA-Z0-9+/=]*`)
	if err != nil {
		t.Errorf("正则表达式编译失败: %v", err)
	}

	apiKeyPattern, err := regexp.Compile(`sk_[a-zA-Z0-9]{20,}`)
	if err != nil {
		t.Errorf("正则表达式编译失败: %v", err)
	}

	consts.LodaRules["jwt"] = jwtPattern
	consts.LodaRules["api_key"] = apiKeyPattern

	// 验证规则已加载
	if len(consts.LodaRules) != 2 {
		t.Errorf("规则加载失败，期望 2 个规则，实际 %d 个", len(consts.LodaRules))
	}

	if consts.LodaRules["jwt"] == nil {
		t.Errorf("jwt 规则未加载")
	}

	if consts.LodaRules["api_key"] == nil {
		t.Errorf("api_key 规则未加载")
	}
}

// TestRegexCompilation 测试正则表达式编译
func TestRegexCompilation(t *testing.T) {
	patterns := []string{
		`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[-_a-zA-Z0-9+/=]*`,
		`sk_[a-zA-Z0-9]{20,}`,
		`(?i)(password|passwd|pwd)[\s]*[:=]`,
		`AKIA[0-9A-Z]{16}`,
		`-----BEGIN (RSA|DSA|EC) PRIVATE KEY-----`,
	}

	for _, pattern := range patterns {
		_, err := regexp.Compile(pattern)
		if err != nil {
			t.Errorf("正则表达式编译失败: %s, 错误: %v", pattern, err)
		}
	}
}

// TestRegexMatchFindAll 测试正则表达式的 FindAllStringSubmatch
func TestRegexMatchFindAll(t *testing.T) {
	pattern := regexp.MustCompile(`password[=:]\s*["']?([^"'}\s,]+)["']?`)

	body := `password="secret1", password: secret2, password=secret3`
	matches := pattern.FindAllStringSubmatch(body, -1)

	if len(matches) != 3 {
		t.Errorf("期望找到 3 个匹配，实际 %d 个", len(matches))
	}
}
