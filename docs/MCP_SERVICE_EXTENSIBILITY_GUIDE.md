# MCP 服务可扩展性使用指南

## 概述

系统现在支持**自动适配**不同类型的 MCP 服务，新增 MCP 服务时无需修改代码，只需配置即可。

## 核心特性

### 1. 自动探测（默认）

系统会自动尝试多种连接方式，按优先级顺序：

1. **Dify SSE 模式**：适用于 Ansible MCP 等服务
   - GET `/sse` → 等待 `event: endpoint` → 获取 session URL
   - POST `initialize` 和 `tools/list` 到 session URL

2. **StreamableHTTP 模式**：适用于支持流式响应的服务
   - 直接 POST JSON-RPC 请求到 BaseURL
   - 支持 JSON 和 SSE 两种响应格式

3. **普通 HTTP 模式**：适用于标准 REST API
   - 尝试常见路径：`/mcp/tools`, `/api/tools`, `/tools`, `/v1/tools`

### 2. 连接策略配置

如果自动探测失败，或者您知道服务使用的协议类型，可以强制指定连接策略：

- `auto`：自动探测（默认）
- `dify-sse`：强制使用 Dify SSE 模式
- `streamable-http`：强制使用 StreamableHTTP 模式
- `http`：强制使用普通 HTTP 模式

## 配置新 MCP 服务

### 步骤 1：在 Web UI 中添加配置

1. 进入"配置管理" → "MCP配置"
2. 点击"添加 MCP 服务"
3. 填写配置信息：

```
服务名称：my-new-mcp-service
服务器标识符：my-new-mcp
类型：HTTP
访问地址（BaseURL）：http://example.com/mcp
超时时间：30 秒
SSE 读取超时：300 秒
连接策略：auto（或留空，默认自动探测）
```

### 步骤 2：测试连接

1. 点击"测试连接"按钮
2. 系统会自动尝试所有连接方式
3. 查看测试结果，确认工具列表是否正确获取

### 步骤 3：启用服务

1. 勾选"启用"选项
2. 保存配置
3. 系统会自动刷新工具列表

## 配置示例

### 示例 1：Ansible MCP（Dify SSE 模式）

```json
{
  "name": "ansible-mcp-server",
  "serverId": "ansible-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091/sse",
  "connectionStrategy": "dify-sse",
  "sessionPathPattern": "/sse/messages/?session_id={session_id}",
  "timeout": 30,
  "sseReadTimeout": 600,
  "enabled": true
}
```

**说明**：
- `connectionStrategy: "dify-sse"`：强制使用 Dify SSE 模式
- `sessionPathPattern`：指定 session 路径模式（可选）
- `sseReadTimeout: 600`：SSE 读取超时设置为 10 分钟，确保有足够时间等待 endpoint 事件

### 示例 2：Kubernetes MCP（自动探测）

```json
{
  "name": "kubernetes-mcp-server",
  "serverId": "kubernetes-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30080/mcp",
  "connectionStrategy": "auto",
  "timeout": 30,
  "sseReadTimeout": 300,
  "enabled": true
}
```

**说明**：
- `connectionStrategy: "auto"`：自动探测连接方式
- 系统会自动尝试所有模式，选择第一个成功的

### 示例 3：自定义 MCP 服务（StreamableHTTP）

```json
{
  "name": "custom-mcp-service",
  "serverId": "custom-mcp",
  "type": "http",
  "baseUrl": "http://example.com/mcp",
  "connectionStrategy": "streamable-http",
  "timeout": 30,
  "sseReadTimeout": 300,
  "enabled": true
}
```

**说明**：
- `connectionStrategy: "streamable-http"`：强制使用 StreamableHTTP 模式
- 适用于支持流式响应的 MCP 服务

### 示例 4：标准 REST API（HTTP 模式）

```json
{
  "name": "rest-api-mcp",
  "serverId": "rest-api-mcp",
  "type": "http",
  "baseUrl": "http://example.com",
  "connectionStrategy": "http",
  "toolsEndpoint": "/api/tools",
  "timeout": 30,
  "enabled": true
}
```

**说明**：
- `connectionStrategy: "http"`：强制使用普通 HTTP 模式
- `toolsEndpoint: "/api/tools"`：指定工具端点路径

## 常见问题

### Q1：如何知道应该使用哪种连接策略？

**A**：建议先使用 `auto`（自动探测），系统会自动尝试所有方式。如果自动探测失败，可以：

1. 查看日志，了解哪种方式失败了
2. 根据 MCP 服务的文档，确定它使用的协议
3. 手动指定连接策略

### Q2：自动探测需要多长时间？

**A**：自动探测会按顺序尝试三种模式，每个模式都有超时限制：
- Dify SSE 模式：最多等待 `sseReadTimeout` 秒（默认 300 秒）
- StreamableHTTP 模式：最多等待 `timeout` 秒（默认 30 秒）
- HTTP 模式：最多等待 `timeout` 秒（默认 30 秒）

如果第一个模式失败，会立即尝试下一个。

### Q3：如何查看连接日志？

**A**：系统会在日志中记录：
- 使用的连接策略
- 自动探测的结果
- 工具发现的成功/失败信息

查看日志示例：
```
[RemoteClient] Auto-detecting connection strategy for ansible-mcp-server
[RemoteClient] Auto-detected: Dify SSE mode for ansible-mcp-server
```

### Q4：如果所有连接策略都失败了怎么办？

**A**：系统会返回详细的错误信息，包括：
- 每种策略的失败原因
- 建议的连接策略
- 可能的配置问题

根据错误信息调整配置后重试。

### Q5：可以同时配置多个 MCP 服务吗？

**A**：可以！系统支持同时配置和启用多个 MCP 服务。每个服务独立配置，互不影响。

### Q6：修改配置后需要重启服务吗？

**A**：不需要！系统会自动检测配置变更，并刷新工具列表。但建议使用"测试连接"功能验证配置是否正确。

## 最佳实践

1. **先测试后启用**：配置新服务后，先使用"测试连接"功能验证
2. **使用自动探测**：除非确定服务使用的协议，否则使用 `auto` 策略
3. **合理设置超时**：根据服务的响应速度调整超时时间
4. **查看日志**：遇到问题时，查看日志了解详细信息
5. **缓存工具列表**：系统会自动缓存工具列表，提高性能

## 技术细节

### 连接策略优先级（auto 模式）

1. **Dify SSE 模式**
   - 检查 BaseURL 是否返回 SSE 流
   - 查找 `event: endpoint` 事件
   - 如果成功，使用该模式

2. **StreamableHTTP 模式**
   - POST `initialize` 请求到 BaseURL
   - 检查响应格式（JSON 或 SSE）
   - 如果成功，使用该模式

3. **HTTP 模式**
   - 尝试常见路径获取工具列表
   - 如果成功，使用该模式

### Session 路径模式

对于 Dify SSE 模式，如果服务返回的 session 路径格式不同，可以使用 `sessionPathPattern` 配置：

- `{session_id}`：会被替换为实际的 session ID
- 示例：`/sse/messages/?session_id={session_id}`

## 总结

通过这个可扩展的架构，您可以：

✅ **零代码修改**：新增 MCP 服务只需配置  
✅ **自动适配**：系统自动尝试多种连接方式  
✅ **灵活配置**：可以强制指定连接策略  
✅ **易于调试**：提供详细的日志和错误信息  
✅ **向后兼容**：不影响现有服务的正常运行

现在，您可以轻松地添加任何符合 MCP 协议的服务，无需修改代码！

