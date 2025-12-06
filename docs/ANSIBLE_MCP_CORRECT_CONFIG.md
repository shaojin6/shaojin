# Ansible MCP Server 正确配置指南

## curl 测试结果分析

### ✅ 测试结果

```bash
curl -v http://11.0.1.110:30091/sse

< HTTP/1.1 200 OK
< content-type: text/event-stream; charset=utf-8

event: endpoint
data: /sse/messages/?session_id=5765fc1ba7164b3e9cdbb5bf9c9e3ae7
```

### 关键发现

1. ✅ **SSE 端点可访问**：返回 200 OK
2. ✅ **Content-Type 正确**：`text/event-stream`
3. ✅ **返回了 endpoint 事件**：`event: endpoint`
4. ✅ **端点路径**：`/sse/messages/?session_id=...`

### 协议类型确认

这是 **Dify SSE 模式**！系统需要：
1. GET `/sse` → 收到 `event: endpoint` 事件
2. 获取端点路径：`/sse/messages/?session_id=...`
3. 构建完整 URL：`http://11.0.1.110:30091/sse/messages/?session_id=...`
4. POST JSON-RPC 请求到该端点

## 正确配置

### 配置参数

```
服务终点 URL: http://11.0.1.110:30091/sse
名称: ansible-mcp-server
服务器标识符: ansible-mcp-server
协议类型: HTTP/REST API
超时时间: 60 秒
SSE 读取超时时间: 600 秒（10分钟，足够等待 endpoint 事件）
请求头: （删除 Authorization: 1，除非确实需要）
工具端点: （留空，让系统自动发现）
```

### 关键配置说明

#### 1. 服务终点 URL

**配置**：
```
http://11.0.1.110:30091/sse
```

**原因**：
- ✅ 使用 NodePort 30091（外部可访问）
- ✅ 指向 `/sse` 端点（SSE 端点）
- ✅ 系统会 GET 这个端点，等待 `event: endpoint` 事件

#### 2. SSE 读取超时

**配置**：
```
600 秒（10分钟）
```

**原因**：
- ✅ 需要足够时间等待 `event: endpoint` 事件
- ✅ 600 秒足够处理 SSE 流式连接
- ✅ 避免超时导致工具发现失败

#### 3. 超时时间

**配置**：
```
60 秒
```

**原因**：
- ✅ 对于 HTTP 请求，60 秒足够
- ✅ SSE 读取超时已经设置为 600 秒
- ✅ 避免请求挂起时间过长

#### 4. 工具端点

**配置**：
```
留空（不填写）
```

**原因**：
- ✅ 系统会自动从 SSE 流中获取端点路径
- ✅ 不需要手动指定
- ✅ 让系统使用 Dify SSE 模式自动发现

## 系统工作流程

### 步骤1：连接 SSE 端点

```
GET http://11.0.1.110:30091/sse
Headers: Accept: text/event-stream
```

**响应**：
```
HTTP/1.1 200 OK
Content-Type: text/event-stream

event: endpoint
data: /sse/messages/?session_id=5765fc1ba7164b3e9cdbb5bf9c9e3ae7
```

### 步骤2：解析 endpoint 事件

系统会：
1. 识别 `event: endpoint`
2. 提取 `data: /sse/messages/?session_id=...`
3. 构建完整 URL：`http://11.0.1.110:30091/sse/messages/?session_id=...`

### 步骤3：发送 initialize 请求

```
POST http://11.0.1.110:30091/sse/messages/?session_id=...
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": "initialize",
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "k8s-mcp-web",
      "version": "1.0.0"
    }
  }
}
```

### 步骤4：发送 tools/list 请求

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

### 步骤5：解析工具列表

系统会从 SSE 响应中解析工具列表，应该得到 8 个工具。

## 配置检查清单

修正配置时，确保：

- [x] 服务终点 URL：`http://11.0.1.110:30091/sse`（使用 NodePort）
- [x] 超时时间：`60` 秒
- [x] SSE 读取超时：`600` 秒（10分钟）
- [x] 工具端点：留空（自动发现）
- [x] 请求头：删除 `Authorization: 1`（除非确实需要认证）

## 验证步骤

### 1. 修正配置

在 Web UI 中：
1. 编辑 `ansible-mcp-server` 配置
2. 修改服务终点 URL：`http://11.0.1.110:30091/sse`
3. 调整超时时间：`60` 秒
4. 确保 SSE 读取超时：`600` 秒
5. 删除或修正请求头
6. 保存配置

### 2. 测试连接

1. 在 MCP 配置列表中，点击"测试"按钮
2. 等待测试完成（可能需要几秒钟）
3. 查看测试结果：
   - ✅ 成功：显示"连接正常，已找到 8 个工具"
   - ❌ 失败：查看错误信息

### 3. 刷新工具列表

1. 点击"刷新远程工具"按钮
2. 查看工具数量（应该显示 8 个工具）
3. 点击工具数量，查看工具列表

### 4. 查看日志

检查 ansible-mcp-server 的日志，应该看到：

```
GET /sse HTTP/1.1" 200 OK
POST /sse/messages/?session_id=... HTTP/1.1" 200 OK
```

而不是：
```
POST /sse/sse/messages/...  （路径重复）
GET /sse/mcp/tools 404      （错误的路径）
```

## 预期结果

### 成功的标志

1. ✅ 测试连接成功
2. ✅ 工具列表显示 8 个工具
3. ✅ 日志中不再出现 404 错误
4. ✅ 日志显示正确的端点路径：`/sse/messages/?session_id=...`

### 工具列表

应该显示以下 8 个工具：

1. **list_inventory** - 列出 Inventory 主机组/结构
2. **list_hosts** - 列出主机清单
3. **ping_hosts** - Ping 测试主机连通性
4. **run_ad_hoc** - 执行 Ansible ad-hoc 命令
5. **run_playbook** - 执行 Playbook（SSE 流式输出）
6. **validate_playbook** - 验证 Playbook 语法
7. **generate_playbook** - 生成 Playbook 文件
8. **get_ansible_version** - 获取 Ansible 版本

## 常见问题

### Q1: 为什么 SSE 读取超时需要 600 秒？

**A**: 因为系统需要等待 `event: endpoint` 事件。虽然通常很快，但为了确保稳定性，600 秒（10分钟）足够处理各种情况。

### Q2: 工具端点需要填写吗？

**A**: 不需要。系统会自动从 SSE 流中获取端点路径（`/sse/messages/?session_id=...`）。

### Q3: 如果测试失败怎么办？

**A**: 
1. 检查服务终点 URL 是否正确（端口 30091）
2. 检查 SSE 读取超时是否足够（600 秒）
3. 查看服务日志，确认端点是否可访问
4. 检查网络连接和防火墙规则

## 总结

### 核心配置

```
服务终点 URL: http://11.0.1.110:30091/sse
超时时间: 60 秒
SSE 读取超时: 600 秒
工具端点: 留空
```

### 工作原理

1. 系统 GET `/sse`，收到 `event: endpoint` 事件
2. 提取端点路径：`/sse/messages/?session_id=...`
3. 构建完整 URL：`http://11.0.1.110:30091/sse/messages/?session_id=...`
4. POST JSON-RPC 请求到该端点
5. 获取工具列表（8 个工具）

### 关键点

- ✅ SSE 端点返回了 `event: endpoint` 事件，说明是 Dify SSE 模式
- ✅ 系统会自动处理 endpoint 事件，获取正确的端点路径
- ✅ 只需要配置正确的 BaseURL 和超时时间即可

按照这个配置，工具发现应该可以正常工作！

