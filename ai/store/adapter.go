// Package store 数据库抽象层
// 支持 SQLite 和 PostgreSQL 后端，用于存储流量特征和 AI 分析结果
package store

import "time"

// 流量分析状态常量
const (
	StatusPending  = "pending"  // 待分析
	StatusSkipped  = "skipped"  // 已跳过（如非 XHR 请求）
	StatusAnalyzed = "analyzed" // 分析完成
	StatusFailed   = "failed"   // 分析失败
)

// 任务状态常量
const (
	TaskStatusPending   = "pending"
	TaskStatusAnalyzing = "analyzing"
	TaskStatusDone      = "done"
	TaskStatusFailed    = "failed"
)

// DBAdapter 数据库适配器接口
type DBAdapter interface {
	// Init 初始化数据库（创建表等）
	Init() error
	// Close 关闭数据库连接
	Close() error
	// InsertSignature 插入流量特征记录
	// 如果 sig_key 已存在，返回已有记录的 ID 和 false
	// 如果是新记录，返回新 ID 和 true
	InsertSignature(rec *TrafficRecord) (id int64, isNew bool, err error)
	// UpdateAnalysisResult 更新 AI 分析结果
	UpdateAnalysisResult(sigKey string, result *AnalysisResult) error
	// ExistsBySigKey 检查 sig_key 是否已存在（不考虑状态）
	ExistsBySigKey(sigKey string) (bool, error)
	// ShouldAnalyze 根据 sig_key 查询状态，返回是否应跳过分析
	// - shouldSkip=true: 已有成功/跳过记录，应跳过
	// - shouldReinsert=true: 已有记录但分析失败，应重新分析（不插入新记录，调用方需处理）
	// - sigKey 不存在时返回 (false, false, nil)
	ShouldAnalyze(sigKey string) (shouldSkip bool, shouldReinsert bool, err error)
	// ListByStatus 按状态查询记录
	ListByStatus(status string, limit int) ([]*TrafficRecord, error)
	// ListByHostAnalyzed 按域名模式查询已分析的记录，按 penetration_priority DESC 排序
	// hostPattern 会被包装为 SQL LIKE 模式（如 "baidu.com" -> "%baidu.com%"），包含子域名
	ListByHostAnalyzed(hostPattern string, limit int) ([]*AnalyzedRecord, error)
	// GetByIDs 按 ID 列表查询记录的完整请求/响应数据（用于 POC 编写时获取完整包数据）
	GetByIDs(ids []int64) ([]*AnalyzedRecord, error)
	// ListAnalyzed 查询已分析的分析记录（按时间倒序）
	ListAnalyzed(limit int) ([]*AnalyzedRecord, error)

	// ============ Dashboard 统计相关 ============
	// GetStatistics 获取累计统计数据
	GetStatistics() (*Statistics, error)
	// IncrementStatistics 原子增加统计字段
	IncrementStatistics(field string, value int64) error
	// IncrementTokenStats 原子增加 Token 统计
	IncrementTokenStats(promptTokens, completionTokens, cachedTokens int64) error

	// ============ 分析记录分页查询 ============
	ListAnalyzedWithPaging(limit, offset int, statusFilter string) ([]*AnalyzedRecord, int64, error)

	// ============ 任务管理 ============
	CreateTask(task *AnalysisTask) error
	UpdateTaskStatus(taskID string, status string, progress int) error
	UpdateTaskError(taskID string, errMsg string) error
	ListActiveTasks() ([]*AnalysisTask, error)
	DeleteTask(taskID string) error
}

// TrafficRecord 流量记录（对应数据库行）
type TrafficRecord struct {
	ID             int64
	Host           string
	Path           string
	Method         string
	QueryParamKeys string
	BodySchemaHash string
	ContentType    string
	SampleRequest  string
	SampleResponse string
	SigKey         string
	Status         string // pending / skipped / analyzed / failed
}

// AnalysisResult AI 分析结果（用于 UPDATE 已有记录）
type AnalysisResult struct {
	SessionID           string
	BusinessName        string
	BusinessDescription string
	FunctionName        string
	Sensitivity         string
	AuthMechanism       string
	AnalysisContext     string
	PenetrationPriority int
	RiskLevel           string
	FinalSummary        string
}

// AnalyzedRecord 已分析的 API 接口记录（用于 Recon 查询）
type AnalyzedRecord struct {
	ID                  int64  `json:"id"`
	Host                string `json:"host"`
	Path                string `json:"path"`
	Method              string `json:"method"`
	BusinessName        string `json:"business_name"`
	BusinessDescription string `json:"business_description"`
	FunctionName        string `json:"function_name"`
	Sensitivity         string `json:"sensitivity"`
	AuthMechanism       string `json:"auth_mechanism"`
	AnalysisContext     string `json:"analysis_context"`
	PenetrationPriority int    `json:"penetration_priority"`
	SampleRequest       string `json:"sample_request"`
	SampleResponse      string `json:"sample_response"`
	RiskLevel           string `json:"risk_level"`
	FinalSummary        string `json:"final_summary"`
	AnalyzedAt          string `json:"analyzed_at"` // 分析时间 (RFC3339 格式)
}

// Statistics AI 分析累计统计
type Statistics struct {
	TotalRequests       int64 `json:"total_requests"`
	SuccessCount        int64 `json:"success_count"`
	FailureCount        int64 `json:"failure_count"`
	TimeoutCount        int64 `json:"timeout_count"`
	LLMErrorCount       int64 `json:"llm_error_count"`
	ParseErrorCount     int64 `json:"parse_error_count"`
	RateLimitedCount    int64 `json:"rate_limited_count"`
	TotalPromptTokens   int64 `json:"total_prompt_tokens"`
	TotalCompletionTokens int64 `json:"total_completion_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
	PromptCachedTokens  int64 `json:"prompt_cached_tokens"`
	TurnCount           int64 `json:"turn_count"`
}

// AnalysisTask 分析任务状态
type AnalysisTask struct {
	ID         string    `json:"id"`
	SigKey     string    `json:"sig_key"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	Method     string    `json:"method"`
	Status     string    `json:"status"`   // pending / analyzing / done / failed
	Progress   int       `json:"progress"` // 0-100
	ErrorMsg   string    `json:"error_msg,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}
