# Keydd AI 模块实现说明

## 概述

Keydd AI 模块负责对 MITMProxy 捕获的 HTTP 流量进行智能分析，支持 API 业务逻辑分析、Token 统计和数据库持久化。

## 目录结构

```
ai/
├── bootstrap.go      # AI 系统初始化入口
├── config/          # 配置定义 (AIConfig, LLMConfig, StoreConfig 等)
├── agent/           # Agent 核心模块
│   ├── factory.go   # Agent 工厂，LLM 初始化
│   ├── ratelimiter.go  # 速率限制器
│   ├── metrics.go   # 分析指标
│   ├── token_tracker.go # Token 追踪
│   └── prompts/     # Prompt 模板
├── store/          # 数据库持久化
│   ├── adapter.go  # 适配器接口
│   ├── factory.go  # 工厂方法
│   ├── sqlite.go   # SQLite 驱动
│   ├── mysql.go    # MySQL 驱动
│   ├── postgres.go  # PostgreSQL 驱动
│   └── schema.go   # 表结构定义
├── tools/          # 工具函数
└── types/          # 公共类型定义
```

## 核心功能

### 1. Agent 工厂 (agent/factory.go)

负责初始化 LLM 连接和创建 API 分析 Agent：

- 支持 OpenAI 格式的 API（OpenAI、DeepSeek、Ollama 等）
- 禁用深度思考，确保输出格式稳定
- 注入 Memory Service 用于上下文增强

**主要接口：**

```go
// 创建 Agent 工厂
func NewAgentFactory(cfg config.AIConfig) (*AgentFactory, error)

// 执行 API 接口分析
func (f *AgentFactory) RunFullAnalysis(
    ctx context.Context,
    sessionID string,
    sigID int64,
    host, method, path string,
    sampleRequest, sampleResponse string,
) (*types.FullAnalysisResult, error)

// 关闭工厂，释放资源
func (f *AgentFactory) Close() error
```

### 2. 速率限制器 (agent/ratelimiter.go)

防止 LLM API 请求超出限制：

- **并发控制**：Semaphore 限制同时 in-flight 请求数
- **滑动窗口**：时间窗口内最大请求数
- **429 自动重试**：指数退避策略

**配置项：**

```yaml
ai:
  llm:
    rate_limit:
      max_concurrent: 3      # 最大并发
      window_secs: 30        # 滑动窗口（秒）
      max_in_window: 50      # 窗口内最大请求数
      retry_on_429: true     # 429 时自动重试
      retry_max_attempts: 3  # 最大重试次数
```

### 3. Token 追踪 (agent/token_tracker.go)

记录 LLM API 的 Token 消耗：

- Prompt Tokens
- Completion Tokens
- Cached Tokens
- Turn Count

**使用方式：**

```go
tracker := NewTokenTracker()
// 分析完成后
snapshot := tracker.GetSnapshot()
fmt.Printf("Total Tokens: %d\n", snapshot.TotalTokens)
```

### 4. 分析指标 (agent/metrics.go)

收集 AI 流量分析的运行指标：

| 指标 | 说明 |
|------|------|
| total_requests | 总请求数 |
| success_count | 成功数 |
| failure_count | 失败数 |
| timeout_count | 超时数 |
| llm_error_count | LLM 错误数 |
| parse_error_count | 解析错误数 |
| rate_limited_count | 429 限流数 |
| success_rate | 成功率 |
| latency_p95_ms | P95 延迟 |

### 5. 数据库持久化 (store/)

支持多种数据库后端：

| 驱动 | 特点 |
|------|------|
| SQLite | 默认，轻量级，无需配置 |
| MySQL | 生产环境，高并发 |
| PostgreSQL | 生产环境，复杂查询 |

**存储内容：**

- Token 统计记录
- 请求统计
- 分析结果（可选）

**使用方式：**

```go
// 通过配置自动选择驱动
db, err := store.NewDBAdapter(store.Config{Type: "sqlite", SQLitePath: "data_ai.db"})
db.Init()

// 记录 Token 消耗
db.IncrementTokenStats(promptTokens, completionTokens, cachedTokens)

// 记录统计数据
db.IncrementStatistics("total_requests", 1)
```

### 6. Memory Service

可选的上下文记忆服务，支持同 Host 历史分析：

- **Redis**：分布式存储，需要 Redis 服务
- **InMemory**：进程内存储，重启丢失

**配置：**

```yaml
ai:
  memory:
    backend: "inmemory"  # redis / inmemory
    redis_addr: "redis://localhost:6379"
    auto_extract: true   # 自动提取记忆
    check_interval: 5    # 检查间隔
```

### 7. 可观测性 (Langfuse)

可选的 LLM 调用追踪：

```yaml
ai:
  observability:
    enabled: true
    public_key: "pk-xxx"
    secret_key: "sk-xxx"
    host: "https://cloud.langfuse.com"  # 自托管或云服务
```

追踪内容：
- 请求输入/输出
- Token 消耗
- 执行时间
- 错误信息

## 配置完整示例

```yaml
ai:
  enabled: true

  # LLM 配置
  llm:
    model: "gpt-4o-mini"
    api_key: "sk-xxx"  # 或通过 OPENAI_API_KEY 环境变量
    base_url: "https://api.openai.com/v1"

    # 速率限制
    rate_limit:
      max_concurrent: 3
      window_secs: 30
      max_in_window: 50
      retry_on_429: true
      retry_max_attempts: 3
      retry_base_delay_ms: 2000

  # 数据库存储
  store:
    type: "sqlite"  # sqlite / postgres / mysql
    sqlite_path: "data_ai.db"
    # postgres_dsn: "postgres://user:pass@localhost:5432/keydd"
    # mysql:
    #   host: "localhost"
    #   port: 3306
    #   username: "root"
    #   password: "xxx"
    #   database: "keydd"

  # 分析配置
  analysis:
    business_analysis_enabled: true
    only_analyze_xhr: false
    deduplication_enabled: true
    memory_analysis_enabled: false

  # Memory Service
  memory:
    backend: "inmemory"
    redis_addr: "redis://localhost:6379"
    auto_extract: true
    check_interval: 5

  # 可观测性
  observability:
    enabled: false
    public_key: ""
    secret_key: ""
    host: ""
```

## 数据流

```
HTTP 流量
    ↓
代理层捕获请求
    ↓
敏感信息检测
    ↓
AI Agent 分析
    ├─ Token 追踪 → 数据库
    ├─ 速率限制 → LLM API
    ├─ Memory 存储 → Redis/InMemory
    └─ 结果存储 → SQLite/MySQL/PostgreSQL
```

## 依赖

| 依赖 | 用途 |
|------|------|
| trpc-agent-go | Agent 框架 |
| go.opentelemetry.io/otel | 追踪 |
| gorm.io | ORM |
| modernc.org/sqlite | SQLite 驱动 |
