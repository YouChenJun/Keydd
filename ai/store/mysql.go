package store

import (
	"database/sql"
	"fmt"
	"time"

	"Keydd/ai/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TrafficAnalysis GORM 模型
type TrafficAnalysis struct {
	ID                  int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	Host                string       `gorm:"type:varchar(255);not null" json:"host"`
	Path                string       `gorm:"type:varchar(1024);not null" json:"path"`
	Method              string       `gorm:"type:varchar(10);not null" json:"method"`
	QueryParamKeys      string       `gorm:"type:varchar(512);default:''" json:"query_param_keys"`
	BodySchemaHash      string       `gorm:"type:varchar(128);default:''" json:"body_schema_hash"`
	ContentType         string       `gorm:"type:varchar(128);default:''" json:"content_type"`
	SampleRequest       string       `gorm:"type:mediumtext" json:"sample_request"`
	SampleResponse      string       `gorm:"type:mediumtext" json:"sample_response"`
	SigKey              string       `gorm:"type:varchar(512);not null;uniqueIndex" json:"sig_key"`
	Status              string       `gorm:"type:varchar(32);not null;default:pending;index" json:"status"`
	SessionID           string       `gorm:"type:varchar(128);default:''" json:"session_id"`
	BusinessName        string       `gorm:"type:varchar(256);default:''" json:"business_name"`
	BusinessDescription string       `gorm:"type:text" json:"business_description"`
	FunctionName        string       `gorm:"type:varchar(256);default:''" json:"function_name"`
	Sensitivity         string       `gorm:"type:varchar(32);default:''" json:"sensitivity"`
	AuthMechanism       string       `gorm:"type:varchar(64);default:''" json:"auth_mechanism"`
	AnalysisContext     string       `gorm:"type:text" json:"analysis_context"`
	PenetrationPriority int         `gorm:"default:0" json:"penetration_priority"`
	RiskLevel           string       `gorm:"type:varchar(32);default:''" json:"risk_level"`
	FinalSummary        string       `gorm:"type:text" json:"final_summary"`
	CreatedAt           time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
	AnalyzedAt          sql.NullTime `gorm:"default:null" json:"analyzed_at"`
}

// TableName 指定表名为 traffic_analysis（与 schema.go 一致）
func (TrafficAnalysis) TableName() string {
	return "traffic_analysis"
}

// AIStatistics 统计表 GORM 模型
type AIStatistics struct {
	ID                    int64 `gorm:"primaryKey;autoIncrement"`
	TotalRequests         int64 `gorm:"default:0"`
	SuccessCount          int64 `gorm:"default:0"`
	FailureCount          int64 `gorm:"default:0"`
	TimeoutCount          int64 `gorm:"default:0"`
	LLMErrorCount         int64 `gorm:"default:0"`
	ParseErrorCount       int64 `gorm:"default:0"`
	RateLimitedCount      int64 `gorm:"default:0"`
	TotalPromptTokens     int64 `gorm:"default:0"`
	TotalCompletionTokens int64 `gorm:"default:0"`
	TotalTokens           int64 `gorm:"default:0"`
	PromptCachedTokens    int64 `gorm:"default:0"`
	TurnCount             int64 `gorm:"default:0"`
	UpdatedAt             time.Time `gorm:"autoUpdateTime"`
}

func (AIStatistics) TableName() string {
	return "ai_statistics"
}

// AnalysisTaskGORM 任务表 GORM 模型
type AnalysisTaskGORM struct {
	ID         string    `gorm:"primaryKey;type:varchar(64)"`
	SigKey     string    `gorm:"type:varchar(512);index"`
	Host       string    `gorm:"type:varchar(255)"`
	Path       string    `gorm:"type:varchar(1024)"`
	Method     string    `gorm:"type:varchar(16)"`
	Status     string    `gorm:"type:varchar(32);index"`
	Progress   int       `gorm:"default:0"`
	ErrorMsg   string    `gorm:"type:text"`
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

func (AnalysisTaskGORM) TableName() string {
	return "analysis_tasks"
}

// MySQLAdapter MySQL 数据库适配器
type MySQLAdapter struct {
	db  *gorm.DB
	cfg *config.MySQLConfig
}

// NewMySQLAdapter 创建 MySQL 适配器
func NewMySQLAdapter(cfg *config.MySQLConfig) *MySQLAdapter {
	return &MySQLAdapter{cfg: cfg}
}

func (a *MySQLAdapter) dsn() string {
	charset := a.cfg.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	port := a.cfg.Port
	if port == 0 {
		port = 3306
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		a.cfg.Username, a.cfg.Password, a.cfg.Host, port, a.cfg.Database, charset)
}

func (a *MySQLAdapter) Init() error {
	// 关闭默认日志，只在错误时打印
	db, err := gorm.Open(mysql.Open(a.dsn()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	a.db = db

	// 自动建表
	if err := db.AutoMigrate(&TrafficAnalysis{}, &AIStatistics{}); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// 初始化统计行
	_ = db.Exec("INSERT IGNORE INTO ai_statistics (id) VALUES (1)")

	return nil
}

func (a *MySQLAdapter) Close() error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (a *MySQLAdapter) toRecord(r *TrafficAnalysis) *TrafficRecord {
	return &TrafficRecord{
		ID:             r.ID,
		Host:           r.Host,
		Path:           r.Path,
		Method:         r.Method,
		QueryParamKeys: r.QueryParamKeys,
		BodySchemaHash: r.BodySchemaHash,
		ContentType:    r.ContentType,
		SampleRequest:  r.SampleRequest,
		SampleResponse: r.SampleResponse,
		SigKey:         r.SigKey,
		Status:         r.Status,
	}
}

func (a *MySQLAdapter) toAnalyzedRecord(r *TrafficAnalysis) *AnalyzedRecord {
	record := &AnalyzedRecord{
		ID:                  r.ID,
		Host:                r.Host,
		Path:                r.Path,
		Method:              r.Method,
		BusinessName:        r.BusinessName,
		BusinessDescription: r.BusinessDescription,
		FunctionName:        r.FunctionName,
		Sensitivity:         r.Sensitivity,
		AuthMechanism:       r.AuthMechanism,
		AnalysisContext:     r.AnalysisContext,
		PenetrationPriority: r.PenetrationPriority,
		SampleRequest:       r.SampleRequest,
		SampleResponse:      r.SampleResponse,
		RiskLevel:           r.RiskLevel,
		FinalSummary:        r.FinalSummary,
	}
	if r.AnalyzedAt.Valid {
		record.AnalyzedAt = r.AnalyzedAt.Time.Format(time.RFC3339)
	}
	return record
}

func (a *MySQLAdapter) InsertSignature(rec *TrafficRecord) (int64, bool, error) {
	// 先检查是否已存在
	var existing TrafficAnalysis
	err := a.db.Where("sig_key = ?", rec.SigKey).First(&existing).Error
	if err == nil {
		return existing.ID, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return 0, false, fmt.Errorf("check existing: %w", err)
	}

	// 插入新记录
	r := &TrafficAnalysis{
		Host:           rec.Host,
		Path:           rec.Path,
		Method:         rec.Method,
		QueryParamKeys: rec.QueryParamKeys,
		BodySchemaHash: rec.BodySchemaHash,
		ContentType:    rec.ContentType,
		SampleRequest:  rec.SampleRequest,
		SampleResponse: rec.SampleResponse,
		SigKey:        rec.SigKey,
		Status:         rec.Status,
	}
	if err := a.db.Create(r).Error; err != nil {
		return 0, false, fmt.Errorf("insert signature: %w", err)
	}
	return r.ID, true, nil
}

func (a *MySQLAdapter) UpdateAnalysisResult(sigKey string, result *AnalysisResult) error {
	updates := map[string]interface{}{}
	if result == nil {
		updates["status"] = StatusFailed
		updates["analyzed_at"] = time.Now()
	} else {
		updates["status"] = StatusAnalyzed
		updates["session_id"] = result.SessionID
		updates["business_name"] = result.BusinessName
		updates["business_description"] = result.BusinessDescription
		updates["function_name"] = result.FunctionName
		updates["sensitivity"] = result.Sensitivity
		updates["auth_mechanism"] = result.AuthMechanism
		updates["analysis_context"] = result.AnalysisContext
		updates["penetration_priority"] = result.PenetrationPriority
		updates["risk_level"] = result.RiskLevel
		updates["final_summary"] = result.FinalSummary
		updates["analyzed_at"] = time.Now()
	}
	return a.db.Model(&TrafficAnalysis{}).Where("sig_key = ?", sigKey).Updates(updates).Error
}

func (a *MySQLAdapter) ExistsBySigKey(sigKey string) (bool, error) {
	var count int64
	if err := a.db.Model(&TrafficAnalysis{}).Where("sig_key = ?", sigKey).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *MySQLAdapter) ShouldAnalyze(sigKey string) (bool, bool, error) {
	var status string
	err := a.db.Model(&TrafficAnalysis{}).Where("sig_key = ?", sigKey).Pluck("status", &status).Error
	if err != nil {
		// 记录不存在
		return false, false, nil
	}
	switch status {
	case StatusFailed:
		// 分析失败过，重新分析
		return false, true, nil
	case StatusPending, StatusAnalyzed, StatusSkipped:
		// 已有分析结果，跳过
		return true, false, nil
	default:
		return false, false, nil
	}
}

func (a *MySQLAdapter) ListByStatus(status string, limit int) ([]*TrafficRecord, error) {
	var records []TrafficAnalysis
	if err := a.db.Where("status = ?", status).Order("id ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]*TrafficRecord, len(records))
	for i := range records {
		result[i] = a.toRecord(&records[i])
	}
	return result, nil
}

func (a *MySQLAdapter) ListByHostAnalyzed(hostPattern string, limit int) ([]*AnalyzedRecord, error) {
	var records []TrafficAnalysis
	if err := a.db.Where("host LIKE ? AND status = ?", "%"+hostPattern+"%", StatusAnalyzed).
		Order("penetration_priority DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]*AnalyzedRecord, len(records))
	for i := range records {
		result[i] = a.toAnalyzedRecord(&records[i])
	}
	return result, nil
}

func (a *MySQLAdapter) GetByIDs(ids []int64) ([]*AnalyzedRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var records []TrafficAnalysis
	if err := a.db.Where("id IN ?", ids).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]*AnalyzedRecord, len(records))
	for i := range records {
		result[i] = a.toAnalyzedRecord(&records[i])
	}
	return result, nil
}

func (a *MySQLAdapter) ListAnalyzed(limit int) ([]*AnalyzedRecord, error) {
	var records []TrafficAnalysis
	if err := a.db.Where("status = ?", StatusAnalyzed).
		Order("analyzed_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]*AnalyzedRecord, len(records))
	for i := range records {
		result[i] = a.toAnalyzedRecord(&records[i])
	}
	return result, nil
}

// ============ Dashboard 统计相关 ============

// GetStatistics 获取累计统计数据
func (a *MySQLAdapter) GetStatistics() (*Statistics, error) {
	var stats AIStatistics
	err := a.db.FirstOrCreate(&stats, AIStatistics{ID: 1}).Error
	if err != nil {
		return nil, err
	}
	return &Statistics{
		TotalRequests:       stats.TotalRequests,
		SuccessCount:        stats.SuccessCount,
		FailureCount:        stats.FailureCount,
		TimeoutCount:        stats.TimeoutCount,
		LLMErrorCount:       stats.LLMErrorCount,
		ParseErrorCount:     stats.ParseErrorCount,
		RateLimitedCount:    stats.RateLimitedCount,
		TotalPromptTokens:   stats.TotalPromptTokens,
		TotalCompletionTokens: stats.TotalCompletionTokens,
		TotalTokens:         stats.TotalTokens,
		PromptCachedTokens:  stats.PromptCachedTokens,
		TurnCount:           stats.TurnCount,
	}, nil
}

// IncrementStatistics 原子增加统计字段
func (a *MySQLAdapter) IncrementStatistics(field string, value int64) error {
	return a.db.Model(&AIStatistics{ID: 1}).
		UpdateColumn(field, gorm.Expr(field+" + ?", value)).Error
}

// IncrementTokenStats 原子增加 Token 统计
func (a *MySQLAdapter) IncrementTokenStats(promptTokens, completionTokens, cachedTokens int64) error {
	return a.db.Model(&AIStatistics{ID: 1}).Updates(map[string]interface{}{
		"total_prompt_tokens":     gorm.Expr("total_prompt_tokens + ?", promptTokens),
		"total_completion_tokens": gorm.Expr("total_completion_tokens + ?", completionTokens),
		"total_tokens":            gorm.Expr("total_tokens + ?", promptTokens+completionTokens),
		"prompt_cached_tokens":    gorm.Expr("prompt_cached_tokens + ?", cachedTokens),
		"turn_count":              gorm.Expr("turn_count + 1"),
	}).Error
}

// ============ 分析记录分页查询 ============

func (a *MySQLAdapter) ListAnalyzedWithPaging(limit, offset int, statusFilter string) ([]*AnalyzedRecord, int64, error) {
	var records []TrafficAnalysis
	var total int64

	query := a.db.Model(&TrafficAnalysis{})
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询分页数据
	if err := query.Order("analyzed_at DESC, id DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*AnalyzedRecord, len(records))
	for i := range records {
		result[i] = a.toAnalyzedRecord(&records[i])
	}
	return result, total, nil
}

// ============ 任务管理 ============

func (a *MySQLAdapter) CreateTask(task *AnalysisTask) error {
	t := &AnalysisTaskGORM{
		ID:        task.ID,
		SigKey:    task.SigKey,
		Host:      task.Host,
		Path:      task.Path,
		Method:    task.Method,
		Status:    task.Status,
		Progress:  task.Progress,
		ErrorMsg:  task.ErrorMsg,
		CreatedAt: task.CreatedAt,
	}
	return a.db.Create(t).Error
}

func (a *MySQLAdapter) UpdateTaskStatus(taskID string, status string, progress int) error {
	updates := map[string]interface{}{
		"status":   status,
		"progress": progress,
	}
	if status == TaskStatusAnalyzing {
		now := time.Now()
		updates["started_at"] = &now
	} else if status == TaskStatusDone || status == TaskStatusFailed {
		now := time.Now()
		updates["finished_at"] = &now
	}
	return a.db.Model(&AnalysisTaskGORM{ID: taskID}).Updates(updates).Error
}

func (a *MySQLAdapter) UpdateTaskError(taskID string, errMsg string) error {
	return a.db.Model(&AnalysisTaskGORM{ID: taskID}).Updates(map[string]interface{}{
		"status":    TaskStatusFailed,
		"error_msg": errMsg,
	}).Error
}

func (a *MySQLAdapter) ListActiveTasks() ([]*AnalysisTask, error) {
	var tasks []AnalysisTaskGORM
	err := a.db.Where("status IN ?", []string{TaskStatusPending, TaskStatusAnalyzing}).
		Order("created_at DESC").Find(&tasks).Error
	if err != nil {
		return nil, err
	}

	result := make([]*AnalysisTask, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		result[i] = &AnalysisTask{
			ID:         t.ID,
			SigKey:     t.SigKey,
			Host:       t.Host,
			Path:       t.Path,
			Method:     t.Method,
			Status:     t.Status,
			Progress:   t.Progress,
			ErrorMsg:   t.ErrorMsg,
			CreatedAt:  t.CreatedAt,
		}
		if t.StartedAt != nil {
			result[i].StartedAt = *t.StartedAt
		}
		if t.FinishedAt != nil {
			result[i].FinishedAt = *t.FinishedAt
		}
	}
	return result, nil
}

func (a *MySQLAdapter) DeleteTask(taskID string) error {
	return a.db.Delete(&AnalysisTaskGORM{ID: taskID}).Error
}
