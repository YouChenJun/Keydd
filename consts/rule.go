package consts

// 白名单规则yaml对应关系
type Rule struct {
	Id      string `yaml:"id"`
	Enabled bool   `yaml:"enabled"`
	Pattern string `yaml:"pattern"`
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
type Rules struct {
	Rules        []Rule        `yaml:"rules"`
	ExcludeRules []ExcludeRule `yaml:"exclude_rules"`
	Lark_webhook string        `yaml:"lark_Webhook"`
}
