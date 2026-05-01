package proxy

import (
	"Keydd/ai"
	"Keydd/ai/config"
	"Keydd/ai/store"
	"Keydd/cmd"
	logger "Keydd/log"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

// TrafficSignatureData 从 proxy.Flow 提取的规范化流量特征
type TrafficSignatureData struct {
	Host             string
	Path             string
	Method           string
	QueryParamKeys   string // JSON: ["cmd", "id"]（排序后的键集合）
	BodySchemaHash   string // SHA256 hash of normalized JSON schema
	BodySchemaDetail string // 完整 schema 字符串（用于调试）
	SampleRequest    string // 请求样本（用于 AI 分析）
	SampleResponse   string // 响应样本（用于 AI 分析）
	ContentType      string // 响应 Content-Type
}

// skipContentTypePrefixes 需要跳过 AI 分析的 Content-Type 前缀
var skipContentTypePrefixes = []string{
	"image/",
	"video/",
	"audio/",
	"font/",
}

// skipContentTypeExact 需要跳过的精确 Content-Type
var skipContentTypeExact = []string{
	"text/css",
	"application/pdf",
	"application/zip",
	"application/x-zip-compressed",
	"application/octet-stream",
	"application/wasm",
	"application/x-shockwave-flash",
	"application/vnd.ms-fontobject",
	"application/x-font-woff",
	"application/font-woff",
}

// Handler 代理响应处理器
type Handler struct {
	aiSystem       *ai.AISystem
	aiConfig       config.AIConfig
	db             store.DBAdapter
	analyzedMu     sync.RWMutex
	analyzedSig    map[string]time.Time // 内存去重：key → 首次添加时间（用于 TTL 清理）
	metricsMu      sync.Mutex
	metricsLogCount int64 // 每 N 次分析输出一次指标日志

	// goroutine 背压控制：限制 pending 分析 goroutine 数量，防止 OOM
	pendingChan chan struct{} // 有缓冲 channel 作为背压上限

	// 去重清理控制
	lastCleanup   time.Time              // 上次清理时间
	dedupCleanInterval time.Duration     // 清理间隔（默认 5 分钟）
}

// NewHandler 创建新的代理处理器
func NewHandler(aiSystem *ai.AISystem, aiConfig config.AIConfig) *Handler {
	// 背压控制：pending goroutine 上限为 maxConcurrent 的 5 倍（队列缓冲空间）
	maxConcurrent := aiConfig.LLM.RateLimit.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	pendingLimit := maxConcurrent * 5
	if pendingLimit < 10 {
		pendingLimit = 10
	}

	h := &Handler{
		aiSystem:          aiSystem,
		aiConfig:          aiConfig,
		analyzedSig:       make(map[string]time.Time),
		pendingChan:       make(chan struct{}, pendingLimit), // buffered: 背压上限
		lastCleanup:       time.Now(),
		dedupCleanInterval: 5 * time.Minute, // 每 5 分钟清理一次过期记录
	}
	// 从 AISystem 中获取数据库引用
	if aiSystem != nil {
		h.db = aiSystem.DB
	}
	return h
}

// tryAcquirePending 尝试获取 pending slot（非阻塞）
// 返回 true 表示获得 slot，分析结束后需调用 releasePending
// 返回 false 表示背压触发，跳过此次分析
func (h *Handler) tryAcquirePending() bool {
	select {
	case h.pendingChan <- struct{}{}:
		return true
	default:
		return false
	}
}

// releasePending 释放 pending slot
func (h *Handler) releasePending() {
	<-h.pendingChan
}

// getAnalyzedSig 检查是否已分析过，并触发 TTL 清理
func (h *Handler) getAnalyzedSig(key string) (struct{}, bool) {
	h.analyzedMu.RLock()
	_, ok := h.analyzedSig[key]
	h.analyzedMu.RUnlock()

	// 触发 TTL 清理（每 dedupCleanInterval 检查一次，由读锁持有者执行）
	h.tryCleanup()

	return struct{}{}, ok
}

// markAnalyzedSig 标记为已分析
func (h *Handler) markAnalyzedSig(key string) {
	h.analyzedMu.Lock()
	defer h.analyzedMu.Unlock()
	h.analyzedSig[key] = time.Now()
}

// tryCleanup 尝试清理过期的去重记录（TTL: 30 分钟）
// 由任意操作触发，不阻塞；清理本身持有写锁，尽量快速完成
func (h *Handler) tryCleanup() {
	h.analyzedMu.Lock()
	defer h.analyzedMu.Unlock()

	if time.Since(h.lastCleanup) < h.dedupCleanInterval {
		return
	}
	h.lastCleanup = time.Now()

	// 清理 30 分钟前的记录
	cutoff := time.Now().Add(-30 * time.Minute)
	before := len(h.analyzedSig)
	for k, v := range h.analyzedSig {
		if v.Before(cutoff) {
			delete(h.analyzedSig, k)
		}
	}
	if removed := before - len(h.analyzedSig); removed > 0 {
		if logger.Info != nil {
			logger.Info.Printf("[AI] 清理过期去重记录: 移除 %d 条（剩余 %d 条）", removed, len(h.analyzedSig))
		}
	}
}

// Response 处理代理响应
func (h *Handler) Response(f *proxy.Flow) {
	// 规则匹配处理（敏感信息匹配，不受 AI 分析影响）
	isTextContent := false
	contentType := ""
	if f.Response != nil {
		contentType = f.Response.Header.Get("Content-Type")
		isTextContent = strings.HasPrefix(contentType, "text/html") ||
			strings.HasPrefix(contentType, "application/json") ||
			strings.HasPrefix(contentType, "application/javascript")
	}

	// 规则匹配处理（仅对文本内容）
	if isTextContent {
		f.Response.ReplaceToDecodedBody()
		body := f.Response.Body
		var wg sync.WaitGroup
		errChan := make(chan error, 200)

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cmd.MatchRules(string(body), f)
			if err != nil {
				select {
				case errChan <- err:
				default:
					logger.Error.Println("Failed to send error to channel:", err)
				}
			}
		}()

		go func() {
			wg.Wait()
			close(errChan)
		}()
	}

	// AI 分析（异步执行，不影响代理响应）
	if h.aiSystem != nil && h.aiSystem.Enabled {
		go h.triggerAIAnalysis(f)
	}
}

// ExtractSignature 从 proxy.Flow 中提取 TrafficSignatureData
// 返回: (signature, shouldAnalyze)
// shouldAnalyze=false 时表示该流量不需要 AI 分析（如图片、视频等）
func ExtractSignature(f *proxy.Flow) (*TrafficSignatureData, bool) {
	if f == nil || f.Request == nil || f.Response == nil {
		return nil, false
	}

	// 检查响应 Content-Type 是否需要跳过
	responseContentType := f.Response.Header.Get("Content-Type")
	if shouldSkipContentType(responseContentType) {
		return nil, false
	}

	// 对于没有 Content-Type 的响应（如 304），通过 URL 后缀和 Accept header 判断
	if responseContentType == "" {
		if shouldSkipByRequestHints(f) {
			return nil, false
		}
	}

	// 提取请求信息
	reqURL := f.Request.URL
	if reqURL == nil {
		return nil, false
	}

	host := reqURL.Host
	if host == "" {
		return nil, false
	}

	path := normalizePath(reqURL.Path)
	method := strings.ToUpper(f.Request.Method)

	// 提取查询参数键集合
	queryParamKeys := extractQueryParamKeys(reqURL.RawQuery)

	// 提取请求 body schema（仅限 POST/PUT/PATCH）
	var bodySchemaHash, bodySchemaDetail string
	if method == "POST" || method == "PUT" || method == "PATCH" {
		// 请求也需要检查 gzip
		reqBody := decompressIfGzip(f.Request.Body, f.Request.Header.Get("Content-Encoding"))
		bodySchemaDetail, bodySchemaHash = extractBodySchemaFromJSON(reqBody)
	}

	// 构建请求样本（用于 AI 分析）
	headers := extractImportantHeaders(f.Request)
	reqBody := decompressIfGzip(f.Request.Body, f.Request.Header.Get("Content-Encoding"))
	sampleRequest := buildSampleRequest(method, reqURL.String(), headers, reqBody)

	// 构建响应样本（解压 gzip 如果需要）
	statusCode := 0
	if f.Response != nil {
		statusCode = f.Response.StatusCode
	}
	respBody := decompressIfGzip(f.Response.Body, f.Response.Header.Get("Content-Encoding"))
	sampleResponse := buildSampleResponse(statusCode, f.Response.Header, respBody)

	sig := &TrafficSignatureData{
		Host:             host,
		Path:             path,
		Method:           method,
		QueryParamKeys:   queryParamKeys,
		BodySchemaHash:   bodySchemaHash,
		BodySchemaDetail: bodySchemaDetail,
		SampleRequest:    sampleRequest,
		SampleResponse:   sampleResponse,
		ContentType:      responseContentType,
	}

	return sig, true
}

// shouldSkipContentType 判断是否需要跳过该 Content-Type（不进行 AI 分析）
func shouldSkipContentType(contentType string) bool {
	// 去掉参数部分（如 "text/html; charset=utf-8" → "text/html"）
	contentTypeBase := contentType
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentTypeBase = strings.TrimSpace(contentType[:idx])
	}
	contentTypeBase = strings.ToLower(contentTypeBase)

	for _, prefix := range skipContentTypePrefixes {
		if strings.HasPrefix(contentTypeBase, prefix) {
			return true
		}
	}

	for _, exact := range skipContentTypeExact {
		if contentTypeBase == exact {
			return true
		}
	}

	return false
}

// skipPathExtensions 需要跳过的 URL 文件后缀（静态资源）
var skipPathExtensions = []string{
	".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp", ".avif", ".bmp",
	".mp4", ".webm", ".avi", ".mov", ".mp3", ".wav", ".ogg", ".flac",
	".woff", ".woff2", ".ttf", ".eot", ".otf",
	".pdf", ".zip", ".gz", ".tar", ".rar", ".7z",
	".wasm", ".map", ".swf",
	".js", ".css", ".mjs", ".cjs", ".less", ".scss", ".sass",
}

// shouldSkipByRequestHints 当响应没有 Content-Type 时（如 304），通过请求信息判断是否为静态资源
func shouldSkipByRequestHints(f *proxy.Flow) bool {
	// 1. 通过 URL 后缀判断
	if f.Request.URL != nil {
		path := strings.ToLower(f.Request.URL.Path)
		// 去掉 query 部分
		if idx := strings.IndexByte(path, '?'); idx >= 0 {
			path = path[:idx]
		}
		for _, ext := range skipPathExtensions {
			if strings.HasSuffix(path, ext) {
				return true
			}
		}
	}

	// 2. 通过 Accept header 判断（浏览器请求图片时 Accept 以 image/ 开头）
	accept := strings.ToLower(f.Request.Header.Get("Accept"))
	if accept != "" {
		// Accept: image/avif,image/webp,... 表示这是图片请求
		if strings.HasPrefix(accept, "image/") ||
			strings.HasPrefix(accept, "video/") ||
			strings.HasPrefix(accept, "audio/") {
			return true
		}
	}

	return false
}

// extractImportantHeaders 从请求中提取重要的 header
func extractImportantHeaders(req *proxy.Request) map[string]string {
	if req == nil {
		return nil
	}
	headers := make(map[string]string)
	importantHeaders := []string{
		"Content-Type", "Authorization", "User-Agent",
		"X-Forwarded-For", "Accept", "Origin", "Referer",
		"Cookie", "X-Auth-Token", "X-API-Key",
	}
	for _, h := range importantHeaders {
		if v := req.Header.Get(h); v != "" {
			headers[h] = v
		}
	}
	return headers
}

// extractBodySchemaFromJSON 从 JSON body 中提取类型 schema
// 将所有值替换为类型占位符，例如:
//
//	{"username": "test", "password": "123"} → {"username": "<string>", "password": "<string>"}
//	{"count": 10, "active": true}           → {"count": "<number>", "active": "<bool>"}
func extractBodySchemaFromJSON(body []byte) (schemaDetail string, schemaHash string) {
	if len(body) == 0 {
		return "", ""
	}

	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		// 不是有效 JSON，返回空
		return "", ""
	}

	normalized := normalizeValue(parsed)
	schemaBytes, err := json.Marshal(normalized)
	if err != nil {
		return "", ""
	}

	schemaDetail = string(schemaBytes)
	hash := sha256.Sum256(schemaBytes)
	schemaHash = fmt.Sprintf("%x", hash)
	return schemaDetail, schemaHash
}

// normalizeValue 递归将 JSON 值替换为类型占位符
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, vv := range val {
			result[k] = normalizeValue(vv)
		}
		return result
	case []interface{}:
		// 数组只保留第一个元素的结构（代表数组元素类型）
		if len(val) == 0 {
			return []interface{}{}
		}
		return []interface{}{normalizeValue(val[0])}
	case float64:
		return "<number>"
	case string:
		return "<string>"
	case bool:
		return "<bool>"
	case nil:
		return "<null>"
	default:
		return "<unknown>"
	}
}

// extractQueryParamKeys 从 query string 中提取排序后的参数键集合
// 例如: "id=111&cmd=sleep+1" → ["cmd", "id"]
func extractQueryParamKeys(queryString string) string {
	if queryString == "" {
		return "[]"
	}

	var keys []string
	seen := make(map[string]bool)

	// 手动解析，避免引入 url 包导致循环依赖
	for _, pair := range splitQueryString(queryString) {
		if pair == "" {
			continue
		}
		key := pair
		if idx := indexOf(pair, '='); idx >= 0 {
			key = pair[:idx]
		}
		if key != "" && !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}

	sort.Strings(keys)

	keyBytes, _ := json.Marshal(keys)
	return string(keyBytes)
}

// splitQueryString 按 & 分割 query string
func splitQueryString(query string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '&' {
			parts = append(parts, query[start:i])
			start = i + 1
		}
	}
	parts = append(parts, query[start:])
	return parts
}

// indexOf 查找字节在字符串中的位置
func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// normalizePath 规范化 URL path
// 保留 path 结构，但将数字 ID 段替换为占位符（可选，当前版本保留原始 path）
func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	// 确保以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// buildSampleRequest 构建请求样本（用于 AI 分析）
// 截取请求内容，避免过长
func buildSampleRequest(method, rawURL string, headers map[string]string, body []byte) string {
	var sb strings.Builder

	sb.WriteString(method)
	sb.WriteString(" ")
	sb.WriteString(rawURL)
	sb.WriteString("\n")

	// 写入关键 headers
	importantHeaders := []string{
		"Content-Type", "Authorization", "User-Agent",
		"X-Forwarded-For", "Accept", "Origin", "Referer",
	}
	for _, h := range importantHeaders {
		if v, ok := headers[h]; ok && v != "" {
			sb.WriteString(h)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteString("\n")
		}
	}

	if len(body) > 0 {
		sb.WriteString("\n")
		// body 最多写入 2000 字节，避免样本过长
		if len(body) > 2000 {
			sb.Write(body[:2000])
			sb.WriteString("\n...[truncated]")
		} else {
			sb.Write(body)
		}
	}

	return sb.String()
}

// decompressIfGzip 如果 body 是 gzip 压缩则解压，返回解压后的内容
func decompressIfGzip(body []byte, contentEncoding string) []byte {
	if body == nil || len(body) == 0 {
		return body
	}
	contentEncoding = strings.ToLower(contentEncoding)
	if !strings.Contains(contentEncoding, "gzip") {
		return body
	}

	// 尝试解压
	buf := bytes.NewBuffer(body)
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return body // 解压失败，返回原始数据
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		return body // 解压失败，返回原始数据
	}
	return decompressed
}

// buildSampleResponse 构建响应样本（用于 AI 分析）
func buildSampleResponse(statusCode int, respHeaders http.Header, body []byte) string {
	var sb strings.Builder

	sb.WriteString("HTTP/1.1 ")
	sb.WriteString(intToStr(statusCode))
	sb.WriteString("\n")

	// 输出所有响应头
	for key, values := range respHeaders {
		for _, value := range values {
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(value)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	if len(body) > 0 {
		sb.Write(body)
	}

	return sb.String()
}

// intToStr 将整数转为字符串（避免引入 strconv 增加依赖）
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// 反转
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// triggerAIAnalysis 触发 AI 流量分析-对单一接口先进行分析
func (h *Handler) triggerAIAnalysis(f *proxy.Flow) {
	if h.aiSystem == nil || h.aiSystem.Factory == nil {
		return
	}

	// 提取流量特征
	sig, shouldAnalyze := ExtractSignature(f)
	if !shouldAnalyze || sig == nil {
		return
	}

	// 基于特征生成唯一 key 用于去重
	sigKey := fmt.Sprintf("%s|%s|%s|%s|%s", sig.Host, sig.Path, sig.Method, sig.QueryParamKeys, sig.BodySchemaHash)

	// 去重：跳过已分析过的相同特征（failed 状态会重新分析）
	var shouldReinsert bool
	if h.aiConfig.Analysis.DeduplicationEnabled {
		// 内存去重：跳过已分析过的相同特征
		if _, exists := h.getAnalyzedSig(sigKey); exists {
			return
		}

		// 数据库去重：检查状态决定是否跳过或重分析
		if h.db != nil {
			shouldSkip, reinsert, err := h.db.ShouldAnalyze(sigKey)
			if err != nil {
				logger.Error.Printf("[AI] 数据库查重失败: %v", err)
			} else if shouldSkip {
				// 已有成功/跳过记录，跳过分析
				h.markAnalyzedSig(sigKey)
				return
			} else if reinsert {
				// 分析失败过，标记重分析
				shouldReinsert = true
			}
		}
	}

	// 背压控制：在 goroutine 启动前获取 slot，超限则跳过此次分析
	if !h.tryAcquirePending() {
		logger.Warning.Printf("[AI] 背压触发，跳过分析: %s %s%s (pending goroutine 已满)", sig.Method, sig.Host, sig.Path)
		return
	}

	// 启动 goroutine 前做内容类型过滤和 DB 存储
	skipAnalysis := false
	if !shouldReinsert {
		// 如果开启了 OnlyAnalyzeXHR，通过 URL 后缀和 Content-Type 过滤非 XHR 请求
		if h.aiConfig.Analysis.OnlyAnalyzeXHR && (!isXHRContentType(sig.ContentType) || isStaticResourcePath(sig.Path)) {
			logger.Info.Printf("[AI] 跳过非 XHR 请求: %s%s (Content-Type: %s)", sig.Host, sig.Path, sig.ContentType)
			// 分层存储：只存基础信息，不存 body
			if h.db != nil {
				rec := &store.TrafficRecord{
					Host:           sig.Host,
					Path:           sig.Path,
					Method:         sig.Method,
					QueryParamKeys: sig.QueryParamKeys,
					BodySchemaHash: sig.BodySchemaHash,
					ContentType:    sig.ContentType,
					SampleRequest:  "", // 跳过的请求不存 body
					SampleResponse: "", // 跳过的请求不存 body
					SigKey:         sigKey,
					Status:         store.StatusSkipped,
				}
				if _, _, err := h.db.InsertSignature(rec); err != nil {
					logger.Error.Printf("[AI] 存储跳过特征失败: %v", err)
				}
			}
			skipAnalysis = true
		} else {
			logger.Info.Printf("[AI] 发现新流量特征: %s %s%s", sig.Method, sig.Host, sig.Path)
			// 存储完整流量特征，状态为 pending
			if h.db != nil {
				rec := &store.TrafficRecord{
					Host:           sig.Host,
					Path:           sig.Path,
					Method:         sig.Method,
					QueryParamKeys: sig.QueryParamKeys,
					BodySchemaHash: sig.BodySchemaHash,
					ContentType:    sig.ContentType,
					SampleRequest:  sig.SampleRequest,
					SampleResponse: sig.SampleResponse,
					SigKey:         sigKey,
					Status:         store.StatusPending,
				}
				if _, _, err := h.db.InsertSignature(rec); err != nil {
					logger.Error.Printf("[AI] 存储流量特征失败: %v", err)
				}
			}
		}
	}

	h.markAnalyzedSig(sigKey)

	go func(sig *TrafficSignatureData, sigKey string, shouldReinsert, skipAnalysis bool) {
		defer h.releasePending() // 退出时必须释放 slot

		// 如果是重分析，先把状态改回 pending
		if shouldReinsert && h.db != nil {
			if err := h.db.UpdateAnalysisResult(sigKey, nil); err != nil {
				logger.Warning.Printf("[AI] 重置失败状态失败，将跳过: %v", err)
				return
			}
			logger.Info.Printf("[AI] 重新分析失败记录: %s %s%s", sig.Method, sig.Host, sig.Path)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		sessionID := fmt.Sprintf("direct-%x", sha256.Sum256([]byte(sigKey)))[:20]
		result, err := h.aiSystem.Factory.RunFullAnalysis(ctx, sessionID, 0, sig.Host, sig.Method, sig.Path, sig.SampleRequest, sig.SampleResponse)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				logger.Error.Printf("[AI] 分析超时(60s): %s %s%s", sig.Method, sig.Host, sig.Path)
			} else {
				logger.Error.Printf("[AI] 分析失败: %v", err)
			}
			// 更新数据库状态为 failed
			if h.db != nil {
				if dbErr := h.db.UpdateAnalysisResult(sigKey, nil); dbErr != nil {
					logger.Error.Printf("[AI] 更新失败状态失败: %v", dbErr)
				}
			}
		} else {
			logger.Info.Printf("[AI] 分析完成: %s %s%s", sig.Method, sig.Host, sig.Path)
			// 更新数据库分析结果
			if h.db != nil && result != nil {
				analysisResult := &store.AnalysisResult{
					SessionID:           result.SessionID,
					BusinessName:        result.TrafficAnalysis.BusinessName,
					BusinessDescription: result.TrafficAnalysis.BusinessDesc,
					FunctionName:        result.TrafficAnalysis.FunctionName,
					Sensitivity:         result.TrafficAnalysis.Sensitivity,
					AuthMechanism:       result.TrafficAnalysis.AuthMechanism,
					AnalysisContext:     result.TrafficAnalysis.AnalysisContext,
					PenetrationPriority: result.TrafficAnalysis.PenetrationPriority,
					RiskLevel:           result.OverallRiskLevel,
					FinalSummary:        result.FinalSummary,
				}
				if dbErr := h.db.UpdateAnalysisResult(sigKey, analysisResult); dbErr != nil {
					logger.Error.Printf("[AI] 存储分析结果失败: %v", dbErr)
				}
			}
		}

		// 每 10 次分析输出一次简化日志
		h.metricsMu.Lock()
		h.metricsLogCount++
		count := h.metricsLogCount
		h.metricsMu.Unlock()
		if count%10 == 0 {
			logger.Info.Printf("[AI] 分析进度: 已完成 %d 次分析", count)
		}
	}(sig, sigKey, shouldReinsert, skipAnalysis)
}

// Addon 实现代理插件接口
type Addon struct {
	proxy.BaseAddon
	handler *Handler
}

// NewAddon 创建代理插件
func NewAddon(handler *Handler) *Addon {
	return &Addon{handler: handler}
}

// ShouldIntercept 判断是否需要拦截这个连接
// 直接跳过 WebSocket，避免 abnormal closure 错误
func (a *Addon) ShouldIntercept(req *http.Request, _ interface{}) bool {
	// 跳过 WebSocket 连接，它们不需要分析
	if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		return false
	}
	return true
}

// Response 代理插件方法
func (a *Addon) Response(f *proxy.Flow) {
	a.handler.Response(f)
}

// isXHRContentType 判断 Content-Type 是否属于 XHR/fetch API 请求
// XHR 请求通常是 JSON、XML 等数据格式，不是 HTML 页面或静态资源
func isXHRContentType(contentType string) bool {
	if contentType == "" {
		return true // 未知类型，保留分析
	}
	contentType = strings.ToLower(contentType)

	// 静态资源（JS 文件不是 API 请求）
	if strings.Contains(contentType, "application/javascript") ||
		strings.Contains(contentType, "text/javascript") ||
		strings.Contains(contentType, "text/html") {
		return false
	}

	// API/XHR 数据类型
	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/xml") ||
		strings.Contains(contentType, "application/problem+json") ||
		strings.Contains(contentType, "application/ld+json") ||
		strings.Contains(contentType, "text/plain") {
		return true
	}

	// 二进制静态资源已经在之前被过滤掉了
	// 对于其他类型（比如 image/*, font/* 已经被过滤了）
	// 未知但非 HTML 的类型，默认保留分析
	return true
}

// staticResourceExtensions 当开启 OnlyAnalyzeXHR 时，通过 URL 后缀过滤的静态资源
var staticResourceExtensions = []string{
	".js", ".css", ".mjs", ".cjs",
	".less", ".scss", ".sass",
	".map", ".wasm",
}

// isStaticResourcePath 判断 URL path 是否指向静态资源（JS/CSS 等）
// 用于 OnlyAnalyzeXHR 模式下，补充 Content-Type 检测的不足
func isStaticResourcePath(path string) bool {
	path = strings.ToLower(path)
	// 去掉 query 参数部分
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	for _, ext := range staticResourceExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
