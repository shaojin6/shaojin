package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/types"
)

const (
	configFileName = "web-config.json"
)

// PersistentStore 持久化存储实现
// 包装 Store 并添加持久化功能（支持 MySQL 和文件双重存储）
type PersistentStore struct {
	store      *Store
	configFile string
	mysqlStore *MySQLStore // MySQL 存储（可选）
	mu         sync.RWMutex
}

// NewPersistentStore 创建持久化存储
func NewPersistentStore() *PersistentStore {
	// 获取配置目录
	configDir := getConfigDir()
	configFile := filepath.Join(configDir, configFileName)

	ps := &PersistentStore{
		store:      GetStore(),
		configFile: configFile,
		mysqlStore: nil, // 稍后通过 SetMySQLStore 设置
	}

	// 不再从文件加载配置，统一使用 MySQL 存储
	// ps.loadFromFile()  // 已禁用：所有配置统一从 MySQL 加载

	return ps
}

// SetMySQLStore 设置 MySQL 存储（可选）
func (ps *PersistentStore) SetMySQLStore(mysqlStore *MySQLStore) {
	ps.mu.Lock()
	ps.mysqlStore = mysqlStore
	
	// 如果 MySQL 存储可用，从 MySQL 加载所有配置
	if mysqlStore != nil {
		ctx := context.Background()
		
		// 1. 加载所有 RemoteMCP 配置（包括 headers）
		// 注意：MySQL 数据优先级高于文件数据，会覆盖文件中的配置
		// 同时，如果 MySQL 中没有某个配置，应该从内存中清除（因为可能从文件加载了）
		remoteMCPs, err := mysqlStore.GetAllRemoteMCPConfigs(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to load Remote MCP configs from MySQL: %v", err)
		} else {
			// 获取 MySQL 中所有 serverId 的集合
			mysqlServerIds := make(map[string]bool)
			for _, mcp := range remoteMCPs {
				if mcp.ServerID != "" {
					mysqlServerIds[mcp.ServerID] = true
				}
			}
			
			// 清除内存中不在 MySQL 中的配置（这些是从文件加载的，但 MySQL 中已删除）
			existingMCPs := ps.store.GetAllRemoteMCPs()
			for _, mcp := range existingMCPs {
				if mcp.ServerID != "" && !mysqlServerIds[mcp.ServerID] {
					log.Printf("[PersistentStore] Removing MCP config '%s' from memory (not in MySQL, was loaded from file)", mcp.ServerID)
					ps.store.DeleteRemoteMCP(mcp.ServerID)
				}
			}
			
			log.Printf("[PersistentStore] Loaded %d Remote MCP configs from MySQL (will override file data)", len(remoteMCPs))
			for _, mcp := range remoteMCPs {
				// 确保 ServerID 不为空
				if mcp.ServerID == "" {
					log.Printf("[PersistentStore] WARNING: Skipping MCP config with empty ServerID: %s", mcp.Name)
					continue
				}
				// 从 MySQL 加载的数据会覆盖文件中的数据（MySQL 优先级更高）
				ps.store.SetRemoteMCP(mcp)
				headersInfo := "no headers"
				if mcp.Headers != nil && len(mcp.Headers) > 0 {
					headersInfo = fmt.Sprintf("%d headers", len(mcp.Headers))
				}
				log.Printf("[PersistentStore] Loaded MCP config from MySQL: ServerID=%s, Name=%s, %s", 
					mcp.ServerID, mcp.Name, headersInfo)
			}
		}
		
		// 2. 加载所有 Agent 配置
		agents, err := mysqlStore.GetAllAgents(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to load agents from MySQL: %v", err)
		} else {
			log.Printf("[PersistentStore] Loaded %d agents from MySQL", len(agents))
			for _, agent := range agents {
				ps.store.SetAgent(agent)
			}
		}

		// 3. 加载所有 K8s 配置
		k8sConfigs, err := mysqlStore.GetAllK8sConfigs(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to load K8s configs from MySQL: %v", err)
		} else {
			log.Printf("[PersistentStore] Loaded %d K8s configs from MySQL", len(k8sConfigs))
			// 清除内存中不在 MySQL 中的配置
			existingK8s := ps.store.GetAllK8sConfigs()
			mysqlK8sIds := make(map[string]bool)
			for _, k8s := range k8sConfigs {
				mysqlK8sIds[k8s.ID] = true
			}
			for _, k8s := range existingK8s {
				if !mysqlK8sIds[k8s.ID] {
					log.Printf("[PersistentStore] Removing K8s config '%s' from memory (not in MySQL, was loaded from file)", k8s.ID)
					ps.store.DeleteK8sConfig(k8s.ID)
				}
			}
			// 加载 MySQL 中的配置（覆盖文件数据）
			for _, k8s := range k8sConfigs {
				ps.store.SetK8sConfig(k8s)
				log.Printf("[PersistentStore] Loaded K8s config from MySQL: ID=%s, Name=%s, Mode=%s", k8s.ID, k8s.Name, k8s.Mode)
			}
		}

		// 4. 加载所有 LLM 配置
		llmConfigs, err := mysqlStore.GetAllLLMConfigs(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to load LLM configs from MySQL: %v", err)
		} else {
			log.Printf("[PersistentStore] Loaded %d LLM configs from MySQL", len(llmConfigs))
			// 清除内存中不在 MySQL 中的配置
			existingLLM := ps.store.GetAllLLMConfigs()
			mysqlLLMIds := make(map[string]bool)
			for _, llm := range llmConfigs {
				mysqlLLMIds[llm.ID] = true
			}
			for _, llm := range existingLLM {
				if !mysqlLLMIds[llm.ID] {
					log.Printf("[PersistentStore] Removing LLM config '%s' from memory (not in MySQL, was loaded from file)", llm.ID)
					ps.store.DeleteLLMConfig(llm.ID)
				}
			}
			// 加载 MySQL 中的配置（覆盖文件数据）
			for _, llm := range llmConfigs {
				ps.store.SetLLMConfig(llm)
				log.Printf("[PersistentStore] Loaded LLM config from MySQL: ID=%s, Name=%s, Provider=%s, IsDefault=%v", 
					llm.ID, llm.Name, llm.Provider, llm.IsDefault)
			}
		}

		ps.mu.Unlock()
		
		// 5. 在锁外恢复缺失的 MCP 服务（如果 Agent 引用了但不存在）
		if len(agents) > 0 {
			ps.restoreMissingMCPServicesFromAgents(ctx, mysqlStore)
		}
	} else {
		ps.mu.Unlock()
	}
}

// getConfigDir 获取配置目录
func getConfigDir() string {
	// 优先使用当前目录
	if cwd, err := os.Getwd(); err == nil {
		configDir := filepath.Join(cwd, ".config")
		os.MkdirAll(configDir, 0755)
		return configDir
	}
	// 备用：使用用户目录
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".k8s-mcp")
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return filepath.Join(home, ".k8s-mcp")
	}
	return ".config"
}

// loadFromFile 从文件加载配置
func (ps *PersistentStore) loadFromFile() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	data, err := os.ReadFile(ps.configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，使用默认配置
			return
		}
		fmt.Printf("Warning: Failed to load config file: %v\n", err)
		return
	}

	var config struct {
		// 兼容旧格式：单个配置
		K8s *types.K8sConfig `json:"k8s,omitempty"`
		LLM *types.LLMConfig `json:"llm,omitempty"`
		// 新格式：多个配置
		K8sList    []types.K8sConfig                 `json:"k8sList,omitempty"`
		LLMList    []types.LLMConfig                 `json:"llmList,omitempty"`
		RemoteMCPs map[string]*types.RemoteMCPConfig `json:"remoteMcps,omitempty"`
		Agents     map[string]*types.AgentConfig     `json:"agents,omitempty"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("Warning: Failed to parse config file: %v\n", err)
		return
	}

	// 恢复配置（兼容旧格式）
	if config.K8s != nil {
		if config.K8s.ID == "" {
			config.K8s.ID = "k8s-default"
		}
		ps.store.SetK8sConfig(*config.K8s)
	}
	if config.LLM != nil {
		if config.LLM.ID == "" {
			config.LLM.ID = "llm-default"
			config.LLM.IsDefault = true
		}
		ps.store.SetLLMConfig(*config.LLM)
	}
	// 新格式：多个配置
	if len(config.K8sList) > 0 {
		for _, k8s := range config.K8sList {
			if k8s.ID == "" {
				k8s.ID = fmt.Sprintf("k8s-%d", len(ps.store.k8sConfigs))
			}
			ps.store.SetK8sConfig(k8s)
		}
	}
	if len(config.LLMList) > 0 {
		for _, llm := range config.LLMList {
			if llm.ID == "" {
				llm.ID = fmt.Sprintf("llm-%d", len(ps.store.llmConfigs))
			}
			ps.store.SetLLMConfig(llm)
		}
	}
	if config.RemoteMCPs != nil {
		for _, mcp := range config.RemoteMCPs {
			ps.store.SetRemoteMCP(*mcp)
		}
	}
	if config.Agents != nil {
		for _, agent := range config.Agents {
			ps.store.SetAgent(*agent)
		}
	}
}

// saveToFile 保存配置到文件
func (ps *PersistentStore) saveToFile() error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	config := struct {
		K8sList    []types.K8sConfig                 `json:"k8sList"`
		LLMList    []types.LLMConfig                 `json:"llmList"`
		RemoteMCPs map[string]*types.RemoteMCPConfig `json:"remoteMcps"`
		Agents     map[string]*types.AgentConfig     `json:"agents"`
	}{
		K8sList:    ps.store.GetAllK8sConfigs(),
		LLMList:    ps.store.GetAllLLMConfigs(),
		RemoteMCPs: make(map[string]*types.RemoteMCPConfig),
		Agents:     make(map[string]*types.AgentConfig),
	}

	// 收集所有远程 MCP 配置
	// 严格使用 ServerID 作为 key，如果为空则跳过（不应该出现这种情况）
	allMcps := ps.store.GetAllRemoteMCPs()
	for _, mcp := range allMcps {
		if mcp.ServerID != "" {
			config.RemoteMCPs[mcp.ServerID] = &mcp
		} else {
			log.Printf("[PersistentStore] WARNING: Skipping MCP config with empty ServerID when saving to file (name: %s)", mcp.Name)
		}
	}

	// 收集所有 Agent 配置
	allAgents := ps.store.GetAllAgents()
	for _, agent := range allAgents {
		config.Agents[agent.ID] = &agent
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 确保目录存在
	configDir := filepath.Dir(ps.configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 写入文件（使用临时文件确保原子性）
	tmpFile := ps.configFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// 原子性替换
	if err := os.Rename(tmpFile, ps.configFile); err != nil {
		return fmt.Errorf("failed to rename config file: %w", err)
	}

	return nil
}

// GetAllK8sConfigs 获取所有 K8s 配置
func (ps *PersistentStore) GetAllK8sConfigs() []types.K8sConfig {
	return ps.store.GetAllK8sConfigs()
}

// GetK8sConfig 获取指定 K8s 配置
func (ps *PersistentStore) GetK8sConfig(id string) *types.K8sConfig {
	return ps.store.GetK8sConfig(id)
}

// GetEnabledK8sConfigs 获取所有启用的 K8s 配置
func (ps *PersistentStore) GetEnabledK8sConfigs() []types.K8sConfig {
	return ps.store.GetEnabledK8sConfigs()
}

// GetDefaultK8sConfig 获取默认（第一个启用的）K8s 配置
func (ps *PersistentStore) GetDefaultK8sConfig() *types.K8sConfig {
	return ps.store.GetDefaultK8sConfig()
}

// SetK8sConfig 设置 K8s 配置（持久化，只保存到 MySQL）
func (ps *PersistentStore) SetK8sConfig(config types.K8sConfig) {
	ps.store.SetK8sConfig(config)
	
	// 只保存到 MySQL（不保存到文件）
	if ps.mysqlStore != nil {
		if err := ps.mysqlStore.SetK8sConfig(context.Background(), config); err != nil {
			log.Printf("[PersistentStore] ERROR: Failed to save K8s config '%s' to MySQL: %v", config.ID, err)
		}
	}
}

// DeleteK8sConfig 删除 K8s 配置（持久化，只从 MySQL 删除）
func (ps *PersistentStore) DeleteK8sConfig(id string) {
	ps.store.DeleteK8sConfig(id)
	
	// 只从 MySQL 删除（不操作文件）
	if ps.mysqlStore != nil {
		if err := ps.mysqlStore.DeleteK8sConfig(context.Background(), id); err != nil {
			log.Printf("[PersistentStore] ERROR: Failed to delete K8s config '%s' from MySQL: %v", id, err)
		}
	}
}

// GetAllLLMConfigs 获取所有 LLM 配置
func (ps *PersistentStore) GetAllLLMConfigs() []types.LLMConfig {
	return ps.store.GetAllLLMConfigs()
}

// GetLLMConfig 获取指定 LLM 配置
func (ps *PersistentStore) GetLLMConfig(id string) *types.LLMConfig {
	return ps.store.GetLLMConfig(id)
}

// GetDefaultLLMConfig 获取默认 LLM 配置
func (ps *PersistentStore) GetDefaultLLMConfig() *types.LLMConfig {
	return ps.store.GetDefaultLLMConfig()
}

// SetLLMConfig 设置 LLM 配置（持久化，只保存到 MySQL）
func (ps *PersistentStore) SetLLMConfig(config types.LLMConfig) {
	ps.store.SetLLMConfig(config)
	
	// 只保存到 MySQL（不保存到文件）
	// TODO: 实现 MySQL SetLLMConfig 方法
	if ps.mysqlStore != nil {
		if err := ps.mysqlStore.SetLLMConfig(context.Background(), config); err != nil {
			log.Printf("[PersistentStore] ERROR: Failed to save LLM config '%s' to MySQL: %v", config.ID, err)
		}
	}
}

// DeleteLLMConfig 删除 LLM 配置（持久化，只从 MySQL 删除）
func (ps *PersistentStore) DeleteLLMConfig(id string) {
	ps.store.DeleteLLMConfig(id)
	
	// 只从 MySQL 删除（不操作文件）
	if ps.mysqlStore != nil {
		if err := ps.mysqlStore.DeleteLLMConfig(context.Background(), id); err != nil {
			log.Printf("[PersistentStore] ERROR: Failed to delete LLM config '%s' from MySQL: %v", id, err)
		}
	}
}

// GetRemoteMCPs 获取所有启用的远程 MCP 配置
func (ps *PersistentStore) GetRemoteMCPs() []types.RemoteMCPConfig {
	return ps.store.GetRemoteMCPs()
}

// GetAllRemoteMCPs 获取所有远程 MCP 配置
func (ps *PersistentStore) GetAllRemoteMCPs() []types.RemoteMCPConfig {
	return ps.store.GetAllRemoteMCPs()
}

// restoreMissingMCPServicesFromAgents 从 Agent 配置中恢复缺失的 MCP 服务
// 如果 Agent 引用了某个 MCP 服务，但该服务不存在，则自动创建默认配置
// 注意：此函数在锁外调用，内部会获取锁
func (ps *PersistentStore) restoreMissingMCPServicesFromAgents(ctx context.Context, mysqlStore *MySQLStore) {
	ps.mu.Lock()
	// 获取所有 Agent 配置（从内存存储）
	agents := ps.store.GetAllAgents()
	if len(agents) == 0 {
		return
	}
	
	// 收集所有被引用的 MCP serverId
	referencedServerIds := make(map[string]bool)
	for _, agent := range agents {
		if agent.MCPServerID != "" {
			referencedServerIds[agent.MCPServerID] = true
		}
	}
	
	// 获取所有现有的 MCP 服务
	existingMCPs := ps.store.GetAllRemoteMCPs()
	existingServerIds := make(map[string]bool)
	for _, mcp := range existingMCPs {
		// 严格使用 ServerID，如果为空则跳过（不应该出现这种情况）
		if mcp.ServerID != "" {
			existingServerIds[mcp.ServerID] = true
		} else {
			log.Printf("[PersistentStore] WARNING: Found MCP config with empty ServerID (name: %s), skipping", mcp.Name)
		}
	}
	
	// 检查是否有缺失的 MCP 服务
	missingServerIds := make([]string, 0)
	for serverId := range referencedServerIds {
		if !existingServerIds[serverId] {
			missingServerIds = append(missingServerIds, serverId)
		}
	}
	
	if len(missingServerIds) == 0 {
		return
	}
	
	log.Printf("[PersistentStore] Found %d missing MCP services referenced by agents: %v", len(missingServerIds), missingServerIds)
	
	// 为缺失的 MCP 服务创建默认配置
	configsToRestore := make([]*types.RemoteMCPConfig, 0)
	for _, serverId := range missingServerIds {
		var restoredConfig *types.RemoteMCPConfig
		
		// 优先级1：从 MySQL 恢复（保留 headers 等完整配置）
		// 注意：如果 MySQL 中没有（返回 nil），说明已被删除，不应该恢复
		if mysqlStore != nil {
			config, err := mysqlStore.GetRemoteMCPConfig(ctx, serverId)
			if err != nil {
				// 查询出错，记录警告但继续尝试其他恢复方式
				log.Printf("[PersistentStore] WARNING: Failed to get MCP config from MySQL for '%s': %v", serverId, err)
			} else if config != nil {
				// MySQL 中存在，恢复它
				log.Printf("[PersistentStore] Restored MCP service '%s' from MySQL (preserving headers and other config, headers count: %d)", 
					serverId, len(config.Headers))
				restoredConfig = config
			} else {
				// MySQL 中不存在（nil），说明已被明确删除，不应该恢复
				// 不再尝试从文件备份恢复或创建默认配置，避免自动恢复已删除的服务
				log.Printf("[PersistentStore] MCP service '%s' not found in MySQL (explicitly deleted), skipping auto-restore to respect deletion", serverId)
				// 跳过恢复，继续下一个
				continue
			}
		}
		
		// 优先级2：从文件备份恢复（已禁用，统一使用 MySQL 存储）
		// 不再从文件恢复，所有配置统一从 MySQL 加载
		if restoredConfig == nil {
			log.Printf("[PersistentStore] Skipping file backup restore for '%s' (file restore disabled, using MySQL only)", serverId)
		}
		
		// 优先级3：创建默认配置（仅在 MySQL 查询出错时，而不是记录不存在时）
		// 注意：如果 MySQL 中明确不存在（nil），说明已被删除，不应该创建默认配置
		if restoredConfig == nil {
			// 只有在 MySQL 查询出错（而不是记录不存在）时，才创建默认配置
			// 但这里 restoredConfig 已经是 nil，说明 MySQL 中不存在，不应该创建
			log.Printf("[PersistentStore] Skipping default config creation for '%s' (explicitly deleted from MySQL)", serverId)
		}
		
		if restoredConfig != nil {
			// 确保 ServerID 正确设置
			if restoredConfig.ServerID == "" {
				restoredConfig.ServerID = serverId
			}
			// 保存到内存
			ps.store.SetRemoteMCP(*restoredConfig)
			configsToRestore = append(configsToRestore, restoredConfig)
		}
	}
	ps.mu.Unlock()
	
	// 在锁外保存到 MySQL，避免死锁
	for _, config := range configsToRestore {
		ps.SetRemoteMCP(*config)
		log.Printf("[PersistentStore] Successfully saved restored MCP service '%s' to MySQL", config.ServerID)
	}
}

// tryRestoreFromFile 尝试从文件备份中恢复 MCP 配置（保留 headers 等完整配置）
func (ps *PersistentStore) tryRestoreFromFile(serverId string) *types.RemoteMCPConfig {
	// 读取配置文件（不需要锁，因为这是只读操作）
	data, err := os.ReadFile(ps.configFile)
	if err != nil {
		log.Printf("[PersistentStore] Failed to read config file for restore: %v", err)
		return nil
	}
	
	var config struct {
		RemoteMCPs map[string]*types.RemoteMCPConfig `json:"remoteMcps,omitempty"`
	}
	
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("[PersistentStore] Failed to parse config file for restore: %v", err)
		return nil
	}
	
	// 查找匹配的配置
	if config.RemoteMCPs != nil {
		// 先尝试用 serverId 查找
		if mcp, ok := config.RemoteMCPs[serverId]; ok && mcp != nil {
			// 创建副本，确保 ServerID 正确
			restored := *mcp
			restored.ServerID = serverId
			// 确保 Headers 不为 nil
			if restored.Headers == nil {
				restored.Headers = make(map[string]string)
			}
			log.Printf("[PersistentStore] Found MCP config in file backup for '%s', headers count: %d", serverId, len(restored.Headers))
			return &restored
		}
		// 如果没找到，尝试用 name 查找
		for _, mcp := range config.RemoteMCPs {
			if mcp != nil && (mcp.ServerID == serverId || mcp.Name == serverId) {
				// 创建副本，确保 ServerID 正确
				restored := *mcp
				restored.ServerID = serverId
				// 确保 Headers 不为 nil
				if restored.Headers == nil {
					restored.Headers = make(map[string]string)
				}
				log.Printf("[PersistentStore] Found MCP config in file backup for '%s' (by name), headers count: %d", serverId, len(restored.Headers))
				return &restored
			}
		}
	}
	
	log.Printf("[PersistentStore] No MCP config found in file backup for '%s'", serverId)
	return nil
}

// createDefaultMCPConfig 根据 serverId 创建默认的 MCP 配置
func (ps *PersistentStore) createDefaultMCPConfig(serverId string) *types.RemoteMCPConfig {
	now := time.Now().Unix()
	
	// 根据 serverId 创建默认配置
	switch serverId {
	case "kubernetes-mcp-server":
		return &types.RemoteMCPConfig{
			Name:           "kubernetes-mcp-server",
			ServerID:       "kubernetes-mcp-server",
			Type:           "http",
			BaseURL:        "http://11.0.1.110:30080/mcp",
			Icon:           "M",
			Timeout:        30,
			SSEReadTimeout: 300,
			Headers:        make(map[string]string),
			ToolsEndpoint:  "",
			Enabled:        true,
			LastUpdate:     now,
		}
	default:
		// 对于其他未知的 serverId，创建基本配置
		log.Printf("[PersistentStore] WARNING: Unknown MCP serverId '%s', creating basic config", serverId)
		return &types.RemoteMCPConfig{
			Name:           serverId,
			ServerID:       serverId,
			Type:           "http",
			BaseURL:        fmt.Sprintf("http://localhost:8080/%s", serverId),
			Icon:           "M",
			Timeout:        30,
			SSEReadTimeout: 300,
			Headers:        make(map[string]string),
			ToolsEndpoint:  "",
			Enabled:        true,
			LastUpdate:     now,
		}
	}
}

// GetRemoteMCP 获取指定远程 MCP 配置（支持 fallback 到 name 查找，用于 API 路由）
func (ps *PersistentStore) GetRemoteMCP(identifier string) *types.RemoteMCPConfig {
	return ps.store.GetRemoteMCP(identifier)
}

// GetRemoteMCPByServerID 严格按 serverId 获取远程 MCP 配置（无 fallback，用于工具缓存等场景）
func (ps *PersistentStore) GetRemoteMCPByServerID(serverID string) *types.RemoteMCPConfig {
	return ps.store.GetRemoteMCPByServerID(serverID)
}

// SetRemoteMCP 设置远程 MCP 配置（持久化，只保存到 MySQL）
// 注意：此方法会先检查是否存在，如果存在则更新，不存在则新增
func (ps *PersistentStore) SetRemoteMCP(config types.RemoteMCPConfig) {
	// 确保 ServerID 不为空
	if config.ServerID == "" {
		log.Printf("[PersistentStore] ERROR: ServerID is empty for MCP config '%s', cannot save", config.Name)
		return
	}

	// 先保存到内存
	ps.store.SetRemoteMCP(config)

	// 只保存到 MySQL（不保存到文件）
	if ps.mysqlStore != nil {
		ctx := context.Background()
		
		// 优化策略：直接尝试更新，如果更新失败（记录不存在），再尝试新增
		// 这样可以避免查询的开销，也更简单可靠
		if err := ps.mysqlStore.UpdateRemoteMCPConfig(ctx, config); err != nil {
			// 更新失败，检查是否是"记录不存在"的错误
			// 如果是，尝试新增；否则直接返回错误
			if strings.Contains(err.Error(), "not found") {
				// 记录不存在，尝试新增
				if insertErr := ps.mysqlStore.SetRemoteMCPConfig(ctx, config); insertErr != nil {
					// 新增也失败，记录错误
					log.Printf("[PersistentStore] ERROR: Failed to save Remote MCP config '%s' to MySQL (tried update then insert): update=%v, insert=%v", config.ServerID, err, insertErr)
				} else {
					log.Printf("[PersistentStore] Successfully saved Remote MCP config '%s' to MySQL (inserted, headers count: %d)", config.ServerID, len(config.Headers))
				}
			} else {
				// 其他错误（如数据库连接问题等），直接返回错误
				log.Printf("[PersistentStore] ERROR: Failed to update Remote MCP config '%s' to MySQL: %v", config.ServerID, err)
			}
		} else {
			// 更新成功
			log.Printf("[PersistentStore] Successfully updated Remote MCP config '%s' to MySQL (headers count: %d)", config.ServerID, len(config.Headers))
		}
	} else {
		log.Printf("[PersistentStore] ERROR: MySQL store not available, cannot save Remote MCP config '%s'", config.ServerID)
	}
}

// DeleteRemoteMCP 删除远程 MCP 配置（持久化，只从 MySQL 删除）
// identifier 可以是 serverId 或 name，但会使用实际的 serverId 进行删除
func (ps *PersistentStore) DeleteRemoteMCP(identifier string) {
	// 先获取配置信息，以便在日志中记录
	existing := ps.store.GetRemoteMCP(identifier)
	if existing == nil {
		log.Printf("[PersistentStore] WARNING: Remote MCP config '%s' not found in memory, skipping delete", identifier)
		return
	}
	
	// 确保使用 serverId 进行删除（而不是 name）
	serverId := existing.ServerID
	if serverId == "" {
		log.Printf("[PersistentStore] ERROR: Cannot delete MCP config with empty ServerID (identifier: %s, name: %s)", identifier, existing.Name)
		return
	}
	
	log.Printf("[PersistentStore] Deleting Remote MCP config: ServerID=%s, Name=%s", serverId, existing.Name)
	
	// 从内存删除（使用 serverId）
	ps.store.DeleteRemoteMCP(serverId)
	
	// 从 MySQL 删除（使用 serverId）
	// 注意：不再保存到文件备份，因为现在只使用 MySQL 存储
	if ps.mysqlStore != nil {
		ctx := context.Background()
		if err := ps.mysqlStore.DeleteRemoteMCPConfig(ctx, serverId); err != nil {
			log.Printf("[PersistentStore] ERROR: Failed to delete Remote MCP config '%s' from MySQL: %v", serverId, err)
		} else {
			log.Printf("[PersistentStore] Successfully deleted Remote MCP config '%s' (Name: %s) from MySQL", serverId, existing.Name)
		}
	} else {
		log.Printf("[PersistentStore] ERROR: MySQL store not available, cannot delete Remote MCP config '%s' from MySQL", serverId)
	}
}

// Agent 相关操作
func (ps *PersistentStore) GetAllAgents() []types.AgentConfig {
	// 优先从 MySQL 获取（如果可用）
	if ps.mysqlStore != nil {
		ctx := context.Background()
		agents, err := ps.mysqlStore.GetAllAgents(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to get agents from MySQL: %v (falling back to memory)", err)
		} else {
			// 同步到内存存储
			for _, agent := range agents {
				ps.store.SetAgent(agent)
			}
			return agents
		}
	}
	
	// 从内存存储获取
	return ps.store.GetAllAgents()
}

func (ps *PersistentStore) GetAgent(id string) *types.AgentConfig {
	// 优先从 MySQL 获取（如果可用）
	if ps.mysqlStore != nil {
		ctx := context.Background()
		agent, err := ps.mysqlStore.GetAgent(ctx, id)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to get agent from MySQL: %v (falling back to memory)", err)
		} else if agent != nil {
			// 同步到内存存储
			ps.store.SetAgent(*agent)
			return agent
		}
	}
	
	// 从内存存储获取
	return ps.store.GetAgent(id)
}

func (ps *PersistentStore) SetAgent(config types.AgentConfig) types.AgentConfig {
	agent := ps.store.SetAgent(config)
	
	// 只保存到 MySQL（不保存到文件）
	// 注意：此方法会先检查是否存在，如果存在则更新，不存在则新增
	if ps.mysqlStore != nil {
		ctx := context.Background()
		
		// 检查是否已存在
		existing, err := ps.mysqlStore.GetAgent(ctx, agent.ID)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to check existing agent: %v", err)
		}
		
		if existing != nil {
			// 已存在，使用更新方法
			if err := ps.mysqlStore.UpdateAgent(ctx, agent); err != nil {
				log.Printf("[PersistentStore] ERROR: Failed to update agent to MySQL: %v", err)
			} else {
				log.Printf("[PersistentStore] Agent updated to MySQL: ID=%s, SystemPrompt length=%d", agent.ID, len(agent.SystemPrompt))
			}
		} else {
			// 不存在，使用新增方法
			if err := ps.mysqlStore.SetAgent(ctx, agent); err != nil {
				log.Printf("[PersistentStore] ERROR: Failed to save agent to MySQL: %v", err)
			} else {
				log.Printf("[PersistentStore] Agent saved to MySQL: ID=%s, SystemPrompt length=%d", agent.ID, len(agent.SystemPrompt))
			}
		}
	} else {
		log.Printf("[PersistentStore] ERROR: MySQL store not available, cannot save agent '%s'", agent.ID)
	}
	
	return agent
}

func (ps *PersistentStore) DeleteAgent(id string) {
	// 优先从 MySQL 删除（如果可用）
	if ps.mysqlStore != nil {
		ctx := context.Background()
		if err := ps.mysqlStore.DeleteAgent(ctx, id); err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to delete agent from MySQL: %v (falling back to memory)", err)
		} else {
			log.Printf("[PersistentStore] Agent deleted from MySQL: ID=%s", id)
		}
	}
	
	// 从内存存储删除
	ps.store.DeleteAgent(id)
	// 不再保存到文件，统一使用 MySQL 存储
}

func (ps *PersistentStore) GetDefaultAgent() *types.AgentConfig {
	// 优先从 MySQL 获取（如果可用）
	if ps.mysqlStore != nil {
		ctx := context.Background()
		agent, err := ps.mysqlStore.GetDefaultAgent(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to get default agent from MySQL: %v (falling back to memory)", err)
		} else if agent != nil {
			// 同步到内存存储
			ps.store.SetAgent(*agent)
			return agent
		}
	}
	
	// 从内存存储获取
	return ps.store.GetDefaultAgent()
}
