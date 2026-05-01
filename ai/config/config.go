package config

// ObservabilityConfig 可观测性配置（Langfuse）
type ObservabilityConfig struct {
	Enabled   bool   `yaml:"enabled"`
	PublicKey string `yaml:"public_key"`
	SecretKey string `yaml:"secret_key"`
	Host      string `yaml:"host"`
	Insecure  bool   `yaml:"insecure"`
}

// AIConfig AI 功能总配置
type AIConfig struct {
	Enabled       bool                `yaml:"enabled"`
	LLM           LLMConfig           `yaml:"llm"`
	Store         StoreConfig         `yaml:"store"`
	Analysis      AnalysisConfig      `yaml:"analysis"`
	Memory        MemoryConfig        `yaml:"memory"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// MySQLConfig MySQL 连接配置
type MySQLConfig struct {
	Host     string `yaml:"host"`      // 数据库地址
	Port     int    `yaml:"port"`      // 端口，默认 3306
	Username string `yaml:"username"`  // 用户名
	Password string `yaml:"password"`  // 密码
	Database string `yaml:"database"`  // 数据库名
	Charset  string `yaml:"charset"`  // 字符集，默认 utf8mb4
}

// StoreConfig 数据库存储配置
type StoreConfig struct {
	Type        string       `yaml:"type"`         // sqlite / postgres / mysql
	SQLitePath  string       `yaml:"sqlite_path"`  // SQLite 文件路径
	PostgresDSN string       `yaml:"postgres_dsn"` // PostgreSQL 连接字符串
	MySQL       *MySQLConfig `yaml:"mysql"`        // MySQL 配置
}

// LLMConfig 大模型连接配置，兼容 OpenAI 格式的接口
type LLMConfig struct {
	Model       string  `yaml:"model"`       // gpt-4o-mini / deepseek-chat 等
	APIKey      string  `yaml:"api_key"`     // 也可通过 OPENAI_API_KEY 环境变量设置
	BaseURL     string  `yaml:"base_url"`    // 默认 https://api.openai.com/v1
	RateLimit   RateLimitConfig `yaml:"rate_limit"` // 速率限制配置
}

// RateLimitConfig 速率限制配置
// 核心限制逻辑：
//   - MaxConcurrent: 同时 in-flight 的最大并发请求数（ semaphore 控制）
//   - WindowSecs + MaxInWindow: 滑动窗口限制（30s 内最多 N 个请求）
// 两者叠加：并发控制 + 时间窗口控制，双重保护
type RateLimitConfig struct {
	MaxConcurrent    int  `yaml:"max_concurrent"`      // 最大并发 in-flight 请求数，默认 3
	WindowSecs       int  `yaml:"window_secs"`        // 滑动时间窗口（秒），默认 30
	MaxInWindow      int  `yaml:"max_in_window"`      // 滑动窗口内最多 N 个请求，默认 50
	RetryOn429       bool `yaml:"retry_on_429"`        // 429 时是否自动重试，默认 true
	RetryMaxAttempts int  `yaml:"retry_max_attempts"`  // 最大重试次数，默认 3
	RetryBaseDelayMs int  `yaml:"retry_base_delay_ms"` // 指数退避初始延迟（毫秒），默认 2000
}

// AnalysisConfig 分析行为配置
type AnalysisConfig struct {
	// 业务分析
	BusinessAnalysisEnabled bool `yaml:"business_analysis_enabled"` // 是否进行业务逻辑分析

	// 过滤：只分析 XHR/fetch 请求
	OnlyAnalyzeXHR bool `yaml:"only_analyze_xhr"` // 默认 false

	// 去重控制：已分析过的接口不再重复分析
	DeduplicationEnabled bool `yaml:"deduplication_enabled"` // 默认 true（开启去重）

	// 记忆服务：启用后将分析结果存入 memory，用于跨接口关联分析
	MemoryAnalysisEnabled bool `yaml:"memory_analysis_enabled"` // 默认 false
}

// MemoryConfig Memory Service 持久化配置
// trpc-agent-go Memory Service 用于存储 Agent 会话记忆和业务理解
type MemoryConfig struct {
	Backend       string `yaml:"backend"`        // redis / inmemory
	RedisAddr     string `yaml:"redis_addr"`     // Redis 地址
	AutoExtract   bool   `yaml:"auto_extract"`   // 是否启用自动记忆提取，默认 true
	CheckInterval int    `yaml:"check_interval"` // 自动提取检查间隔（条数），默认 5
}

// DefaultAIConfig 返回默认 AI 配置
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		Enabled: false,
		LLM: LLMConfig{
			Model:   "gpt-4o-mini",
			APIKey:  "",
			BaseURL: "https://api.openai.com/v1",
			RateLimit: RateLimitConfig{
				MaxConcurrent:    3,
				WindowSecs:       30,
				MaxInWindow:      50,
				RetryOn429:       true,
				RetryMaxAttempts: 3,
				RetryBaseDelayMs: 2000,
			},
		},
		Store: StoreConfig{
			Type:       "sqlite",
			SQLitePath: "data_ai.db",
		},
		Analysis: AnalysisConfig{
			BusinessAnalysisEnabled: true,
			OnlyAnalyzeXHR:          false,
			DeduplicationEnabled:    true,
			MemoryAnalysisEnabled:   false,
		},
		Memory: MemoryConfig{
			Backend:       "inmemory",
			AutoExtract:   true,
			CheckInterval: 5,
		},
	}
}
