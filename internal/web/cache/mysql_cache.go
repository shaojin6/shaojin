package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// MySQLCache 使用 MySQL 存储工具列表缓存，并可选回退到本地持久化存储
type MySQLCache struct {
	db       *sql.DB
	fallback Cache
}

// NewMySQLCache 创建 MySQL 缓存
func NewMySQLCache(dsn string, fallback Cache) (*MySQLCache, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL: %w", err)
	}

	// 设置连接池参数，防止长连接占用
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(20)

	// 增加 Ping 超时时间到 10 秒
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	cache := &MySQLCache{
		db:       db,
		fallback: fallback,
	}

	if err := cache.ensureTable(); err != nil {
		return nil, err
	}

	return cache, nil
}

// ensureTable 确保缓存表存在
func (m *MySQLCache) ensureTable() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS mcp_tools_cache (
    identifier VARCHAR(255) NOT NULL PRIMARY KEY,
    tools LONGTEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARSET=utf8mb4`

	// 增加表创建超时时间到 30 秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := m.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("failed to ensure cache table: %w", err)
	}

	return nil
}

// GetTools 从 MySQL 获取工具列表，不存在时回退到 fallback
func (m *MySQLCache) GetTools(ctx context.Context, identifier string) ([]mcp.Tool, error) {
	if m.db == nil {
		return m.getFromFallback(ctx, identifier)
	}

	var data []byte
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := m.db.QueryRowContext(queryCtx, "SELECT tools FROM mcp_tools_cache WHERE identifier = ? LIMIT 1", identifier).Scan(&data)
	if err == sql.ErrNoRows {
		return m.getFromFallback(ctx, identifier)
	}
	if err != nil {
		log.Printf("[Cache] MySQL query error: %v", err)
		return m.getFromFallback(ctx, identifier)
	}

	var tools []mcp.Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		log.Printf("[Cache] MySQL data unmarshal error: %v", err)
		return m.getFromFallback(ctx, identifier)
	}

	return tools, nil
}

func (m *MySQLCache) getFromFallback(ctx context.Context, identifier string) ([]mcp.Tool, error) {
	if m.fallback != nil {
		return m.fallback.GetTools(ctx, identifier)
	}
	return nil, nil
}

// SetTools 写入 MySQL，并同步 fallback
func (m *MySQLCache) SetTools(ctx context.Context, identifier string, tools []mcp.Tool, ttl time.Duration) error {
	if m.db != nil {
		payload, err := json.Marshal(tools)
		if err != nil {
			log.Printf("[Cache] Failed to marshal tools for MySQL: %v", err)
		} else {
			execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, err = m.db.ExecContext(execCtx, `
INSERT INTO mcp_tools_cache (identifier, tools, updated_at)
VALUES (?, ?, NOW())
ON DUPLICATE KEY UPDATE tools = VALUES(tools), updated_at = NOW()`, identifier, payload)
			if err != nil {
				log.Printf("[Cache] Failed to write tools to MySQL: %v", err)
			}
		}
	}

	if m.fallback != nil {
		return m.fallback.SetTools(ctx, identifier, tools, ttl)
	}
	return nil
}

// DeleteTools 删除 MySQL 与 fallback 中的缓存
func (m *MySQLCache) DeleteTools(ctx context.Context, identifier string) error {
	if m.db != nil {
		execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := m.db.ExecContext(execCtx, "DELETE FROM mcp_tools_cache WHERE identifier = ?", identifier); err != nil {
			log.Printf("[Cache] Failed to delete tools from MySQL: %v", err)
		}
	}

	if m.fallback != nil {
		return m.fallback.DeleteTools(ctx, identifier)
	}

	return nil
}

// IsAvailable 检查 MySQL 连接是否可用
func (m *MySQLCache) IsAvailable() bool {
	if m.db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return m.db.PingContext(ctx) == nil
}
