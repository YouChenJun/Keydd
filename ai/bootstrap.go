package ai

import (
	"Keydd/ai/agent"
	"Keydd/ai/config"
	"Keydd/ai/store"
	logger "Keydd/log"
	"context"
	"fmt"
	"sync"
)

// AISystem 封装所有 AI 子系统的组件
type AISystem struct {
	Factory *agent.AgentFactory
	DB      store.DBAdapter
	Enabled bool
	mu      sync.Mutex
}

// GetRecentAnalyses 获取最近的分析记录
func (s *AISystem) GetRecentAnalyses(limit int) ([]interface{}, error) {
	if s == nil || s.DB == nil {
		return nil, nil
	}
	records, err := s.DB.ListAnalyzed(limit)
	if err != nil {
		return nil, err
	}
	// 转换为 []interface{}
	result := make([]interface{}, len(records))
	for i, r := range records {
		result[i] = r
	}
	return result, nil
}

// InitializeAISystem 初始化完整的 AI 分析系统
// 如果 AI 未启用，返回 nil，error == nil
func InitializeAISystem(ctx context.Context, cfg config.AIConfig) (*AISystem, error) {
	if !cfg.Enabled {
		logger.Info.Println("[AI] AI 分析功能未启用，跳过初始化")
		return nil, nil
	}

	logger.Info.Println("[AI] 正在初始化 AI 分析系统...")

	system := &AISystem{}

	// 初始化数据库
	db, err := store.NewDBAdapter(cfg.Store)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库适配器失败: %w", err)
	}
	if err = db.Init(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}
	system.DB = db
	logger.Info.Printf("[AI] 数据库初始化成功 (type: %s)", cfg.Store.Type)

	// 初始化 Agent 工厂（包含 LLM 连接）
	system.Factory, err = agent.NewAgentFactory(cfg)
	if err != nil {
		logger.Error.Printf("[AI] 初始化 Agent 工厂失败: %v (将以无 LLM 模式运行)", err)
		// 即使 LLM 初始化失败，仍然记录流量特征（去重），不执行分析
	} else {
		// 注入数据库适配器用于持久化统计
		system.Factory.DB = db
		logger.Info.Println("[AI] Agent 工厂初始化成功")
	}

	system.Enabled = true
	logger.Info.Printf("[AI] AI 分析系统初始化完成")
	return system, nil
}

// Shutdown 优雅关闭 AI 系统，刷新所有待发数据
func (s *AISystem) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s == nil || !s.Enabled {
		return nil
	}

	// 关闭数据库
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			logger.Error.Printf("[AI] 关闭数据库失败: %v", err)
		}
	}

	// 关闭 Agent 工厂
	if s.Factory != nil {
		_ = s.Factory.Close()
	}

	logger.Info.Println("[AI] AI 系统已安全关闭")
	return nil
}
