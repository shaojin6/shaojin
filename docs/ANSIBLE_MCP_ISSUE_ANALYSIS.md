# Ansible MCP Server 工具发现问题分析

## 问题现象

从日志和错误信息可以看出：

### 1. 配置问题
- **配置的地址**：`http://11.0.1.110:8080/sse`
- **实际应该访问**：`http://11.0.1.110:30091/sse`（通过 NodePort）

### 2. 日志分析

#### 成功的请求
```
GET /sse HTTP/1.1" 200 OK
```
- ✅ SSE 端点可以访问
- ✅ 返回 200 状态码

#### 失败的请求
```
POST /sse/sse/messages/?session_id=d9c86f03f8904a8
GET /sse/mcp/tools HTTP/1.1" 404 Not Found
GET /sse/api/tools HTTP/1.1" 404 Not Found
GET /sse/tools HTTP/1.1" 404 Not Found
GET /sse/v1/tools HTTP/1.1" 404 Not Found
GET /sse/api/v1/tools HTTP/1.1" 404 Not Found
GET /sse/api/mcp/tools HTTP/1.1" 404 Not Found
```

### 3. 问题分析

#### 问题1：路径拼接错误
- `POST /sse/sse/messages/` - **路径重复了 `/sse`**
- 说明 BaseURL 可能已经包含了 `/sse`，系统又追加了 `/sse`

#### 问题2：工具发现路径错误
- 系统尝试了 `/sse/mcp/tools`, `/sse/api/tools` 等路径
- 这些路径在 BaseURL 后面追加，导致变成了 `http://11.0.1.110:8080/sse/mcp/tools`
- 但 `fastapi-mcp` 的工具端点不在这些路径下

#### 问题3：端口配置错误
- 配置中使用的是 `:8080`（容器端口）
- 应该使用 `:30091`（NodePort）

## 根本原因

### 1. BaseURL 配置错误

**当前配置**：
```
BaseURL: http://11.0.1.110:8080/sse
```

**问题**：
- 端口错误：8080 是容器内部端口，外部无法直接访问
- 应该使用 NodePort 30091

**正确配置**：
```
BaseURL: http://11.0.1.110:30091/sse
```

### 2. fastapi-mcp 的端点结构

根据 `fastapi-mcp` 的实现：
- `mount_sse()` 挂载的 SSE 端点：`/sse`
- `mount_http()` 挂载的 HTTP 端点：通常是 `/mcp` 或根路径

**工具发现应该通过**：
- 直接 POST JSON-RPC 请求到 `/sse` 端点
- 而不是在 `/sse` 后面追加 `/mcp/tools` 等路径

### 3. 系统工具发现逻辑

系统在工具发现时的行为：
1. 先尝试 SSE 模式（GET `/sse`，等待 endpoint 事件）
2. 如果失败，尝试 StreamableHTTP 模式（POST `/sse`）
3. 如果还失败，回退到普通 HTTP 模式，在 BaseURL 后面追加常见路径

**问题**：当 BaseURL 是 `/sse` 时，追加路径变成了 `/sse/mcp/tools`，这是错误的。

## 解决方案

### 方案1：修正 BaseURL（必须）

**修改配置**：
```
BaseURL: http://11.0.1.110:30091/sse
```

**原因**：
- 使用正确的 NodePort 端口
- 确保外部可以访问

### 方案2：调整 BaseURL 结构（推荐）

**选项A：使用根路径 + 明确工具端点**
```
BaseURL: http://11.0.1.110:30091
ToolsEndpoint: /sse
```

**选项B：直接使用 SSE 端点（当前方式）**
```
BaseURL: http://11.0.1.110:30091/sse
ToolsEndpoint: （留空，让系统自动发现）
```

### 方案3：增加超时时间

由于 SSE 连接可能需要时间建立：

```
timeout: 60（从 30 增加到 60）
sseReadTimeout: 600（从 300 增加到 600）
```

## 验证步骤

### 1. 修正配置

在 Web UI 中修改 ansible-mcp-server 配置：
- **访问地址**：`http://11.0.1.110:30091/sse`（改为 NodePort）
- **超时时间**：60 秒
- **SSE 读取超时**：600 秒

### 2. 测试连接

1. 点击"测试"按钮
2. 查看是否成功连接
3. 如果成功，应该显示"连接正常，已找到 8 个工具"

### 3. 刷新工具列表

1. 点击"刷新远程工具"按钮
2. 查看工具数量（应该显示 8 个工具）

### 4. 查看日志

检查 ansible-mcp-server 的日志，应该看到：
```
POST /sse HTTP/1.1" 200 OK  （而不是 /sse/sse/messages/）
```

## 预期结果

修正配置后，应该看到：

### 成功的日志
```
POST /sse HTTP/1.1" 200 OK  （JSON-RPC initialize 请求）
POST /sse HTTP/1.1" 200 OK  （JSON-RPC tools/list 请求）
```

### 工具列表
- 应该显示 8 个工具：
  1. list_inventory
  2. list_hosts
  3. ping_hosts
  4. run_ad_hoc
  5. run_playbook
  6. validate_playbook
  7. generate_playbook
  8. get_ansible_version

## 总结

### 核心问题
1. **端口配置错误**：使用了容器端口 8080 而不是 NodePort 30091
2. **路径拼接问题**：BaseURL 包含 `/sse` 时，系统追加路径导致错误

### 解决方案
1. **修正 BaseURL**：`http://11.0.1.110:30091/sse`
2. **增加超时时间**：确保 SSE 连接有足够时间建立
3. **保持 ToolsEndpoint 为空**：让系统自动发现

### 关键点
- ✅ SSE 端点 `/sse` 本身是可以访问的（GET 返回 200）
- ❌ 工具发现失败是因为路径拼接和端口配置错误
- ✅ 修正配置后应该可以正常工作

