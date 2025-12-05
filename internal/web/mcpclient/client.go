package mcpclient

import (
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// Client MCP 客户端封装
type Client struct {
	server *mcp.Server
}

// NewClient 创建新的 MCP 客户端
func NewClient(server *mcp.Server) *Client {
	return &Client{
		server: server,
	}
}

// ListTools 列出所有可用工具
func (c *Client) ListTools() []mcp.Tool {
	return c.server.ListTools()
}

// CallTool 调用指定工具
func (c *Client) CallTool(name string, args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	return c.server.CallTool(name, args)
}

