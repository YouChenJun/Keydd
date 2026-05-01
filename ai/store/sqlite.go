package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteAdapter SQLite 数据库适配器
type SQLiteAdapter struct {
	db   *sql.DB
	path string
}

// NewSQLiteAdapter 创建 SQLite 适配器
func NewSQLiteAdapter(path string) *SQLiteAdapter {
	return &SQLiteAdapter{path: path}
}

func (a *SQLiteAdapter) Init() error {
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", a.path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写入连接
	a.db = db

	// 执行建表
	_, err = a.db.Exec(sqliteSchema)
	if err != nil {
		return fmt.Errorf("init sqlite schema: %w", err)
	}
	return nil
}

func (a *SQLiteAdapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

func (a *SQLiteAdapter) InsertSignature(rec *TrafficRecord) (int64, bool, error) {
	// 先检查是否已存在
	var existingID int64
	err := a.db.QueryRow("SELECT id FROM traffic_analysis WHERE sig_key = ?", rec.SigKey).Scan(&existingID)
	if err == nil {
		return existingID, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("check existing: %w", err)
	}

	// 插入新记录
	result, err := a.db.Exec(`INSERT INTO traffic_analysis 
		(host, path, method, query_param_keys, body_schema_hash, content_type, 
		 sample_request, sample_response, sig_key, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Host, rec.Path, rec.Method, rec.QueryParamKeys, rec.BodySchemaHash,
		rec.ContentType, rec.SampleRequest, rec.SampleResponse, rec.SigKey, rec.Status)
	if err != nil {
		return 0, false, fmt.Errorf("insert signature: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("get last insert id: %w", err)
	}
	return id, true, nil
}

func (a *SQLiteAdapter) UpdateAnalysisResult(sigKey string, result *AnalysisResult) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	status := StatusAnalyzed
	if result == nil {
		// 分析失败
		_, err := a.db.Exec(`UPDATE traffic_analysis SET status = ?, updated_at = ? WHERE sig_key = ?`,
			StatusFailed, now, sigKey)
		return err
	}

	_, err := a.db.Exec(`UPDATE traffic_analysis SET 
		status = ?, session_id = ?, business_name = ?, business_description = ?,
		function_name = ?, sensitivity = ?, auth_mechanism = ?, analysis_context = ?,
		penetration_priority = ?, risk_level = ?, final_summary = ?,
		updated_at = ?, analyzed_at = ?
		WHERE sig_key = ?`,
		status, result.SessionID, result.BusinessName, result.BusinessDescription,
		result.FunctionName, result.Sensitivity, result.AuthMechanism, result.AnalysisContext,
		result.PenetrationPriority, result.RiskLevel, result.FinalSummary,
		now, now, sigKey)
	return err
}

func (a *SQLiteAdapter) ExistsBySigKey(sigKey string) (bool, error) {
	var count int
	err := a.db.QueryRow("SELECT COUNT(*) FROM traffic_analysis WHERE sig_key = ?", sigKey).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *SQLiteAdapter) ShouldAnalyze(sigKey string) (bool, bool, error) {
	var status string
	err := a.db.QueryRow("SELECT status FROM traffic_analysis WHERE sig_key = ?", sigKey).Scan(&status)
	if err != nil {
		// 记录不存在
		return false, false, nil
	}
	switch status {
	case StatusFailed:
		return false, true, nil
	case StatusPending, StatusAnalyzed, StatusSkipped:
		return true, false, nil
	default:
		return false, false, nil
	}
}

func (a *SQLiteAdapter) ListByStatus(status string, limit int) ([]*TrafficRecord, error) {
	rows, err := a.db.Query(`SELECT id, host, path, method, query_param_keys, body_schema_hash,
		content_type, sample_request, sample_response, sig_key, status
		FROM traffic_analysis WHERE status = ? ORDER BY id ASC LIMIT ?`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*TrafficRecord
	for rows.Next() {
		r := &TrafficRecord{}
		err := rows.Scan(&r.ID, &r.Host, &r.Path, &r.Method, &r.QueryParamKeys,
			&r.BodySchemaHash, &r.ContentType, &r.SampleRequest, &r.SampleResponse,
			&r.SigKey, &r.Status)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (a *SQLiteAdapter) ListByHostAnalyzed(hostPattern string, limit int) ([]*AnalyzedRecord, error) {
	likePattern := "%" + hostPattern + "%"
	rows, err := a.db.Query(`SELECT id, host, path, method, business_name, business_description,
		function_name, sensitivity, auth_mechanism, analysis_context,
		penetration_priority, sample_request, sample_response
		FROM traffic_analysis
		WHERE host LIKE ? AND status = 'analyzed'
		ORDER BY penetration_priority DESC
		LIMIT ?`, likePattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*AnalyzedRecord
	for rows.Next() {
		r := &AnalyzedRecord{}
		err := rows.Scan(&r.ID, &r.Host, &r.Path, &r.Method, &r.BusinessName,
			&r.BusinessDescription, &r.FunctionName, &r.Sensitivity, &r.AuthMechanism,
			&r.AnalysisContext, &r.PenetrationPriority, &r.SampleRequest, &r.SampleResponse)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (a *SQLiteAdapter) GetByIDs(ids []int64) ([]*AnalyzedRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := "(" + sqliteJoinStrings(placeholders, ",") + ")"

	query := `SELECT id, host, path, method, business_name, business_description,
		function_name, sensitivity, auth_mechanism, analysis_context,
		penetration_priority, sample_request, sample_response
		FROM traffic_analysis
		WHERE id IN ` + inClause

	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*AnalyzedRecord
	for rows.Next() {
		r := &AnalyzedRecord{}
		err := rows.Scan(&r.ID, &r.Host, &r.Path, &r.Method, &r.BusinessName,
			&r.BusinessDescription, &r.FunctionName, &r.Sensitivity, &r.AuthMechanism,
			&r.AnalysisContext, &r.PenetrationPriority, &r.SampleRequest, &r.SampleResponse)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (a *SQLiteAdapter) ListAnalyzed(limit int) ([]*AnalyzedRecord, error) {
	rows, err := a.db.Query(`SELECT id, host, path, method, business_name, business_description,
		function_name, sensitivity, auth_mechanism, analysis_context,
		penetration_priority, sample_request, sample_response, risk_level, final_summary, analyzed_at
		FROM traffic_analysis
		WHERE status = 'analyzed'
		ORDER BY analyzed_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*AnalyzedRecord
	for rows.Next() {
		r := &AnalyzedRecord{}
		var analyzedAt sql.NullTime
		err := rows.Scan(&r.ID, &r.Host, &r.Path, &r.Method, &r.BusinessName,
			&r.BusinessDescription, &r.FunctionName, &r.Sensitivity, &r.AuthMechanism,
			&r.AnalysisContext, &r.PenetrationPriority, &r.SampleRequest, &r.SampleResponse,
			&r.RiskLevel, &r.FinalSummary, &analyzedAt)
		if err != nil {
			return nil, err
		}
		if analyzedAt.Valid {
			r.AnalyzedAt = analyzedAt.Time.Format(time.RFC3339)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func sqliteJoinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// ============ Dashboard 统计相关 ============

// GetStatistics 获取累计统计数据
func (a *SQLiteAdapter) GetStatistics() (*Statistics, error) {
	var stats Statistics
	err := a.db.QueryRow(`SELECT total_requests, success_count, failure_count,
		timeout_count, llm_error_count, parse_error_count, rate_limited_count,
		total_prompt_tokens, total_completion_tokens, total_tokens, prompt_cached_tokens,
		turn_count FROM ai_statistics WHERE id = 1`).Scan(
		&stats.TotalRequests, &stats.SuccessCount, &stats.FailureCount,
		&stats.TimeoutCount, &stats.LLMErrorCount, &stats.ParseErrorCount, &stats.RateLimitedCount,
		&stats.TotalPromptTokens, &stats.TotalCompletionTokens, &stats.TotalTokens,
		&stats.PromptCachedTokens, &stats.TurnCount,
	)
	if err == sql.ErrNoRows {
		// 没有记录则初始化一行
		_, err = a.db.Exec(`INSERT OR IGNORE INTO ai_statistics (id) VALUES (1)`)
		return &Statistics{}, err
	}
	return &stats, err
}

// IncrementStatistics 原子增加统计字段
func (a *SQLiteAdapter) IncrementStatistics(field string, value int64) error {
	_, err := a.db.Exec(`UPDATE ai_statistics SET ` + field + ` = ` + field + ` + ? WHERE id = 1`, value)
	return err
}

// IncrementTokenStats 原子增加 Token 统计
func (a *SQLiteAdapter) IncrementTokenStats(promptTokens, completionTokens, cachedTokens int64) error {
	_, err := a.db.Exec(`UPDATE ai_statistics SET
		total_prompt_tokens = total_prompt_tokens + ?,
		total_completion_tokens = total_completion_tokens + ?,
		total_tokens = total_tokens + ?,
		prompt_cached_tokens = prompt_cached_tokens + ?,
		turn_count = turn_count + 1
		WHERE id = 1`, promptTokens, completionTokens, promptTokens+completionTokens, cachedTokens)
	return err
}

// ============ 分析记录分页查询 ============

func (a *SQLiteAdapter) ListAnalyzedWithPaging(limit, offset int, statusFilter string) ([]*AnalyzedRecord, int64, error) {
	var total int64
	query := `SELECT COUNT(*) FROM traffic_analysis`
	args := []interface{}{}
	if statusFilter != "" {
		query += ` WHERE status = ?`
		args = append(args, statusFilter)
	}
	err := a.db.QueryRow(query, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 查询分页数据
	query = `SELECT id, host, path, method, business_name, business_description,
		function_name, sensitivity, auth_mechanism, analysis_context,
		penetration_priority, sample_request, sample_response, risk_level, final_summary, analyzed_at
		FROM traffic_analysis ORDER BY analyzed_at DESC LIMIT ? OFFSET ?`
	rows, err := a.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []*AnalyzedRecord
	for rows.Next() {
		r := &AnalyzedRecord{}
		var analyzedAt sql.NullTime
		err := rows.Scan(&r.ID, &r.Host, &r.Path, &r.Method, &r.BusinessName,
			&r.BusinessDescription, &r.FunctionName, &r.Sensitivity, &r.AuthMechanism,
			&r.AnalysisContext, &r.PenetrationPriority, &r.SampleRequest, &r.SampleResponse,
			&r.RiskLevel, &r.FinalSummary, &analyzedAt)
		if err != nil {
			return nil, 0, err
		}
		if analyzedAt.Valid {
			r.AnalyzedAt = analyzedAt.Time.Format(time.RFC3339)
		}
		records = append(records, r)
	}
	return records, total, rows.Err()
}

// ============ 任务管理（简化实现，暂不使用） ============

func (a *SQLiteAdapter) CreateTask(task *AnalysisTask) error    { return nil }
func (a *SQLiteAdapter) UpdateTaskStatus(taskID string, status string, progress int) error { return nil }
func (a *SQLiteAdapter) UpdateTaskError(taskID string, errMsg string) error { return nil }
func (a *SQLiteAdapter) ListActiveTasks() ([]*AnalysisTask, error)          { return nil, nil }
func (a *SQLiteAdapter) DeleteTask(taskID string) error                      { return nil }
