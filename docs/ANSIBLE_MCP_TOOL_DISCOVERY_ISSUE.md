# Ansible MCP 工具发现问题分析

## 问题现象

### 错误信息
```
Expected formats: {"tools": [...]}, JSON-RPC format, or tool array. 
Response body: event: endpoint data: /sse/messages/?
```

### 当前配置
- **服务终点 URL**: `http://11.0.1.110:30091`
- **工具端点**: `/sse`
- **超时时间**: 60 秒
- **SSE 读取超时**: 300 秒
- **请求头**: `Authorization: 1234`

## 问题分析

### 1. 工具发现流程

系统工具发现的完整流程应该是：

#### 步骤1：连接 SSE 端点
```
GET http://11.0.1.110:30091/sse
Headers: Accept: text/event-stream
```

**响应**：
```
event: endpoint
data: /sse/messages/?session_id=...
```

#### 步骤2：解析 endpoint 事件
系统应该：
1. 识别 `event: endpoint`
2. 提取 `data: /sse/messages/?session_id=...`
3. 构建完整 URL：`http://11.0.1.110:30091/sse/messages/?session_id=...`

#### 步骤3：发送 initialize 请求
```
POST http://11.0.1.110:30091/sse/messages/?session_id=...
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": "initialize",
  "method": "initialize",
  ...
}
```

#### 步骤4：发送 tools/list 请求
```
POST http://11.0.1.110:30091/sse/messages/?session_id=...
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": "tools-list",
  "method": "tools/list",
  "params": {}
}
```

#### 步骤5：解析工具列表
系统应该从 SSE 响应中解析工具列表，期望格式：
- `{"tools": [...]}`
- JSON-RPC 格式
- 工具数组

### 2. 问题根源分析

#### 问题1：endpoint 事件被当作工具列表响应

**错误信息**：
```
Response body: event: endpoint data: /sse/messages/?
```

**分析**：
- ❌ 系统在解析工具列表时，收到了 `event: endpoint` 事件
- ❌ 系统期望的是工具列表格式，但收到的是 endpoint 事件
- ❌ 说明系统可能在错误的阶段解析了响应

**可能的原因**：
1. **endpoint 事件解析失败**：系统没有正确识别和提取 endpoint 路径
2. **URL 构建错误**：虽然提取了 endpoint 路径，但构建的 URL 不正确
3. **请求发送到错误端点**：请求可能发送到了 `/sse` 而不是 `/sse/messages/?session_id=...`
4. **响应解析阶段错误**：系统在解析工具列表时，错误地解析了 endpoint 事件的响应

#### 问题2：配置问题

**当前配置**：
- BaseURL: `http://11.0.1.110:30091`
- ToolsEndpoint: `/sse`

**分析**：
- 系统会使用 `BaseURL + ToolsEndpoint` = `http://11.0.1.110:30091/sse` 连接 SSE
- 收到 endpoint 事件后，提取路径 `/sse/messages/?session_id=...`
- 构建完整 URL：`BaseURL + endpointPath` = `http://11.0.1.110:30091/sse/messages/?session_id=...`

**这个逻辑看起来是正确的**，但可能存在问题：

1. **session_id 可能不完整**：从错误信息看，`data: /sse/messages/?` 后面可能没有完整的 session_id
2. **超时时间可能不够**：SSE 读取超时 300 秒可能不够等待完整的 endpoint 事件

#### 问题3：认证问题

**官方文档说明**：
- 认证那里比较特殊，由于没有设置认证，可以随便写点啥都行
- 如果不填写没法点授权按钮
- 示例中填写了 `Authorization: 1`

**当前配置**：
- `Authorization: 1234`

**分析**：
- ✅ 这个配置应该是正确的（符合官方文档要求）
- ✅ 认证头不会影响工具发现（因为服务端不需要认证）
- ⚠️ 但可能在某些情况下，错误的认证头会导致请求被拒绝

### 3. 可能的问题场景

#### 场景1：endpoint 事件解析超时

**问题**：
- SSE 读取超时设置为 300 秒
- 但 endpoint 事件可能在 300 秒内没有完整接收
- 系统超时后，尝试解析不完整的响应

**表现**：
- 错误信息显示 `data: /sse/messages/?`（session_id 可能被截断）

#### 场景2：URL 构建错误

**问题**：
- 虽然提取了 endpoint 路径，但构建的 URL 不正确
- 可能构建成了 `http://11.0.1.110:30091/sse/sse/messages/?session_id=...`（路径重复）

**表现**：
- 请求发送到错误的端点
- 返回 404 或错误的响应

#### 场景3：响应解析阶段错误

**问题**：
- 系统在 `requestToolsViaSSE` 或 `parseSSEStream` 中
- 错误地解析了 endpoint 事件的响应
- 将 endpoint 事件当作工具列表响应

**表现**：
- 错误信息：`Response body: event: endpoint data: /sse/messages/?`

## 解决方案分析（不修改代码）

### 方案1：增加 SSE 读取超时

**当前配置**：
```
SSE 读取超时: 300 秒
```

**建议配置**：
```
SSE 读取超时: 600 秒（10分钟）
```

**原因**：
- 确保有足够时间等待完整的 endpoint 事件
- 避免超时导致响应解析错误

### 方案2：调整配置结构

**选项A：使用完整 SSE 端点**
```
服务终点 URL: http://11.0.1.110:30091/sse
工具端点: （留空）
```

**选项B：使用根路径 + 工具端点（当前方式）**
```
服务终点 URL: http://11.0.1.110:30091
工具端点: /sse
```

**分析**：
- 两种方式都应该可以工作
- 但可能需要调整超时时间

### 方案3：检查认证头

**当前配置**：
```
Authorization: 1234
```

**建议**：
- 保持这个配置（符合官方文档要求）
- 如果不行，可以尝试删除认证头（但可能无法授权）

## 问题诊断步骤

### 1. 检查日志

查看系统日志，确认：
- 是否成功连接到 SSE 端点
- 是否收到完整的 endpoint 事件
- 构建的完整 URL 是什么
- 发送的请求是什么
- 收到的响应是什么

### 2. 手动测试

使用 curl 测试完整流程：

```bash
# 步骤1：连接 SSE 端点
curl -N http://11.0.1.110:30091/sse -H "Accept: text/event-stream"

# 步骤2：获取 endpoint 路径（从响应中提取）
# 假设得到：/sse/messages/?session_id=abc123

# 步骤3：发送 initialize 请求
curl -X POST "http://11.0.1.110:30091/sse/messages/?session_id=abc123" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"jsonrpc":"2.0","id":"initialize","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}'

# 步骤4：发送 tools/list 请求
curl -X POST "http://11.0.1.110:30091/sse/messages/?session_id=abc123" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"jsonrpc":"2.0","id":"tools-list","method":"tools/list","params":{}}'
```

### 3. 验证响应格式

确认 tools/list 请求的响应格式：
- 是否是 SSE 格式（`event: message`, `data: {...}`）
- 是否包含工具列表
- 工具列表的格式是什么

## 可能的原因总结

### 最可能的原因

1. **SSE 读取超时不够**（60% 可能性）
   - 300 秒可能不够等待完整的 endpoint 事件
   - 建议增加到 600 秒

2. **endpoint 事件解析不完整**（30% 可能性）
   - session_id 可能被截断
   - 导致构建的 URL 不正确

3. **响应解析阶段错误**（10% 可能性）
   - 系统在错误的阶段解析了响应
   - 将 endpoint 事件当作工具列表响应

### 建议的优化

1. **增加 SSE 读取超时**：300 → 600 秒
2. **保持当前配置结构**：BaseURL + ToolsEndpoint
3. **保持认证头**：`Authorization: 1234`（符合官方要求）

## 总结

### 核心问题

系统在工具发现时，收到了 `event: endpoint` 事件，但将其当作工具列表响应来解析，导致错误。

### 可能的原因

1. **超时时间不够**：SSE 读取超时 300 秒可能不够
2. **endpoint 事件不完整**：session_id 可能被截断
3. **响应解析错误**：系统在错误的阶段解析了响应

### 建议

1. **增加 SSE 读取超时**到 600 秒
2. **保持当前配置**（BaseURL + ToolsEndpoint）
3. **保持认证头**（符合官方要求）
4. **查看详细日志**，确认具体问题

### 关键点

- ✅ 配置结构是正确的（BaseURL + ToolsEndpoint）
- ✅ 认证头配置符合官方要求
- ⚠️ SSE 读取超时可能需要增加
- ⚠️ 需要查看详细日志确认具体问题

