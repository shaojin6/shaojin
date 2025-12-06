package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

	// 创建 remote_mcp_configs 表
	remoteMCPDDL := `
CREATE TABLE IF NOT EXISTS remote_mcp_configs (
    server_id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    icon VARCHAR(255),
    timeout INT DEFAULT 30,
    sse_read_timeout INT DEFAULT 300,
    frontend_timeout INT DEFAULT 0,
    headers TEXT,
    tools_endpoint VARCHAR(500),
    enabled BOOLEAN DEFAULT TRUE,
    tools TEXT,
    tools_last_update BIGINT,
    last_update BIGINT NOT NULL,
    INDEX idx_enabled (enabled),
    INDEX idx_name (name),
    INDEX idx_type (type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

	if _, err := m.db.ExecContext(ctx, remoteMCPDDL); err != nil {
		return fmt.Errorf("failed to create remote_mcp_configs table: %w", err)
	}

	log.Printf("[MySQLStore] Remote MCP configs table ensured")

	// 检查并添加 frontend_timeout 列（如果不存在）
	if err := m.ensureColumn(ctx, "remote_mcp_configs", "frontend_timeout", "INT DEFAULT 0"); err != nil {
		log.Printf("[MySQLStore] WARNING: Failed to ensure frontend_timeout column: %v", err)
		// 不返回错误，继续执行
	}

	// 创建 agents 表
	agentsDDL := `
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mcp_server_id VARCHAR(255) NOT NULL,
    llm_id VARCHAR(255),
    system_prompt LONGTEXT,
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_mcp_server_id (mcp_server_id),
    INDEX idx_enabled (enabled),
    INDEX idx_is_default (is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

	if _, err := m.db.ExecContext(ctx, agentsDDL); err != nil {
		return fmt.Errorf("failed to create agents table: %w", err)
	}

	log.Printf("[MySQLStore] Agents table ensured")

	return nil
}

// ensureColumn 确保列存在，如果不存在则添加
func (m *MySQLStore) ensureColumn(ctx context.Context, tableName, columnName, columnDef string) error {
	// 检查列是否存在
	var count int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		AND TABLE_NAME = ? 
		AND COLUMN_NAME = ?
	`, tableName, columnName).Scan(&count)

	if err != nil {
		return fmt.Errorf("failed to check column existence: %w", err)
	}

	if count > 0 {
		// 列已存在
		return nil
	}

	// 列不存在，添加列
	alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	if _, err := m.db.ExecContext(ctx, alterSQL); err != nil {
		return fmt.Errorf("failed to add column %s to %s: %w", columnName, tableName, err)
	}

	log.Printf("[MySQLStore] Added column %s to table %s", columnName, tableName)
	return nil
}

// GetRemoteMCPConfig 从 MySQL 获取 RemoteMCP 配置（用于恢复 headers 等完整配置）
func (m *MySQLStore) GetRemoteMCPConfig(ctx context.Context, serverID string) (*types.RemoteMCPConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var config types.RemoteMCPConfig
	var icon, headers, toolsEndpoint, tools sql.NullString
	var toolsLastUpdate sql.NullInt64

	var frontendTimeout sql.NullInt64
	err := m.db.QueryRowContext(queryCtx, `
		SELECT server_id, name, type, base_url, icon, timeout, sse_read_timeout,
		       COALESCE(frontend_timeout, 0) as frontend_timeout,
		       headers, tools_endpoint, enabled, tools, tools_last_update, last_update
		FROM remote_mcp_configs
		WHERE server_id = ?
	`, serverID).Scan(
		&config.ServerID, &config.Name, &config.Type, &config.BaseURL, &icon,
		&config.Timeout, &config.SSEReadTimeout, &frontendTimeout, &headers, &toolsEndpoint,
		&config.Enabled, &tools, &toolsLastUpdate, &config.LastUpdate,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query remote mcp config: %w", err)
	}

	config.Icon = icon.String
	config.ToolsEndpoint = toolsEndpoint.String
	
	// 解析 headers（JSON 格式）
	if headers.Valid && headers.String != "" {
		if err := json.Unmarshal([]byte(headers.String), &config.Headers); err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to parse headers for %s: %v", serverID, err)
			config.Headers = make(map[string]string)
		}
	} else {
		config.Headers = make(map[string]string)
	}

	// 解析 tools（JSON 格式）
	if tools.Valid && tools.String != "" {
		if err := json.Unmarshal([]byte(tools.String), &config.Tools); err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to parse tools for %s: %v", serverID, err)
			config.Tools = nil
		}
	}

	if toolsLastUpdate.Valid {
		config.ToolsLastUpdate = toolsLastUpdate.Int64
	}

	// 处理 frontendTimeout
	if frontendTimeout.Valid {
		config.FrontendTimeout = int(frontendTimeout.Int64)
	}

	return &config, nil
}

// GetAllRemoteMCPConfigs 从 MySQL 获取所有 RemoteMCP 配置
func (m *MySQLStore) GetAllRemoteMCPConfigs(ctx context.Context) ([]types.RemoteMCPConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := m.db.QueryContext(queryCtx, `
		SELECT server_id, name, type, base_url, icon, timeout, sse_read_timeout,
		       COALESCE(frontend_timeout, 0) as frontend_timeout,
		       headers, tools_endpoint, enabled, tools, tools_last_update, last_update
		FROM remote_mcp_configs
		ORDER BY last_update DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query remote mcp configs: %w", err)
	}
	defer rows.Close()

	var configs []types.RemoteMCPConfig
	for rows.Next() {
		var config types.RemoteMCPConfig
		var icon, headers, toolsEndpoint, tools sql.NullString
		var frontendTimeout, toolsLastUpdate sql.NullInt64

		err := rows.Scan(
			&config.ServerID, &config.Name, &config.Type, &config.BaseURL, &icon,
			&config.Timeout, &config.SSEReadTimeout, &frontendTimeout, &headers, &toolsEndpoint,
			&config.Enabled, &tools, &toolsLastUpdate, &config.LastUpdate,
		)
		if err != nil {
			log.Printf("[MySQLStore] ERROR: Failed to scan remote mcp config row: %v", err)
			continue
		}

		config.Icon = icon.String
		config.ToolsEndpoint = toolsEndpoint.String

		// 解析 headers（JSON 格式）
		if headers.Valid && headers.String != "" {
			// 确保 Headers 字段已初始化
			if config.Headers == nil {
				config.Headers = make(map[string]string)
			}
			if err := json.Unmarshal([]byte(headers.String), &config.Headers); err != nil {
				log.Printf("[MySQLStore] ERROR: Failed to parse headers for %s: %v (headers string: %s)", config.ServerID, err, headers.String)
				config.Headers = make(map[string]string)
			} else {
				log.Printf("[MySQLStore] Successfully parsed headers for %s: %d headers", config.ServerID, len(config.Headers))
			}
		} else {
			log.Printf("[MySQLStore] Headers is NULL or empty for %s", config.ServerID)
			config.Headers = make(map[string]string)
		}

		// 解析 tools（JSON 格式）
		if tools.Valid && tools.String != "" {
			if err := json.Unmarshal([]byte(tools.String), &config.Tools); err != nil {
				log.Printf("[MySQLStore] WARNING: Failed to parse tools for %s: %v", config.ServerID, err)
				config.Tools = nil
			}
		}

		if toolsLastUpdate.Valid {
			config.ToolsLastUpdate = toolsLastUpdate.Int64
		}

		// 处理 frontendTimeout
		if frontendTimeout.Valid {
			config.FrontendTimeout = int(frontendTimeout.Int64)
		}

		configs = append(configs, config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating remote mcp configs: %w", err)
	}

	return configs, nil
}

// SetRemoteMCPConfig 保存 RemoteMCP 配置到 MySQL
func (m *MySQLStore) SetRemoteMCPConfig(ctx context.Context, config types.RemoteMCPConfig) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 确保 ServerID 不为空
	if config.ServerID == "" {
		return fmt.Errorf("server_id cannot be empty")
	}

	now := time.Now().Unix()
	if config.LastUpdate == 0 {
		config.LastUpdate = now
	}

	// 序列化 headers 为 JSON
	headersJSON := "{}"
	if config.Headers != nil && len(config.Headers) > 0 {
		headersBytes, err := json.Marshal(config.Headers)
		if err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to marshal headers for %s: %v", config.ServerID, err)
		} else {
			headersJSON = string(headersBytes)
		}
	}

	// 序列化 tools 为 JSON
	toolsJSON := sql.NullString{}
	if config.Tools != nil && len(config.Tools) > 0 {
		toolsBytes, err := json.Marshal(config.Tools)
		if err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to marshal tools for %s: %v", config.ServerID, err)
		} else {
			toolsJSON = sql.NullString{String: string(toolsBytes), Valid: true}
		}
	}

	// 处理 frontendTimeout
	var frontendTimeout sql.NullInt64
	if config.FrontendTimeout > 0 {
		frontendTimeout = sql.NullInt64{Int64: int64(config.FrontendTimeout), Valid: true}
	}

	// 先检查是否已存在（避免覆盖）
	var existingID string
	err := m.db.QueryRowContext(queryCtx, `
		SELECT server_id FROM remote_mcp_configs WHERE server_id = ?
	`, config.ServerID).Scan(&existingID)
	
	if err == nil {
		// 已存在，返回错误（不允许覆盖）
		return fmt.Errorf("server_id '%s' already exists, cannot create duplicate", config.ServerID)
	} else if err != sql.ErrNoRows {
		// 查询出错
		return fmt.Errorf("failed to check existing config: %w", err)
	}
	
	// 不存在，执行 INSERT（新增）
	_, err = m.db.ExecContext(queryCtx, `
		INSERT INTO remote_mcp_configs (
			server_id, name, type, base_url, icon, timeout, sse_read_timeout,
			frontend_timeout, headers, tools_endpoint, enabled, tools, tools_last_update, last_update
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, config.ServerID, config.Name, config.Type, config.BaseURL, config.Icon,
		config.Timeout, config.SSEReadTimeout, frontendTimeout, headersJSON, config.ToolsEndpoint,
		config.Enabled, toolsJSON, config.ToolsLastUpdate, config.LastUpdate)

	if err != nil {
		return fmt.Errorf("failed to save remote mcp config: %w", err)
	}

	log.Printf("[MySQLStore] Remote MCP config saved: ServerID=%s, Name=%s, Headers count=%d",
		config.ServerID, config.Name, len(config.Headers))

	return nil
}

// UpdateRemoteMCPConfig 更新 RemoteMCP 配置到 MySQL（用于 PUT 请求）
// 注意：此方法会更新所有字段，以传入的 config 为准（从前端传入的值）
func (m *MySQLStore) UpdateRemoteMCPConfig(ctx context.Context, config types.RemoteMCPConfig) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 确保 ServerID 不为空
	if config.ServerID == "" {
		return fmt.Errorf("server_id cannot be empty")
	}

	now := time.Now().Unix()
	config.LastUpdate = now // 更新时总是更新 LastUpdate

	// 序列化 headers 为 JSON
	headersJSON := "{}"
	if config.Headers != nil && len(config.Headers) > 0 {
		headersBytes, err := json.Marshal(config.Headers)
		if err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to marshal headers for %s: %v", config.ServerID, err)
		} else {
			headersJSON = string(headersBytes)
		}
	}

	// 序列化 tools 为 JSON
	toolsJSON := sql.NullString{}
	if config.Tools != nil && len(config.Tools) > 0 {
		toolsBytes, err := json.Marshal(config.Tools)
		if err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to marshal tools for %s: %v", config.ServerID, err)
		} else {
			toolsJSON = sql.NullString{String: string(toolsBytes), Valid: true}
		}
	}

	// 处理 frontendTimeout
	var frontendTimeout sql.NullInt64
	if config.FrontendTimeout > 0 {
		frontendTimeout = sql.NullInt64{Int64: int64(config.FrontendTimeout), Valid: true}
	}

	// 使用 UPDATE
	result, err := m.db.ExecContext(queryCtx, `
		UPDATE remote_mcp_configs SET
			name = ?,
			type = ?,
			base_url = ?,
			icon = ?,
			timeout = ?,
			sse_read_timeout = ?,
			frontend_timeout = ?,
			headers = ?,
			tools_endpoint = ?,
			enabled = ?,
			tools = ?,
			tools_last_update = ?,
			last_update = ?
		WHERE server_id = ?
	`, config.Name, config.Type, config.BaseURL, config.Icon,
		config.Timeout, config.SSEReadTimeout, frontendTimeout, headersJSON, config.ToolsEndpoint,
		config.Enabled, toolsJSON, config.ToolsLastUpdate, config.LastUpdate, config.ServerID)

	if err != nil {
		return fmt.Errorf("failed to update remote mcp config: %w", err)
	}

	// 检查是否有记录被更新
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// 没有记录被更新，说明记录不存在
		return fmt.Errorf("remote mcp config with server_id '%s' not found", config.ServerID)
	}

	log.Printf("[MySQLStore] Remote MCP config updated: ServerID=%s, Name=%s, Headers count=%d",
		config.ServerID, config.Name, len(config.Headers))

	return nil
}

// DeleteRemoteMCPConfig 从 MySQL 删除 RemoteMCP 配置
func (m *MySQLStore) DeleteRemoteMCPConfig(ctx context.Context, serverID string) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if serverID == "" {
		return fmt.Errorf("server_id cannot be empty")
	}

	// 先查询是否存在，以便在日志中记录
	var name string
	err := m.db.QueryRowContext(queryCtx, `
		SELECT name FROM remote_mcp_configs WHERE server_id = ?
	`, serverID).Scan(&name)
	
	if err == sql.ErrNoRows {
		// 不存在，返回错误
		return fmt.Errorf("remote mcp config with server_id '%s' not found", serverID)
	} else if err != nil {
		return fmt.Errorf("failed to check existing config: %w", err)
	}

	// 执行删除
	result, err := m.db.ExecContext(queryCtx, `
		DELETE FROM remote_mcp_configs WHERE server_id = ?
	`, serverID)
	
	if err != nil {
		return fmt.Errorf("failed to delete remote mcp config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("[MySQLStore] WARNING: Failed to get rows affected: %v", err)
	} else {
		log.Printf("[MySQLStore] Remote MCP config deleted: ServerID=%s, Name=%s, RowsAffected=%d", serverID, name, rowsAffected)
	}

	return nil
}

// GetAllAgents 获取所有 Agent 配置
func (m *MySQLStore) GetAllAgents(ctx context.Context) ([]types.AgentConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := m.db.QueryContext(queryCtx, `
		SELECT id, name, description, mcp_server_id, llm_id, system_prompt, 
		       enabled, is_default, created_at, updated_at
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
		var description, llmID, systemPrompt sql.NullString

		err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&description,
			&agent.MCPServerID,
			&llmID,
			&systemPrompt,
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

	err := m.db.QueryRowContext(queryCtx, `
		SELECT id, name, description, mcp_server_id, llm_id, system_prompt,
		       enabled, is_default, created_at, updated_at
		FROM agents
		WHERE id = ?
	`, id).Scan(
		&agent.ID,
		&agent.Name,
		&description,
		&agent.MCPServerID,
		&llmID,
		&systemPrompt,
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
			enabled, is_default, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			description = VALUES(description),
			mcp_server_id = VALUES(mcp_server_id),
			llm_id = VALUES(llm_id),
			system_prompt = VALUES(system_prompt),
			enabled = VALUES(enabled),
			is_default = VALUES(is_default),
			updated_at = VALUES(updated_at)
	`, agent.ID, agent.Name, agent.Description, agent.MCPServerID, agent.LLMID,
		agent.SystemPrompt, agent.Enabled, agent.IsDefault, agent.CreatedAt, agent.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	log.Printf("[MySQLStore] Agent saved: ID=%s, Name=%s, SystemPrompt length=%d",
		agent.ID, agent.Name, len(agent.SystemPrompt))

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

// UpdateAgent 更新 Agent（用于 PUT 请求）
func (m *MySQLStore) UpdateAgent(ctx context.Context, agent types.AgentConfig) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().Unix()
	agent.UpdatedAt = now

	// 如果设置为默认，先取消其他默认 Agent
	if agent.IsDefault {
		_, err := m.db.ExecContext(queryCtx, `
			UPDATE agents SET is_default = FALSE WHERE is_default = TRUE AND id != ?
		`, agent.ID)
		if err != nil {
			log.Printf("[MySQLStore] WARNING: Failed to unset other default agents: %v", err)
		}
	}

	// 使用 UPDATE
	_, err := m.db.ExecContext(queryCtx, `
		UPDATE agents SET
			name = ?,
			description = ?,
			mcp_server_id = ?,
			llm_id = ?,
			system_prompt = ?,
			enabled = ?,
			is_default = ?,
			updated_at = ?
		WHERE id = ?
	`, agent.Name, agent.Description, agent.MCPServerID, agent.LLMID,
		agent.SystemPrompt, agent.Enabled, agent.IsDefault, agent.UpdatedAt, agent.ID)

	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	log.Printf("[MySQLStore] Agent updated: ID=%s, Name=%s, SystemPrompt length=%d",
		agent.ID, agent.Name, len(agent.SystemPrompt))

	return nil
}

// GetDefaultAgent 获取默认 Agent
func (m *MySQLStore) GetDefaultAgent(ctx context.Context) (*types.AgentConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var agent types.AgentConfig
	var description, llmID, systemPrompt sql.NullString

	err := m.db.QueryRowContext(queryCtx, `
		SELECT id, name, description, mcp_server_id, llm_id, system_prompt,
		       enabled, is_default, created_at, updated_at
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

