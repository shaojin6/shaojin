package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/your-org/k8s-mcp-agent/internal/web/types"
)

// MySQLStore MySQL 存储实现（用于 Agent 配置，包括 SystemPrompt）
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore 创建 MySQL 存储
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL: %w", err)
	}

	// 设置连接池参数
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(20)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	store := &MySQLStore{
		db: db,
	}

	if err := store.ensureTables(); err != nil {
		return nil, err
	}

	return store, nil
}

// ensureTables 确保表存在
func (m *MySQLStore) ensureTables() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建 agents 表
	ddl := `
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mcp_server_id VARCHAR(255) NOT NULL,
    llm_id VARCHAR(255),
    system_prompt LONGTEXT,
    strategy VARCHAR(50) DEFAULT NULL COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)',
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_mcp_server_id (mcp_server_id),
    INDEX idx_enabled (enabled),
    INDEX idx_is_default (is_default),
    INDEX idx_strategy (strategy)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

	if _, err := m.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("failed to create agents table: %w", err)
	}

	// 检查并添加 strategy 字段（如果表已存在但字段不存在）
	// 兼容现有数据库
	checkColumnSQL := `
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		AND TABLE_NAME = 'agents' 
		AND COLUMN_NAME = 'strategy'`
	
	var count int
	if err := m.db.QueryRowContext(ctx, checkColumnSQL).Scan(&count); err == nil && count == 0 {
		// 字段不存在，添加字段
		alterSQL := `
			ALTER TABLE agents 
			ADD COLUMN strategy VARCHAR(50) DEFAULT NULL 
			COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)',
			ADD INDEX idx_strategy (strategy)`
		if _, err := m.db.ExecContext(ctx, alterSQL); err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to add strategy column: %v", err)
			// 不返回错误，允许继续运行
		} else {
			log.Printf("[MySQLStore] Added strategy column to agents table")
		}
	}

	log.Printf("[MySQLStore] Agents table ensured")

	return nil
}

// GetAllAgents 获取所有 Agent 配置
func (m *MySQLStore) GetAllAgents(ctx context.Context) ([]types.AgentConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := m.db.QueryContext(queryCtx, `
		SELECT id, name, description, mcp_server_id, llm_id, system_prompt, 
		       strategy, enabled, is_default, created_at, updated_at
		FROM agents
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []types.AgentConfig
	for rows.Next() {
		var agent types.AgentConfig
		var description, llmID, systemPrompt, strategy sql.NullString

		err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&description,
			&agent.MCPServerID,
			&llmID,
			&systemPrompt,
			&strategy,
			&agent.Enabled,
			&agent.IsDefault,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			log.Printf("[MySQLStore] ERROR: Failed to scan agent row: %v", err)
			continue
		}

		if description.Valid {
			agent.Description = description.String
		}
		if llmID.Valid {
			agent.LLMID = llmID.String
		}
		if systemPrompt.Valid {
			agent.SystemPrompt = systemPrompt.String
		}
		if strategy.Valid {
			agent.Strategy = strategy.String
		}

		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, nil
}

// GetAgent 获取指定 Agent
func (m *MySQLStore) GetAgent(ctx context.Context, id string) (*types.AgentConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var agent types.AgentConfig
	var description, llmID, systemPrompt sql.NullString

	var strategy sql.NullString
	err := m.db.QueryRowContext(queryCtx, `
		SELECT id, name, description, mcp_server_id, llm_id, system_prompt,
		       strategy, enabled, is_default, created_at, updated_at
		FROM agents
		WHERE id = ?
	`, id).Scan(
		&agent.ID,
		&agent.Name,
		&description,
		&agent.MCPServerID,
		&llmID,
		&systemPrompt,
		&strategy,
		&agent.Enabled,
		&agent.IsDefault,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query agent: %w", err)
	}

	if description.Valid {
		agent.Description = description.String
	}
	if llmID.Valid {
		agent.LLMID = llmID.String
	}
	if systemPrompt.Valid {
		agent.SystemPrompt = systemPrompt.String
	}

	return &agent, nil
}

// SetAgent 新增或更新 Agent
func (m *MySQLStore) SetAgent(ctx context.Context, agent types.AgentConfig) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().Unix()
	if agent.CreatedAt == 0 {
		agent.CreatedAt = now
	}
	agent.UpdatedAt = now

	// 如果设置为默认，先取消其他默认 Agent
	if agent.IsDefault {
		_, err := m.db.ExecContext(queryCtx, `
			UPDATE agents SET is_default = FALSE WHERE is_default = TRUE
		`)
		if err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to unset other default agents: %v", err)
		}
	}

	// 使用 INSERT ... ON DUPLICATE KEY UPDATE
	_, err := m.db.ExecContext(queryCtx, `
		INSERT INTO agents (
			id, name, description, mcp_server_id, llm_id, system_prompt,
			strategy, enabled, is_default, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			description = VALUES(description),
			mcp_server_id = VALUES(mcp_server_id),
			llm_id = VALUES(llm_id),
			system_prompt = VALUES(system_prompt),
			strategy = VALUES(strategy),
			enabled = VALUES(enabled),
			is_default = VALUES(is_default),
			updated_at = VALUES(updated_at)
	`, agent.ID, agent.Name, agent.Description, agent.MCPServerID, agent.LLMID,
		agent.SystemPrompt, agent.Strategy, agent.Enabled, agent.IsDefault, agent.CreatedAt, agent.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	log.Printf("[MySQLStore] Agent saved: ID=%s, Name=%s, Strategy=%s, SystemPrompt length=%d",
		agent.ID, agent.Name, agent.Strategy, len(agent.SystemPrompt))

	return nil
}

// DeleteAgent 删除 Agent
func (m *MySQLStore) DeleteAgent(ctx context.Context, id string) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.db.ExecContext(queryCtx, `DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	return nil
}

// GetDefaultAgent 获取默认 Agent
func (m *MySQLStore) GetDefaultAgent(ctx context.Context) (*types.AgentConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var agent types.AgentConfig
	var description, llmID, systemPrompt sql.NullString

	var strategy sql.NullString
	err := m.db.QueryRowContext(queryCtx, `
		SELECT id, name, description, mcp_server_id, llm_id, system_prompt,
		       strategy, enabled, is_default, created_at, updated_at
		FROM agents
		WHERE is_default = TRUE AND enabled = TRUE
		LIMIT 1
	`).Scan(
		&agent.ID,
		&agent.Name,
		&description,
		&agent.MCPServerID,
		&llmID,
		&systemPrompt,
		&strategy,
		&agent.Enabled,
		&agent.IsDefault,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query default agent: %w", err)
	}

	if description.Valid {
		agent.Description = description.String
	}
	if llmID.Valid {
		agent.LLMID = llmID.String
	}
	if systemPrompt.Valid {
		agent.SystemPrompt = systemPrompt.String
	}
	if strategy.Valid {
		agent.Strategy = strategy.String
	}

	return &agent, nil
}

// Close 关闭数据库连接
func (m *MySQLStore) Close() error {
	return m.db.Close()
}

// MigrateFromFileStore 从文件存储迁移数据到 MySQL（一次性操作）
func (m *MySQLStore) MigrateFromFileStore(fileStore *PersistentStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agents := fileStore.GetAllAgents()
	log.Printf("[MySQLStore] Migrating %d agents from file store to MySQL", len(agents))

	for _, agent := range agents {
		if err := m.SetAgent(ctx, agent); err != nil {
			log.Printf("[MySQLStore] ERROR: Failed to migrate agent %s: %v", agent.ID, err)
			continue
		}
		log.Printf("[MySQLStore] Migrated agent: %s", agent.ID)
	}

	log.Printf("[MySQLStore] Migration completed")
	return nil
}

