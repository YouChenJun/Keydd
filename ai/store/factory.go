package store

import (
	"Keydd/ai/config"
	"fmt"
)

// NewDBAdapter 根据配置创建数据库适配器
func NewDBAdapter(cfg config.StoreConfig) (DBAdapter, error) {
	switch cfg.Type {
	case "sqlite":
		path := cfg.SQLitePath
		if path == "" {
			path = "data_ai.db"
		}
		return NewSQLiteAdapter(path), nil
	case "postgres":
		if cfg.PostgresDSN == "" {
			return nil, fmt.Errorf("PostgreSQL DSN 未配置")
		}
		return NewPostgresAdapter(cfg.PostgresDSN), nil
	case "mysql":
		if cfg.MySQL == nil {
			return nil, fmt.Errorf("MySQL 配置未填写")
		}
		if cfg.MySQL.Host == "" || cfg.MySQL.Database == "" {
			return nil, fmt.Errorf("MySQL host 和 database 不能为空")
		}
		return NewMySQLAdapter(cfg.MySQL), nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s（支持 sqlite / postgres / mysql）", cfg.Type)
	}
}
