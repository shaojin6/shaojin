package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/store"
	"github.com/your-org/k8s-mcp-agent/internal/web/types"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// DBCache 数据库缓存实现（基于持久化存储）
type DBCache struct {
	store *store.PersistentStore
}

// NewDBCache 创建数据库缓存
func NewDBCache(store *store.PersistentStore) *DBCache {
	return &DBCache{
		store: store,
	}
}

// GetTools 从数据库获取工具列表
// identifier 应该是 serverId，使用严格查找避免 fallback 导致的数据串改
func (d *DBCache) GetTools(ctx context.Context, identifier string) ([]mcp.Tool, error) {
	// 使用严格按 serverId 查找，避免 fallback 到 name 导致的数据串改
	mcpConfig := d.store.GetRemoteMCPByServerID(identifier)
	if mcpConfig == nil {
		return nil, nil // 配置不存在
	}

	if len(mcpConfig.Tools) == 0 {
		return nil, nil // 工具列表为空
	}

	// 转换为 mcp.Tool 格式
	tools := make([]mcp.Tool, 0, len(mcpConfig.Tools))
	for _, cachedTool := range mcpConfig.Tools {
		// 转换 InputSchema
		var inputSchema mcp.ToolSchema
		if cachedTool.InputSchema != nil {
			if schemaType, ok := cachedTool.InputSchema["type"].(string); ok {
				inputSchema.Type = schemaType
			}
			if props, ok := cachedTool.InputSchema["properties"].(map[string]interface{}); ok {
				inputSchema.Properties = props
			}
			if required, ok := cachedTool.InputSchema["required"].([]interface{}); ok {
				inputSchema.Required = make([]string, 0, len(required))
				for _, r := range required {
					if str, ok := r.(string); ok {
						inputSchema.Required = append(inputSchema.Required, str)
					}
				}
			}
		}

		tools = append(tools, mcp.Tool{
			Name:        cachedTool.Name,
			Description: cachedTool.Description,
			InputSchema: inputSchema,
		})
	}

	return tools, nil
}

// SetTools 将工具列表写入数据库
// identifier 应该是 serverId，使用严格查找避免 fallback 导致的数据串改
func (d *DBCache) SetTools(ctx context.Context, identifier string, tools []mcp.Tool, ttl time.Duration) error {
	// 使用严格按 serverId 查找，避免 fallback 到 name 导致的数据串改
	mcpConfig := d.store.GetRemoteMCPByServerID(identifier)
	if mcpConfig == nil {
		return fmt.Errorf("MCP config not found: %s", identifier)
	}

	// 转换为持久化格式
	cachedTools := make([]types.Tool, 0, len(tools))
	for _, tool := range tools {
		// 转换 InputSchema 为 map
		inputSchemaMap := make(map[string]interface{})
		inputSchemaMap["type"] = tool.InputSchema.Type
		if tool.InputSchema.Properties != nil {
			inputSchemaMap["properties"] = tool.InputSchema.Properties
		}
		if len(tool.InputSchema.Required) > 0 {
			required := make([]interface{}, len(tool.InputSchema.Required))
			for i, r := range tool.InputSchema.Required {
				required[i] = r
			}
			inputSchemaMap["required"] = required
		}

		cachedTools = append(cachedTools, types.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchemaMap,
			Annotations: make(map[string]interface{}),
		})
	}

	mcpConfig.Tools = cachedTools
	// 使用当前时间作为最后更新时间
	mcpConfig.ToolsLastUpdate = time.Now().Unix()
	d.store.SetRemoteMCP(*mcpConfig)

	return nil
}

// DeleteTools 删除工具列表缓存
// identifier 应该是 serverId，使用严格查找避免 fallback 导致的数据串改
func (d *DBCache) DeleteTools(ctx context.Context, identifier string) error {
	// 使用严格按 serverId 查找，避免 fallback 到 name 导致的数据串改
	mcpConfig := d.store.GetRemoteMCPByServerID(identifier)
	if mcpConfig == nil {
		return nil // 配置不存在，无需删除
	}

	mcpConfig.Tools = nil
	mcpConfig.ToolsLastUpdate = 0
	d.store.SetRemoteMCP(*mcpConfig)

	return nil
}

// IsAvailable 检查数据库缓存是否可用
func (d *DBCache) IsAvailable() bool {
	return d.store != nil
}
