# MCP 服务可扩展性设计

## 设计目标

设计一个可扩展的架构，使得新增 MCP 服务时能够：
1. **自动适配**：无需修改代码，自动尝试多种连接方式
2. **灵活配置**：通过配置即可支持新的 MCP 服务
3. **智能探测**：自动检测服务类型和连接方式
4. **向后兼容**：不影响现有服务的正常运行

## 当前架构分析

### 现有连接模式

系统已经支持多种连接模式，按优先级自动尝试：

1. **Dify SSE 模式**（Ansible MCP）
   - GET `/sse` → 等待 `event: endpoint` → 获取 session URL
   - POST `initialize` 和 `tools/list` 到 session URL

2. **StreamableHTTP 模式**
   - 直接 POST JSON-RPC 请求到 BaseURL
   - 支持 JSON 和 SSE 两种响应格式

3. **普通 HTTP 模式**
   - 尝试常见路径：`/mcp/tools`, `/api/tools`, `/tools`, `/v1/tools`

### 配置结构

```go
type RemoteMCPConfig struct {
    Name            string            // 显示名称
    ServerID        string            // 服务器唯一标识符
    Type            string            // http, websocket, stdio
    BaseURL         string            // 服务终点 URL
    Timeout         int               // 超时时间（秒）
    SSEReadTimeout  int               // SSE 读取超时时间（秒）
    Headers         map[string]string // HTTP 请求头
    ToolsEndpoint   string            // 工具端点（可选）
    Enabled         bool              // 是否启用
}
```

## 增强设计

### 1. 连接策略配置

为 `RemoteMCPConfig` 添加连接策略配置：

```go
type ConnectionStrategy string

const (
    StrategyAuto        ConnectionStrategy = "auto"        // 自动探测（默认）
    StrategyDifySSE     ConnectionStrategy = "dify-sse"    // 强制使用 Dify SSE 模式
    StrategyStreamableHTTP ConnectionStrategy = "streamable-http" // 强制使用 StreamableHTTP
    StrategyHTTP        ConnectionStrategy = "http"        // 强制使用普通 HTTP
)

type RemoteMCPConfig struct {
    // ... 现有字段 ...
    ConnectionStrategy ConnectionStrategy `json:"connectionStrategy,omitempty"` // 连接策略
    SessionPathPattern string            `json:"sessionPathPattern,omitempty"` // Session 路径模式（如 /sse/messages/?session_id={session_id}）
}
```

### 2. 自动探测机制

实现智能探测流程：

```
1. 检查配置的连接策略
   ├─ 如果指定了策略 → 直接使用该策略
   └─ 如果是 "auto" → 进入自动探测流程

2. 自动探测流程（按优先级）
   ├─ 尝试 Dify SSE 模式
   │  ├─ GET BaseURL → 检查是否返回 SSE 流
   │  ├─ 检查是否有 endpoint 事件
   │  └─ 如果成功 → 使用该模式
   │
   ├─ 尝试 StreamableHTTP 模式
   │  ├─ POST initialize 到 BaseURL
   │  ├─ 检查响应格式（JSON 或 SSE）
   │  └─ 如果成功 → 使用该模式
   │
   └─ 尝试普通 HTTP 模式
      ├─ 尝试常见路径
      └─ 如果成功 → 使用该模式
```

### 3. 连接适配器模式

实现连接适配器接口，支持插件化扩展：

```go
// ConnectionAdapter 连接适配器接口
type ConnectionAdapter interface {
    // 检测是否支持该服务
    Detect(baseURL string, config RemoteMCPConfig) (bool, error)
    
    // 发现工具
    DiscoverTools(baseURL string, config RemoteMCPConfig) ([]mcp.Tool, error)
    
    // 调用工具
    CallTool(toolName string, arguments map[string]interface{}, config RemoteMCPConfig) (interface{}, error)
}

// 实现不同的适配器
type DifySSEAdapter struct {}
type StreamableHTTPAdapter struct {}
type HTTPAdapter struct {}
```

### 4. 连接测试功能

添加连接测试 API，帮助用户验证配置：

```go
// TestConnection 测试 MCP 服务连接
func TestConnection(config RemoteMCPConfig) (*ConnectionTestResult, error) {
    result := &ConnectionTestResult{
        Config:      config,
        Strategies:  []StrategyTestResult{},
        Recommended: "",
    }
    
    // 测试所有策略
    for _, strategy := range []ConnectionStrategy{
        StrategyDifySSE,
        StrategyStreamableHTTP,
        StrategyHTTP,
    } {
        testResult := testStrategy(strategy, config)
        result.Strategies = append(result.Strategies, testResult)
        if testResult.Success && result.Recommended == "" {
            result.Recommended = string(strategy)
        }
    }
    
    return result, nil
}
```

### 5. 增强的错误处理和日志

```go
// ConnectionError 连接错误
type ConnectionError struct {
    Strategy    ConnectionStrategy
    BaseURL     string
    Message     string
    Details     map[string]interface{}
    Suggestions []string
}

func (e *ConnectionError) Error() string {
    return fmt.Sprintf("Connection failed with strategy %s: %s", e.Strategy, e.Message)
}
```

## 实现方案

### 阶段 1：增强配置结构（向后兼容）

1. 在 `RemoteMCPConfig` 中添加可选字段
2. 默认值：`ConnectionStrategy = "auto"`
3. 保持现有代码的兼容性

### 阶段 2：实现自动探测

1. 重构 `discoverToolsSSE` 为通用探测函数
2. 实现策略选择逻辑
3. 添加详细的日志记录

### 阶段 3：连接测试 API

1. 添加 `/api/mcp/test-connection` 端点
2. 实现测试功能
3. 在前端添加测试按钮

### 阶段 4：适配器模式（可选）

1. 如果需要支持更多协议（如 WebSocket、stdio）
2. 实现适配器接口
3. 支持插件化扩展

## 使用示例

### 配置新 MCP 服务

```json
{
  "name": "my-new-mcp-service",
  "serverId": "my-new-mcp",
  "type": "http",
  "baseUrl": "http://example.com/mcp",
  "connectionStrategy": "auto",  // 自动探测
  "timeout": 30,
  "sseReadTimeout": 300,
  "enabled": true
}
```

### 强制使用特定策略

```json
{
  "name": "ansible-mcp",
  "serverId": "ansible-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091/sse",
  "connectionStrategy": "dify-sse",  // 强制使用 Dify SSE 模式
  "sessionPathPattern": "/sse/messages/?session_id={session_id}",
  "timeout": 30,
  "sseReadTimeout": 600,
  "enabled": true
}
```

## 优势

1. **零代码修改**：新增服务只需配置，无需修改代码
2. **自动适配**：系统自动尝试多种连接方式
3. **灵活配置**：可以强制指定连接策略
4. **易于调试**：提供连接测试功能
5. **向后兼容**：不影响现有服务

## 注意事项

1. **性能考虑**：自动探测可能需要多次尝试，建议使用缓存
2. **超时设置**：不同策略需要不同的超时时间
3. **错误处理**：提供清晰的错误信息和修复建议
4. **日志记录**：记录探测过程和结果，便于排查问题

