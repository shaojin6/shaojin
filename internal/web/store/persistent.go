package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// 加载已保存的配置
	ps.loadFromFile()

	return ps
}

// SetMySQLStore 设置 MySQL 存储（可选）
func (ps *PersistentStore) SetMySQLStore(mysqlStore *MySQLStore) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.mysqlStore = mysqlStore
	
	// 如果 MySQL 存储可用，从 MySQL 加载 Agent 配置
	if mysqlStore != nil {
		ctx := context.Background()
		agents, err := mysqlStore.GetAllAgents(ctx)
		if err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to load agents from MySQL: %v", err)
		} else {
			log.Printf("[PersistentStore] Loaded %d agents from MySQL", len(agents))
			for _, agent := range agents {
				ps.store.SetAgent(agent)
			}
		}
		
		// 自动恢复缺失的 MCP 服务（从 Agent 配置中检测）
		ps.restoreMissingMCPServicesFromAgents(ctx, mysqlStore)
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
	allMcps := ps.store.GetAllRemoteMCPs()
	for _, mcp := range allMcps {
		key := mcp.ServerID
		if key == "" {
			key = mcp.Name
		}
		config.RemoteMCPs[key] = &mcp
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

// SetK8sConfig 设置 K8s 配置（持久化）
func (ps *PersistentStore) SetK8sConfig(config types.K8sConfig) {
	ps.store.SetK8sConfig(config)
	ps.saveToFile()
}

// DeleteK8sConfig 删除 K8s 配置（持久化）
func (ps *PersistentStore) DeleteK8sConfig(id string) {
	ps.store.DeleteK8sConfig(id)
	ps.saveToFile()
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

// SetLLMConfig 设置 LLM 配置（持久化）
func (ps *PersistentStore) SetLLMConfig(config types.LLMConfig) {
	ps.store.SetLLMConfig(config)
	ps.saveToFile()
}

// DeleteLLMConfig 删除 LLM 配置（持久化）
func (ps *PersistentStore) DeleteLLMConfig(id string) {
	ps.store.DeleteLLMConfig(id)
	ps.saveToFile()
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
func (ps *PersistentStore) restoreMissingMCPServicesFromAgents(ctx context.Context, mysqlStore *MySQLStore) {
	// 获取所有 Agent 配置
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
		serverId := mcp.ServerID
		if serverId == "" {
			serverId = mcp.Name
		}
		existingServerIds[serverId] = true
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
	for _, serverId := range missingServerIds {
		// 根据 serverId 创建默认配置
		defaultConfig := ps.createDefaultMCPConfig(serverId)
		if defaultConfig != nil {
			log.Printf("[PersistentStore] Auto-restoring MCP service: %s (serverId: %s, baseUrl: %s)", 
				defaultConfig.Name, defaultConfig.ServerID, defaultConfig.BaseURL)
			
			// 保存到内存
			ps.store.SetRemoteMCP(*defaultConfig)
			
			// 保存到持久化存储（会保存到 MySQL 和文件）
			ps.SetRemoteMCP(*defaultConfig)
			log.Printf("[PersistentStore] Successfully restored MCP service '%s'", serverId)
			
			// 保存到文件备份
			if err := ps.saveToFile(); err != nil {
				log.Printf("[PersistentStore] WARNING: Failed to save restored MCP service to file: %v", err)
			}
		}
	}
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

// GetRemoteMCP 获取指定远程 MCP 配置
func (ps *PersistentStore) GetRemoteMCP(identifier string) *types.RemoteMCPConfig {
	return ps.store.GetRemoteMCP(identifier)
}

// SetRemoteMCP 设置远程 MCP 配置（持久化）
func (ps *PersistentStore) SetRemoteMCP(config types.RemoteMCPConfig) {
	ps.store.SetRemoteMCP(config)
	// 保存到文件备份
	if err := ps.saveToFile(); err != nil {
		log.Printf("[PersistentStore] ERROR: Failed to save Remote MCP config '%s' to file: %v", config.Name, err)
	} else {
		log.Printf("[PersistentStore] Successfully saved Remote MCP config '%s' to file", config.Name)
	}
}

// DeleteRemoteMCP 删除远程 MCP 配置（持久化）
func (ps *PersistentStore) DeleteRemoteMCP(identifier string) {
	ps.store.DeleteRemoteMCP(identifier)
	ps.saveToFile()
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
	
	// 优先保存到 MySQL（如果可用）
	if ps.mysqlStore != nil {
		ctx := context.Background()
		if err := ps.mysqlStore.SetAgent(ctx, agent); err != nil {
			log.Printf("[PersistentStore] WARNING: Failed to save agent to MySQL: %v (falling back to file)", err)
			ps.saveToFile() // MySQL 失败时回退到文件
		} else {
			log.Printf("[PersistentStore] Agent saved to MySQL: ID=%s, SystemPrompt length=%d", agent.ID, len(agent.SystemPrompt))
		}
	} else {
		// 没有 MySQL，保存到文件
		ps.saveToFile()
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
	ps.saveToFile()
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
