// Package types 定义公共类型，供各层引用，避免循环依赖
// 所有跨模块共享的结构体定义都集中放置在这里
package types

// ============ 基础数据结构 ============

// FullAnalysisResult 完整分析结果（对外输出结构）
// 保存一次完整分析的所有输出
type FullAnalysisResult struct {
	SessionID        string                `json:"session_id"`
	TrafficSigID     int64                 `json:"traffic_sig_id"`
	TrafficAnalysis  TrafficAnalyzerOutput `json:"traffic_analysis"`
	FinalSummary     string                `json:"final_summary"`
	OverallRiskLevel string                `json:"overall_risk_level"`
}

// ============ 工具输入输出结构 ============

// TrafficAnalyzerInput 流量分析工具输入
type TrafficAnalyzerInput struct {
	Host           string `json:"host"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	QueryParamKeys string `json:"query_param_keys"`
	SampleRequest  string `json:"sample_request"`
	SampleResponse string `json:"sample_response"`
	ContentType    string `json:"content_type"`
}

// TrafficAnalyzerOutput 流量分析工具输出
type TrafficAnalyzerOutput struct {
	BusinessName        string `json:"business_name"`
	BusinessDesc        string `json:"business_description"`
	FunctionName        string `json:"function_name"`
	Sensitivity         string `json:"sensitivity"`
	AuthMechanism       string `json:"auth_mechanism"`     // 认证机制
	AnalysisContext     string `json:"analysis_context"`   // 给 Agent-2 用的完整上下文
	PenetrationPriority int    `json:"penetration_priority"` // 渗透测试优先级（1-100，和 prompt 一致）
}
