package consts

// 白名单规则yaml对应关系
type Rule struct {
	Id        string   `yaml:"id"`
	Enabled   bool     `yaml:"enabled"`
	Pattern   string   `yaml:"pattern"`
	TestCases []string `yaml:"test_cases"`
}

// 排除规则
type ExcludeRule struct {
	Name      string `yaml:"name"`
	ID        string `yaml:"id"`
	Target    string `yaml:"target"`
	Content   string `yaml:"content"`
	SourceTag string `yaml:"source_tag"`
	Enabled   bool   `yaml:"enabled"`
}

// Rules 规则配置文件结构（rule.yaml）
// 只包含规则和飞书 webhook
type Rules struct {
	Rules        []Rule        `yaml:"rules"`
	ExcludeRules []ExcludeRule `yaml:"exclude_rules"`
	Lark_webhook string        `yaml:"lark_Webhook"`
}
