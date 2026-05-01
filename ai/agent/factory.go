// Package agent Agent 层
// 负责 Agent 工厂和初始化，所有 Agent 实例管理
package agent

import (
	"Keydd/ai/agent/prompts"
	"Keydd/ai/config"
	"Keydd/ai/store"
	"Keydd/ai/tools"
	"Keydd/ai/types"
	logger "Keydd/log"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	oteltrace "go.opentelemetry.io/otel/trace"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/memory/extractor"
	"trpc.group/trpc-go/trpc-agent-go/memory/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/memory/redis"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/telemetry/langfuse"
	atrace "trpc.group/trpc-go/trpc-agent-go/telemetry/trace"
)

// AgentFactory AI Agent 工厂
// 负责初始化 Agent 和模型，只保留单接口分析功能
type AgentFactory struct {
	cfg             config.AIConfig
	model           model.Model
	memService      memory.Service              // 持久化 Memory Service
	langfuseCleanup func(context.Context) error // Langfuse cleanup 函数，退出前调用
	rateLimiter     *RateLimiter                // LLM 请求速率限制器
	DB              store.DBAdapter             // 数据库适配器（持久化统计）

	// 单接口分析 Agent
	apiAnalyzerRunner runner.Runner // 预创建的 api_analysis runner，复用于每次 RunFullAnalysis
}

// TokenTracker Token 消耗追踪器
type TokenTracker struct {
	totalPromptTokens     atomic.Int64
	totalCompletionTokens atomic.Int64
	totalTokens           atomic.Int64
	promptCachedTokens    atomic.Int64
	turnCount             atomic.Int64
}

// NewTokenTracker 创建新的 Token 追踪器
func NewTokenTracker() *TokenTracker {
	return &TokenTracker{}
}

// Record 记录一次分析的 Token 消耗
func (t *TokenTracker) Record(promptTokens, completionTokens, cachedTokens int) {
	t.totalPromptTokens.Add(int64(promptTokens))
	t.totalCompletionTokens.Add(int64(completionTokens))
	t.totalTokens.Add(int64(promptTokens + completionTokens))
	t.promptCachedTokens.Add(int64(cachedTokens))
	t.turnCount.Add(1)
}

// GetSnapshot 获取当前 Token 统计快照
func (t *TokenTracker) GetSnapshot() TokenUsageSnapshot {
	return TokenUsageSnapshot{
		TotalPromptTokens:     t.totalPromptTokens.Load(),
		TotalCompletionTokens: t.totalCompletionTokens.Load(),
		TotalTokens:           t.totalTokens.Load(),
		PromptCachedTokens:    t.promptCachedTokens.Load(),
		TurnCount:             t.turnCount.Load(),
	}
}

// TokenUsageSnapshot Token 使用统计快照
type TokenUsageSnapshot struct {
	TotalPromptTokens     int64 `json:"total_prompt_tokens"`
	TotalCompletionTokens int64 `json:"total_completion_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
	PromptCachedTokens    int64 `json:"prompt_cached_tokens"`
	TurnCount             int64 `json:"turn_count"`
}

// NewAgentFactory 创建 Agent 工厂，初始化 LLM 模型和所有 Agent
func NewAgentFactory(cfg config.AIConfig) (*AgentFactory, error) {
	// 获取 API Key（优先级：配置 > 环境变量）
	apiKey := cfg.LLM.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API Key 未配置，请在 config/rule.yaml 中设置 ai.llm.api_key 或通过 OPENAI_API_KEY 环境变量设置")
	}

	// 构建 openai.Model 选项
	modelOpts := []openai.Option{
		openai.WithAPIKey(apiKey),
	}
	if cfg.LLM.BaseURL != "" {
		modelOpts = append(modelOpts, openai.WithBaseURL(cfg.LLM.BaseURL))
	}

	llmModel := openai.New(cfg.LLM.Model, modelOpts...)
	logger.Info.Printf("[AgentFactory] 初始化 LLM 模型: %s", cfg.LLM.Model)

	// 初始化 Langfuse 可观测性（如果启用）
	var cleanup func(context.Context) error = nil
	if cfg.Observability.Enabled {
		var opts []langfuse.Option
		if cfg.Observability.PublicKey != "" {
			opts = append(opts, langfuse.WithPublicKey(cfg.Observability.PublicKey))
		}
		if cfg.Observability.SecretKey != "" {
			opts = append(opts, langfuse.WithSecretKey(cfg.Observability.SecretKey))
		}
		if cfg.Observability.Host != "" {
			opts = append(opts, langfuse.WithHost(cfg.Observability.Host))
		}
		if cfg.Observability.Insecure {
			opts = append(opts, langfuse.WithInsecure())
		}
		var err error
		cleanup, err = langfuse.Start(context.Background(), opts...)
		if err != nil {
			logger.Warning.Printf("[AgentFactory] Failed to initialize Langfuse: %v", err)
			cleanup = nil
		} else {
			logger.Info.Println("[AgentFactory] Langfuse observability initialized")
		}
	}

	factory := &AgentFactory{
		cfg:             cfg,
		model:           llmModel,
		langfuseCleanup: cleanup,
	}

	// 初始化速率限制器（并发控制 + 429 重试）
	factory.rateLimiter = NewRateLimiter(cfg.LLM.RateLimit, atrace.Tracer)
	logger.Info.Printf("[AgentFactory] 速率限制器初始化: maxConcurrent=%d, windowSecs=%d, retryOn429=%v",
		cfg.LLM.RateLimit.MaxConcurrent, cfg.LLM.RateLimit.WindowSecs, cfg.LLM.RateLimit.RetryOn429)

	// ---- 初始化 Agent ----

	// api_analysis 专用 Agent（预创建，复用于每次 RunFullAnalysis，避免每次请求重新创建）
	maxTokens := 2000
	temperature := 0.1
	// 明确禁用深度思考，确保输出格式稳定（思考过程会破坏结构化输出）
	reasoningEffort := "" // o-series: 空字符串禁用 reasoning
	thinkingEnabled := false
	apiAnalyzerAgent := llmagent.New("api-analyzer",
		llmagent.WithModel(llmModel),
		llmagent.WithDescription("API业务逻辑分析助手"),
		llmagent.WithGlobalInstruction(prompts.GetSystemPrompt("api_analysis")),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			MaxTokens:       &maxTokens,
			Temperature:     &temperature,
			Stream:          true,
			ReasoningEffort: &reasoningEffort, // 禁用 OpenAI o-series 深度思考
			ThinkingEnabled: &thinkingEnabled, // 禁用 Claude/Gemini 思考模式
		}),
	)
	factory.apiAnalyzerRunner = runner.NewRunner("keydd-api-analysis", apiAnalyzerAgent)

	// ---- Memory Service（持久化存储上下文）----
	// Memory Service（保存分析上下文）- 支持持久化到数据库
	memService, err := factory.createMemoryService(cfg)
	if err != nil {
		logger.Warning.Printf("[AgentFactory] 创建持久化Memory失败，回退到内存: %v", err)
		memService = inmemory.NewMemoryService()
	}
	factory.memService = memService

	logger.Info.Printf("[AgentFactory] 初始化成功：Single Agent (only API traffic analysis)")

	return factory, nil
}

// RunFullAnalysis 入口进行 API 接口的分析
// 此接口进行 API 初步业务逻辑判断和初筛。不进行安全个分析
func (f *AgentFactory) RunFullAnalysis(ctx context.Context, sessionID string, sigID int64,
	host, method, path, sampleRequest, sampleResponse string) (*types.FullAnalysisResult, error) {

	if !f.cfg.Analysis.BusinessAnalysisEnabled {
		logger.Info.Printf("[AgentFactory] 业务分析功能未启用，跳过: %s", host)
		return nil, nil
	}

	userID := "keydd-system"
	// 直接构建 API 分析请求消息（不需要 Orchestrator）
	userMsg := tools.BuildApiAnalyzerMessage(host, method, path, sampleRequest, sampleResponse)

	// Memory Service: 搜索同 Host 的历史分析上下文，增强 LLM 分析质量
	var memoryContext string
	if f.cfg.Analysis.MemoryAnalysisEnabled && f.memService != nil {
		memoryKey := memory.UserKey{AppName: "keydd", UserID: host}
		entries, err := f.memService.SearchMemories(ctx, memoryKey, method+" "+path)
		if err != nil {
			logger.Warning.Printf("[AgentFactory] Memory search failed: %v", err)
		} else if len(entries) > 0 {
			// 提取历史分析上下文
			var ctxLines []string
			for _, e := range entries {
				if e.Memory != nil && e.Memory.Memory != "" {
					ctxLines = append(ctxLines, e.Memory.Memory)
				}
			}
			if len(ctxLines) > 0 {
				memoryContext = "\n\n【历史分析上下文】该 Host 已有分析记录：\n" + strings.Join(ctxLines, "\n---\n")
				userMsg += memoryContext
				logger.Info.Printf("[AgentFactory] Memory context loaded: %d entries for host %s", len(ctxLines), host)
			}
		}
	}

	fullHost := host + path
	logger.Info.Printf("[AgentFactory] 启动 API 接口分析 (sessionID: %s, host: %s)", sessionID, fullHost)

	// 添加 Langfuse 属性到 baggage，通过上下文传播
	mSession, err := baggage.NewMemberRaw("langfuse.session.id", sessionID)
	if err != nil {
		logger.Warning.Printf("[AgentFactory] failed to create session baggage: %v", err)
	}
	mUser, err := baggage.NewMemberRaw("langfuse.user.id", userID)
	if err != nil {
		logger.Warning.Printf("[AgentFactory] failed to create user baggage: %v", err)
	}
	bag, err := baggage.New(mSession, mUser)
	if err == nil {
		ctx = baggage.ContextWithBaggage(ctx, bag)
	}

	// 创建顶层 trace span（Langfuse 的 input/output 通过 span attributes 设置）
	traceInput := fmt.Sprintf("API分析: [%s] %s%s", method, host, path)
	ctx, span := atrace.Tracer.Start(ctx, "api-analyzer",
		oteltrace.WithAttributes(
			attribute.String("agentName", "api-analyzer"),
			attribute.String("modelName", f.cfg.LLM.Model),
			attribute.String("langfuse.trace.input", traceInput),
		),
	)
	defer span.End()

	// 运行并收集结果（复用预创建的 apiAnalyzerRunner，带速率限制）
	var rawResult string
	var promptTokens, outputTokens int
	var llmErr error
	isRateLimited := false

	// 包装函数用于速率限制器
	var agentResult AgentResult
	rawResult, llmErr, isRateLimited = f.rateLimiter.Execute(ctx, sessionID, func(rlCtx context.Context) (string, error) {
		content, pTokens, oTokens, cTokens, err := runAgent(rlCtx, f.apiAnalyzerRunner, userID, sessionID, userMsg)
		agentResult = AgentResult{
			Content:      content,
			PromptTokens: pTokens,
			OutputTokens: oTokens,
			CachedTokens: cTokens,
		}
		return content, err
	})
	rawResult = agentResult.Content
	promptTokens = agentResult.PromptTokens
	outputTokens = agentResult.OutputTokens

	// 记录 Token 消耗到数据库
	if promptTokens > 0 || outputTokens > 0 {
		if f.DB != nil {
			_ = f.DB.IncrementTokenStats(int64(promptTokens), int64(outputTokens), int64(agentResult.CachedTokens))
		}
		logger.Info.Printf("[AgentFactory] Token 消耗: prompt=%d, output=%d, cached=%d, total=%d",
			promptTokens, outputTokens, agentResult.CachedTokens, promptTokens+outputTokens)
	}

	// 记录总请求数
	if f.DB != nil {
		_ = f.DB.IncrementStatistics("total_requests", 1)
	}

	if llmErr != nil {
		span.SetAttributes(attribute.String("error", llmErr.Error()))
		// 区分超时错误、速率限制错误和其他 LLM 错误
		if ctx.Err() == context.DeadlineExceeded {
			if f.DB != nil {
				_ = f.DB.IncrementStatistics("timeout_count", 1)
			}
		} else if isRateLimited {
			if f.DB != nil {
				_ = f.DB.IncrementStatistics("rate_limited_count", 1)
			}
		} else {
			if f.DB != nil {
				_ = f.DB.IncrementStatistics("llm_error_count", 1)
			}
		}
		if f.DB != nil {
			_ = f.DB.IncrementStatistics("failure_count", 1)
		}
		return nil, fmt.Errorf("traffic analyzer run failed (session: %s): %w", sessionID, llmErr)
	}

	// 检查是否为 LLM 服务端错误（API 返回的错误文本，而非正常分析结果）
	if isLLMServiceError(rawResult) {
		span.SetAttributes(attribute.String("error", rawResult))
		if f.DB != nil {
			_ = f.DB.IncrementStatistics("failure_count", 1)
			_ = f.DB.IncrementStatistics("llm_error_count", 1)
		}
		return nil, fmt.Errorf("LLM 服务端返回错误 (session: %s): %s", sessionID, rawResult)
	}

	// 解析最终结果
	result := &types.FullAnalysisResult{
		SessionID:    sessionID,
		TrafficSigID: sigID,
		FinalSummary: rawResult,
	}

	// 尝试从原始文本中提取结构化 JSON
	jsonStr := tools.ExtractJSON(rawResult)
	if jsonStr != "" {
		// 优先尝试直接解析为 TrafficAnalyzerOutput（LLM 直接返回业务 JSON 的情况）
		var trafficOutput types.TrafficAnalyzerOutput
		if err := json.Unmarshal([]byte(jsonStr), &trafficOutput); err == nil && trafficOutput.FunctionName != "" {
			result.TrafficAnalysis = trafficOutput
		} else {
			// 尝试解析为 Chat Completion 格式（message.content 中包含业务 JSON）
			var chatResp struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &chatResp); err == nil && len(chatResp.Message.Content) > 0 {
				// content 可能是 JSON 对象或字符串
				var innerOutput types.TrafficAnalyzerOutput
				if err := json.Unmarshal(chatResp.Message.Content, &innerOutput); err == nil {
					result.TrafficAnalysis = innerOutput
				} else {
					// content 是字符串形式的 JSON，先解引号再解析
					var contentStr string
					if err := json.Unmarshal(chatResp.Message.Content, &contentStr); err == nil {
						innerJSON := tools.ExtractJSON(contentStr)
						if innerJSON != "" {
							json.Unmarshal([]byte(innerJSON), &innerOutput)
							result.TrafficAnalysis = innerOutput
						}
					}
				}
			} else {
				// 最后尝试解析为 FullAnalysisResult（兼容旧格式）
				var parsed types.FullAnalysisResult
				if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
					parsed.SessionID = sessionID
					parsed.TrafficSigID = sigID
					parsed.FinalSummary = rawResult
					result = &parsed
				}
			}
		}
	}

	// 回退：如果结构化解析失败，至少把原始输出放到描述字段，避免全空
	parseFailed := result.TrafficAnalysis.FunctionName == "" && result.TrafficAnalysis.BusinessDesc == ""
	if parseFailed {
		// 将原始文本作为描述，这样数据库中不会完全是空的
		result.TrafficAnalysis.BusinessDesc = rawResult
		// 尝试启发式提取风险级别到 sensitivity
		if result.TrafficAnalysis.Sensitivity == "" {
			sensitivity := tools.ExtractRiskLevel(rawResult)
			if sensitivity != "" {
				result.TrafficAnalysis.Sensitivity = sensitivity
			} else {
				result.TrafficAnalysis.Sensitivity = "low"
			}
		}
	}

	// 总是尝试启发式提取 penetration_priority（即使结构化解析成功了，但该字段为 0）
	if result.TrafficAnalysis.PenetrationPriority == 0 {
		priority := tools.ExtractPenetrationPriority(rawResult)
		if priority > 0 {
			result.TrafficAnalysis.PenetrationPriority = priority
		}
	}

	// 从原始文本中提取风险级别（简单启发式）
	if result.OverallRiskLevel == "" {
		result.OverallRiskLevel = tools.ExtractRiskLevel(rawResult)
	}

	logger.Info.Printf("[AgentFactory] API 接口分析完成 (session: %s, risk: %s)", sessionID, result.OverallRiskLevel)

	// 设置 Langfuse trace output
	outputJSON, _ := json.Marshal(result.TrafficAnalysis)
	span.SetAttributes(attribute.String("langfuse.trace.output", string(outputJSON)))

	// Memory Service: 将分析结果存入记忆，用于跨接口关联分析
	if f.cfg.Analysis.MemoryAnalysisEnabled && f.memService != nil && result.TrafficAnalysis.FunctionName != "" {
		memoryKey := memory.UserKey{AppName: "keydd", UserID: host}
		memoryStr := fmt.Sprintf("[%s] %s: %s | 敏感度: %s | 认证: %s | 优先级: %d | %s",
			method, path, result.TrafficAnalysis.FunctionName,
			result.TrafficAnalysis.Sensitivity,
			result.TrafficAnalysis.AuthMechanism,
			result.TrafficAnalysis.PenetrationPriority,
			result.TrafficAnalysis.BusinessDesc,
		)
		topics := []string{"api-analysis", method, "fact"}
		if err := f.memService.AddMemory(ctx, memoryKey, memoryStr, topics); err != nil {
			logger.Warning.Printf("[AgentFactory] Memory store failed: %v", err)
		} else {
			logger.Info.Printf("[AgentFactory] Memory stored for %s %s", host, path)
		}
	}

	// 记录解析失败（LLM 调用成功但结构化解析失败）
	if parseFailed {
		if f.DB != nil {
			_ = f.DB.IncrementStatistics("parse_error_count", 1)
			_ = f.DB.IncrementStatistics("failure_count", 1)
		}
	} else {
		if f.DB != nil {
			_ = f.DB.IncrementStatistics("success_count", 1)
		}
	}

	return result, nil
}

// AgentResult Agent 执行结果
type AgentResult struct {
	Content      string
	PromptTokens int
	OutputTokens int
	CachedTokens int
}

// runAgent 执行 Agent 并收集完整的流式响应文本和 Token 使用情况
func runAgent(ctx context.Context, r runner.Runner, userID, sessionID, message string) (string, int, int, int, error) {
	msg := model.Message{
		Role:    model.RoleUser,
		Content: message,
	}

	eventCh, err := r.Run(ctx, userID, sessionID, msg)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("agent run failed: %w", err)
	}

	var sb strings.Builder
	var promptTokens, outputTokens, cachedTokens int

	for event := range eventCh {
		if event == nil || event.Response == nil {
			continue
		}

		// 收集 Token 使用信息（使用最终响应的 Usage）
		if event.Response.Usage != nil {
			promptTokens = event.Response.Usage.PromptTokens
			outputTokens = event.Response.Usage.CompletionTokens
			cachedTokens = event.Response.Usage.PromptTokensDetails.CachedTokens
		}

		for _, choice := range event.Response.Choices {
			// 优先使用 Delta（流式内容），再使用 Message（最终内容）
			if choice.Delta.Content != "" {
				sb.WriteString(choice.Delta.Content)
			} else if choice.Message.Content != "" && event.Response.Done {
				sb.WriteString(choice.Message.Content)
			}
		}
	}

	return strings.TrimSpace(sb.String()), promptTokens, outputTokens, cachedTokens, nil
}

// Close 关闭工厂（清理资源）
func (f *AgentFactory) Close() error {
	if f.apiAnalyzerRunner != nil {
		_ = f.apiAnalyzerRunner.Close()
	}
	// 调用 Langfuse cleanup - 确保所有 spans 都发送完成
	if f.langfuseCleanup != nil {
		_ = f.langfuseCleanup(context.Background())
	}
	return nil
}

// createMemoryService 根据配置创建持久化 Memory Service
// 支持 redis / inmemory 后端
func (f *AgentFactory) createMemoryService(cfg config.AIConfig) (memory.Service, error) {
	var service memory.Service
	var err error

	switch cfg.Memory.Backend {
	case "redis":
		// redis 已经有 options 构造器
		var opts []redis.ServiceOpt
		if cfg.Memory.RedisAddr != "" {
			// redis uses WithRedisClientURL not WithAddress
			opts = append(opts, redis.WithRedisClientURL(cfg.Memory.RedisAddr))
		}
		if cfg.Memory.AutoExtract && f.model != nil {
			checkInterval := cfg.Memory.CheckInterval
			if checkInterval <= 0 {
				checkInterval = 5
			}
			checker := extractor.CheckMessageThreshold(checkInterval)
			extractor := extractor.NewExtractor(f.model, extractor.WithChecker(checker))
			opts = append(opts, redis.WithExtractor(extractor))
		}
		opts = append(opts,
			redis.WithToolEnabled("memory_add", true),
			redis.WithToolEnabled("memory_search", true),
			redis.WithToolEnabled("memory_update", true),
			redis.WithToolEnabled("memory_load", true),
		)
		service, err = redis.NewService(opts...)
		if err != nil {
			return nil, err
		}
	default:
		// fallback 到 inmemory
		return inmemory.NewMemoryService(), nil
	}

	if err != nil {
		return nil, fmt.Errorf("create memory service failed: %w", err)
	}

	return service, nil
}

// GetMemoryService 获取 Memory Service（用于测试）
func (f *AgentFactory) GetMemoryService() memory.Service {
	return f.memService
}

// GetModel 获取 LLM 模型实例（供工具使用）
func (f *AgentFactory) GetModel() model.Model {
	return f.model
}

// isLLMServiceError 检查 LLM 返回的文本是否为服务端错误（非正常分析结果）
func isLLMServiceError(text string) bool {
	if text == "" {
		return true
	}
	// 已知的 LLM 服务端错误特征
	errorPatterns := []string{
		"An error occurred during execution",
		"Please contact the service provider",
		"Internal server error",
		"Rate limit exceeded",
		"服务内部错误",
		"请求过于频繁",
		"content_filter",
	}
	for _, pattern := range errorPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}
