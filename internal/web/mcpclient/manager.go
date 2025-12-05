package mcpclient

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/cache"
	"github.com/your-org/k8s-mcp-agent/internal/web/types"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// ConfigStore 配置存储接口
type ConfigStore interface {
	GetRemoteMCPs() []types.RemoteMCPConfig
	GetAllRemoteMCPs() []types.RemoteMCPConfig
}

// ToolManager 统一的工具管理器，管理本地和远程 MCP 工具
type ToolManager struct {
	mu            sync.RWMutex
	localClient   *Client
	remoteClients map[string]*RemoteClient // key: serverID or name
	toolToClient  map[string]string        // key: tool name, value: client identifier
	cfgStore      ConfigStore
	toolsCache    cache.Cache           // 工具列表缓存（Redis + DB）
	cachedTools   map[string][]mcp.Tool // key: identifier, value: cached tools (用于 ListAllTools)
}

// NewToolManager 创建新的工具管理器
func NewToolManager(localClient *Client, cfgStore ConfigStore, toolsCache cache.Cache) *ToolManager {
	manager := &ToolManager{
		localClient:   localClient,
		remoteClients: make(map[string]*RemoteClient),
		toolToClient:  make(map[string]string),
		cfgStore:      cfgStore,
		toolsCache:    toolsCache,
		cachedTools:   make(map[string][]mcp.Tool),
	}

	// 延迟加载远程 MCP 工具，避免阻塞服务启动
	// 远程工具将在首次使用时或通过 API 调用 RefreshRemoteTools 时加载
	// manager.RefreshRemoteTools()

	return manager
}

// RefreshRemoteTools 刷新所有远程 MCP 工具（使用缓存）
func (tm *ToolManager) RefreshRemoteTools() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取所有启用的远程 MCP 配置
	remoteMCPs := tm.cfgStore.GetRemoteMCPs()
	fmt.Printf("[ToolManager] Refreshing tools for %d remote MCP services\n", len(remoteMCPs))

	// 清理旧的客户端映射
	tm.remoteClients = make(map[string]*RemoteClient)
	tm.toolToClient = make(map[string]string)
	tm.cachedTools = make(map[string][]mcp.Tool)

	// 加载本地工具
	if tm.localClient != nil {
		localTools := tm.localClient.ListTools()
		for _, tool := range localTools {
			tm.toolToClient[tool.Name] = "local"
		}
	}

	// 加载远程工具（使用缓存）
	ctx := context.Background()
	for _, mcpConfig := range remoteMCPs {
		identifier := mcpConfig.ServerID
		if identifier == "" {
			identifier = mcpConfig.Name
		}

		// 1. 先尝试从缓存获取工具列表
		var tools []mcp.Tool
		if tm.toolsCache != nil {
			cachedTools, err := tm.toolsCache.GetTools(ctx, identifier)
			if err == nil && len(cachedTools) > 0 {
				tools = cachedTools
				// 保存到 cachedTools 映射中，供 ListAllTools 使用
				tm.cachedTools[identifier] = tools
				fmt.Printf("[ToolManager] Loaded %d tools from cache for %s\n", len(tools), identifier)
			} else {
				fmt.Printf("[ToolManager] Cache miss for %s (err: %v, count: %d)\n", identifier, err, len(cachedTools))
			}
		}

		// 2. 如果缓存未命中，创建远程客户端并获取工具
		if len(tools) == 0 {
			remoteClient, err := NewRemoteClient(RemoteMCPConfig{
				Name:           mcpConfig.Name,
				ServerID:       mcpConfig.ServerID,
				Type:           mcpConfig.Type,
				BaseURL:        mcpConfig.BaseURL,
				Timeout:        mcpConfig.Timeout,
				SSEReadTimeout: mcpConfig.SSEReadTimeout,
				Headers:        mcpConfig.Headers,
				ToolsEndpoint:  mcpConfig.ToolsEndpoint,
			})

			if err != nil {
				// 如果创建失败，记录错误但继续处理其他 MCP
				fmt.Printf("Warning: Failed to create remote MCP client for %s: %v\n", identifier, err)
				continue
			}

			tm.remoteClients[identifier] = remoteClient
			tools = remoteClient.ListTools()

			// 3. 将工具列表写入缓存和 cachedTools 映射
			if len(tools) > 0 {
				tm.cachedTools[identifier] = tools
				if tm.toolsCache != nil {
					if err := tm.toolsCache.SetTools(ctx, identifier, tools, 24*time.Hour); err != nil {
						fmt.Printf("Warning: Failed to cache tools for %s: %v\n", identifier, err)
					}
				}
			}
		}

		// 无论是否使用缓存，都需要创建客户端用于后续调用工具
		// 注意：如果工具列表已在缓存中，跳过工具发现以加快速度
		if _, exists := tm.remoteClients[identifier]; !exists {
			// 如果工具列表已在缓存中，跳过工具发现（避免耗时5分钟的SSE连接）
			skipDiscovery := len(tools) > 0
			remoteClient, err := NewRemoteClientWithOptions(RemoteMCPConfig{
				Name:           mcpConfig.Name,
				ServerID:       mcpConfig.ServerID,
				Type:           mcpConfig.Type,
				BaseURL:        mcpConfig.BaseURL,
				Timeout:        mcpConfig.Timeout,
				SSEReadTimeout: mcpConfig.SSEReadTimeout,
				Headers:        mcpConfig.Headers,
				ToolsEndpoint:  mcpConfig.ToolsEndpoint,
			}, skipDiscovery)
			if err == nil {
				tm.remoteClients[identifier] = remoteClient
				if skipDiscovery {
					fmt.Printf("[ToolManager] Created remote client for %s (skipped discovery, tools from cache)\n", identifier)
				} else {
					fmt.Printf("[ToolManager] Created remote client for %s (with tool discovery)\n", identifier)
				}
			} else {
				// 如果客户端创建失败，记录警告
				// 注意：这不会影响工具列表的显示，但会影响工具调用
				fmt.Printf("Warning: Failed to create remote client for %s (tool calls will fail): %v\n", identifier, err)
			}
		}

		// 映射工具名称到客户端标识符
		for _, tool := range tools {
			// 如果工具名称已存在，记录警告但继续（远程工具优先）
			if existingClient, exists := tm.toolToClient[tool.Name]; exists {
				if existingClient != "local" {
					fmt.Printf("Warning: Tool %s already exists in client %s, overwriting with %s\n", tool.Name, existingClient, identifier)
				}
			}
			tm.toolToClient[tool.Name] = identifier
		}
	}
}

// ListAllTools 列出所有工具（本地 + 远程）
// 优先使用缓存的工具列表，确保即使客户端未连接也能返回工具
func (tm *ToolManager) ListAllTools() []mcp.Tool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.listAllToolsLocked()
}

func (tm *ToolManager) listAllToolsLocked() []mcp.Tool {
	var allTools []mcp.Tool

	if tm.localClient != nil {
		allTools = append(allTools, tm.localClient.ListTools()...)
	}

	for identifier, remoteClient := range tm.remoteClients {
		if cachedTools, exists := tm.cachedTools[identifier]; exists && len(cachedTools) > 0 {
			allTools = append(allTools, cachedTools...)
		} else {
			allTools = append(allTools, remoteClient.ListTools()...)
		}
	}
	return allTools
}

// ListToolsForAgent 返回指定 Agent 可用的工具列表
func (tm *ToolManager) ListToolsForAgent(agent *types.AgentConfig) []mcp.Tool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if agent == nil || agent.MCPServerID == "" {
		return tm.listAllToolsLocked()
	}

	identifier := agent.MCPServerID

	// 1. 优先使用缓存的工具列表
	if cached, exists := tm.cachedTools[identifier]; exists && len(cached) > 0 {
		fmt.Printf("[ToolManager] Found %d cached tools for agent %s (MCP: %s)\n", len(cached), agent.Name, identifier)
		return cached
	}

	// 2. 如果缓存未命中，尝试从远程客户端获取
	if remoteClient, exists := tm.remoteClients[identifier]; exists {
		tools := remoteClient.ListTools()
		if len(tools) > 0 {
			fmt.Printf("[ToolManager] Found %d tools from remote client for agent %s (MCP: %s)\n", len(tools), agent.Name, identifier)
			return tools
		}
	}

	// 3. 如果都没有，尝试从缓存系统获取（可能缓存系统有但 cachedTools 没有）
	if tm.toolsCache != nil {
		ctx := context.Background()
		cachedTools, err := tm.toolsCache.GetTools(ctx, identifier)
		if err == nil && len(cachedTools) > 0 {
			// 更新 cachedTools 映射
			tm.mu.RUnlock()
			tm.mu.Lock()
			tm.cachedTools[identifier] = cachedTools
			tm.mu.Unlock()
			tm.mu.RLock()
			fmt.Printf("[ToolManager] Loaded %d tools from cache system for agent %s (MCP: %s)\n", len(cachedTools), agent.Name, identifier)
			return cachedTools
		}
	}

	fmt.Printf("[ToolManager] WARNING: No tools found for agent %s (MCP: %s)\n", agent.Name, identifier)
	return nil
}

// CallTool 调用工具（自动路由到正确的客户端）
func (tm *ToolManager) CallTool(toolName string, args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	tm.mu.RLock()
	clientID, exists := tm.toolToClient[toolName]
	tm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	// 调用本地工具
	if clientID == "local" {
		if tm.localClient == nil {
			return nil, fmt.Errorf("local MCP client not initialized")
		}
		return tm.localClient.CallTool(toolName, args)
	}

	// 调用远程工具
	tm.mu.RLock()
	remoteClient, exists := tm.remoteClients[clientID]
	tm.mu.RUnlock()

	if !exists {
		// 如果客户端不存在，尝试创建（延迟创建）
		tm.mu.Lock()
		// 再次检查（双重检查锁定）
		if remoteClient, exists = tm.remoteClients[clientID]; !exists {
			// 从配置中获取 MCP 配置
			remoteMCPs := tm.cfgStore.GetRemoteMCPs()
			var mcpConfig *types.RemoteMCPConfig
			for _, mcp := range remoteMCPs {
				if mcp.ServerID == clientID || mcp.Name == clientID {
					mcpConfig = &mcp
					break
				}
			}
			if mcpConfig != nil {
				remoteClient, err := NewRemoteClient(RemoteMCPConfig{
					Name:           mcpConfig.Name,
					ServerID:       mcpConfig.ServerID,
					Type:           mcpConfig.Type,
					BaseURL:        mcpConfig.BaseURL,
					Timeout:        mcpConfig.Timeout,
					SSEReadTimeout: mcpConfig.SSEReadTimeout,
					Headers:        mcpConfig.Headers,
					ToolsEndpoint:  mcpConfig.ToolsEndpoint,
				})
				if err == nil {
					tm.remoteClients[clientID] = remoteClient
					exists = true
					fmt.Printf("[ToolManager] Created remote client for %s on-demand\n", clientID)
				} else {
					tm.mu.Unlock()
					return nil, fmt.Errorf("failed to create remote MCP client %s: %w", clientID, err)
				}
			} else {
				tm.mu.Unlock()
				return nil, fmt.Errorf("remote MCP client %s not found and config not available", clientID)
			}
		}
		tm.mu.Unlock()
	}

	log.Printf("[ToolManager] Calling remote tool %s on client %s with args: %+v", toolName, clientID, args)
	result, err := remoteClient.CallTool(toolName, args)
	if err != nil {
		log.Printf("[ToolManager] ERROR: Remote tool %s call failed: %v", toolName, err)
		return nil, err
	}
	if result != nil && len(result.Content) > 0 {
		resultPreview := result.Content[0].Text
		if len(resultPreview) > 200 {
			resultPreview = resultPreview[:200] + "..."
		}
		log.Printf("[ToolManager] Remote tool %s returned result (length: %d, preview: %s)", toolName, len(result.Content[0].Text), resultPreview)
	}
	return result, err
}

// GetToolClient 获取工具所属的客户端标识符
func (tm *ToolManager) GetToolClient(toolName string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.toolToClient[toolName]
}
