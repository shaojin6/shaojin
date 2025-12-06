package store

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/types"
)

var (
	instance *Store
	once     sync.Once
)

// Store 配置存储（内存实现，后续可替换为数据库）
type Store struct {
	mu         sync.RWMutex
	k8sConfigs map[string]*types.K8sConfig // 多个K8s集群配置，key为ID
	llmConfigs map[string]*types.LLMConfig // 多个LLM配置，key为ID
	remoteMCPs map[string]*types.RemoteMCPConfig
	agents     map[string]*types.AgentConfig
}

// GetStore 获取单例 Store
func GetStore() *Store {
	once.Do(func() {
		instance = &Store{
			k8sConfigs: make(map[string]*types.K8sConfig),
			llmConfigs: make(map[string]*types.LLMConfig),
			remoteMCPs: make(map[string]*types.RemoteMCPConfig),
			agents:     make(map[string]*types.AgentConfig),
		}
	})
	return instance
}

// GetAllK8sConfigs 获取所有 K8s 配置
func (s *Store) GetAllK8sConfigs() []types.K8sConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.K8sConfig, 0, len(s.k8sConfigs))
	for _, config := range s.k8sConfigs {
		result = append(result, *config)
	}
	return result
}

// GetK8sConfig 获取指定 K8s 配置
func (s *Store) GetK8sConfig(id string) *types.K8sConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.k8sConfigs[id]
}

// GetEnabledK8sConfigs 获取所有启用的 K8s 配置
func (s *Store) GetEnabledK8sConfigs() []types.K8sConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.K8sConfig, 0, len(s.k8sConfigs))
	for _, config := range s.k8sConfigs {
		if config.Enabled {
			result = append(result, *config)
		}
	}
	return result
}

// GetDefaultK8sConfig 获取默认（第一个启用的）K8s 配置
func (s *Store) GetDefaultK8sConfig() *types.K8sConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, config := range s.k8sConfigs {
		if config.Enabled {
			return config
		}
	}
	return nil
}

// SetK8sConfig 设置 K8s 配置（如果ID存在则更新，否则新增）
func (s *Store) SetK8sConfig(config types.K8sConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.LastUpdate = time.Now().Unix()
	if config.ID == "" {
		// 如果没有ID，生成一个
		config.ID = fmt.Sprintf("k8s-%d", time.Now().UnixNano())
	}
	s.k8sConfigs[config.ID] = &config
}

// DeleteK8sConfig 删除 K8s 配置
func (s *Store) DeleteK8sConfig(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.k8sConfigs, id)
}

// GetAllLLMConfigs 获取所有 LLM 配置
func (s *Store) GetAllLLMConfigs() []types.LLMConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.LLMConfig, 0, len(s.llmConfigs))
	for _, config := range s.llmConfigs {
		result = append(result, *config)
	}
	return result
}

// GetLLMConfig 获取指定 LLM 配置
func (s *Store) GetLLMConfig(id string) *types.LLMConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.llmConfigs[id]
}

// GetDefaultLLMConfig 获取默认 LLM 配置
func (s *Store) GetDefaultLLMConfig() *types.LLMConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, config := range s.llmConfigs {
		if config.IsDefault {
			return config
		}
	}
	// 如果没有默认配置，返回第一个
	for _, config := range s.llmConfigs {
		return config
	}
	return nil
}

// SetLLMConfig 设置 LLM 配置（如果ID存在则更新，否则新增）
func (s *Store) SetLLMConfig(config types.LLMConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.LastUpdate = time.Now().Unix()
	if config.ID == "" {
		// 如果没有ID，生成一个
		config.ID = fmt.Sprintf("llm-%d", time.Now().UnixNano())
	}
	// 如果更新现有配置且 APIKey 为空，保留原有的 APIKey
	if existing, exists := s.llmConfigs[config.ID]; exists && config.APIKey == "" {
		config.APIKey = existing.APIKey
	}
	// 如果设置为默认，取消其他配置的默认状态
	if config.IsDefault {
		for _, c := range s.llmConfigs {
			c.IsDefault = false
		}
	}
	s.llmConfigs[config.ID] = &config
}

// DeleteLLMConfig 删除 LLM 配置
func (s *Store) DeleteLLMConfig(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.llmConfigs, id)
}

// GetRemoteMCPs 获取所有远程 MCP 配置
func (s *Store) GetRemoteMCPs() []types.RemoteMCPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]types.RemoteMCPConfig, 0, len(s.remoteMCPs))
	for _, config := range s.remoteMCPs {
		if config.Enabled {
			result = append(result, *config)
		}
	}
	return result
}

// GetAllRemoteMCPs 获取所有远程 MCP 配置（包括禁用的）
func (s *Store) GetAllRemoteMCPs() []types.RemoteMCPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]types.RemoteMCPConfig, 0, len(s.remoteMCPs))
	for _, config := range s.remoteMCPs {
		result = append(result, *config)
	}
	return result
}

// SetRemoteMCP 设置远程 MCP 配置
func (s *Store) SetRemoteMCP(config types.RemoteMCPConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.LastUpdate = time.Now().Unix()
	// 严格使用 serverId 作为 key，不允许使用 name 作为 fallback
	// 如果 serverId 为空，这是一个错误，不应该保存
	if config.ServerID == "" {
		log.Printf("[Store] ERROR: Cannot save MCP config with empty ServerID (name: %s)", config.Name)
		return
	}
	s.remoteMCPs[config.ServerID] = &config
}

// GetRemoteMCP 获取指定远程 MCP 配置
// identifier 可以是 serverId 或 name，但优先使用 serverId
func (s *Store) GetRemoteMCP(identifier string) *types.RemoteMCPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 优先使用 serverId 查找（最常见的情况）
	if config, ok := s.remoteMCPs[identifier]; ok {
		return config
	}

	// 如果按 serverId 找不到，尝试按 name 查找（向后兼容）
	for _, config := range s.remoteMCPs {
		if config != nil && config.Name == identifier {
			return config
		}
	}

	return nil
}

// DeleteRemoteMCP 删除远程 MCP 配置
// identifier 可以是 serverId 或 name，但优先使用 serverId
func (s *Store) DeleteRemoteMCP(identifier string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 优先使用 serverId 删除（最常见的情况）
	if _, ok := s.remoteMCPs[identifier]; ok {
		delete(s.remoteMCPs, identifier)
		return
	}

	// 如果按 serverId 找不到，尝试按 name 查找并删除（向后兼容）
	for key, config := range s.remoteMCPs {
		if config != nil && config.Name == identifier {
			delete(s.remoteMCPs, key)
			return
		}
	}
}

// GetAllAgents 获取所有 Agent 配置
func (s *Store) GetAllAgents() []types.AgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.AgentConfig, 0, len(s.agents))
	for _, agent := range s.agents {
		result = append(result, *agent)
	}
	return result
}

// GetAgent 获取指定 Agent
func (s *Store) GetAgent(id string) *types.AgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[id]
}

// SetAgent 新增或更新 Agent
func (s *Store) SetAgent(agent types.AgentConfig) types.AgentConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if agent.ID == "" {
		agent.ID = fmt.Sprintf("agent-%d", time.Now().UnixNano())
	}
	if agent.CreatedAt == 0 {
		agent.CreatedAt = now
	}
	agent.UpdatedAt = now
	if agent.IsDefault {
		for _, existing := range s.agents {
			existing.IsDefault = false
		}
	}
	s.agents[agent.ID] = &agent
	return agent
}

// DeleteAgent 删除 Agent
func (s *Store) DeleteAgent(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, id)
}

// GetDefaultAgent 获取默认 Agent
func (s *Store) GetDefaultAgent() *types.AgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, agent := range s.agents {
		if agent.IsDefault && agent.Enabled {
			return agent
		}
	}
	// 如果没有默认，返回第一个启用的
	for _, agent := range s.agents {
		if agent.Enabled {
			return agent
		}
	}
	return nil
}

// GetPersistentStore 获取持久化存储实例
func GetPersistentStore() *PersistentStore {
	return NewPersistentStore()
}
