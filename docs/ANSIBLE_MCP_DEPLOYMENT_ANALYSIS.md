# Ansible MCP Server K8s 部署方案分析

## 部署方案概述

您通过 Kubernetes 部署 ansible-mcp-server 的方案是**完全可行的**，这是一个很好的架构设计：

### ✅ 优点

1. **服务分离**：ansible-mcp-server 与主系统分离，便于独立升级和维护
2. **K8s 原生**：使用 Kubernetes Deployment + Service，符合云原生架构
3. **NodePort 暴露**：通过 NodePort 30091 暴露服务，便于外部访问
4. **资源控制**：设置了合理的资源限制（CPU 200m-500m，内存 256Mi-512Mi）

### 📋 部署配置分析

您的 YAML 配置：

```yaml
# Deployment
- 命名空间: mcp
- 副本数: 1
- 镜像: ansible-mcp:latest
- 容器端口: 8080
- 节点选择器: control-plane（控制节点）

# Service
- 类型: NodePort
- 端口映射: 30091 -> 8080
- 协议: TCP
```

**配置评估**：
- ✅ 配置合理，符合最佳实践
- ⚠️ 建议：如果有多节点，考虑使用 LoadBalancer 或 Ingress
- ⚠️ 建议：生产环境建议使用固定标签的镜像版本（如 `ansible-mcp:v1.0.0`）

## 工具发现流程分析

### 系统工具发现机制

当您配置 `BaseURL: http://11.0.1.110:30091/sse` 时，系统会按以下顺序尝试：

#### 1. Dify SSE 模式（第一步）
```
GET http://11.0.1.110:30091/sse
Headers: Accept: text/event-stream
期望: 收到 SSE 流，查找 event: endpoint 事件
```

**问题分析**：
- `fastapi-mcp` 的 `mount_sse()` 可能**不遵循 Dify SSE 模式**
- Dify SSE 模式需要先 GET 连接，等待 `event: endpoint` 事件
- `fastapi-mcp` 可能直接使用 StreamableHTTP 模式

#### 2. StreamableHTTP 模式（第二步）
```
POST http://11.0.1.110:30091/sse
Headers: 
  Content-Type: application/json
  Accept: application/json, text/event-stream
Body: {"jsonrpc":"2.0","id":"initialize","method":"initialize",...}
期望: 收到 JSON 或 SSE 响应
```

**这是最可能成功的模式**，因为 `fastapi-mcp` 的 `mount_sse()` 通常支持直接 POST JSON-RPC 请求。

#### 3. 普通 HTTP 模式（回退）
```
尝试多个路径:
- GET http://11.0.1.110:30091/mcp/tools
- GET http://11.0.1.110:30091/api/tools
- GET http://11.0.1.110:30091/tools
- GET http://11.0.1.110:30091/v1/tools
- GET http://11.0.1.110:30091/api/v1/tools
- GET http://11.0.1.110:30091/api/mcp/tools
```

**这些路径都会返回 404**，因为 `fastapi-mcp` 不使用这些路径。

## 问题根源分析

### 错误信息解读

从您提供的错误信息：
```
points ([/mcp/tools /api/tools /tools /v1/tools /api/v1/tools /api/mcp/tools]) 
but all failed. Last error: status 404
```

这说明：
1. ✅ SSE 端点 `/sse` 可以访问（否则不会尝试普通 HTTP 模式）
2. ❌ Dify SSE 模式失败（没有找到 `event: endpoint`）
3. ❌ StreamableHTTP 模式可能也失败（没有正确响应）
4. ❌ 回退到普通 HTTP 模式，所有路径都返回 404

### 可能的原因

#### 原因1：fastapi-mcp 的 SSE 实现方式不同
- `fastapi-mcp` 可能使用**标准 MCP SSE 协议**，而不是 Dify SSE 模式
- 需要直接 POST JSON-RPC 请求，而不是先 GET 等待 endpoint 事件

#### 原因2：SSE 端点需要特定的请求格式
- 可能需要先发送 `initialize` 请求
- 然后才能发送 `tools/list` 请求

#### 原因3：Content-Type 或 Accept 头不匹配
- SSE 端点可能对请求头有特定要求

## 优化方案（不修改代码）

### 方案1：调整 BaseURL（推荐）

**问题**：当前配置 `http://11.0.1.110:30091/sse` 可能不是正确的端点。

**优化**：尝试以下 BaseURL：

1. **尝试 HTTP 端点**（如果 fastapi-mcp 同时挂载了 HTTP）：
   ```
   BaseURL: http://11.0.1.110:30091/mcp
   或
   BaseURL: http://11.0.1.110:30091
   ```

2. **保持 SSE 端点，但调整配置**：
   ```
   BaseURL: http://11.0.1.110:30091/sse
   ToolsEndpoint: /sse  （明确指定工具端点）
   ```

### 方案2：增加超时时间

**问题**：SSE 连接可能需要更长时间建立。

**优化**：
- **SSE 读取超时**：从 300 秒增加到 **600 秒**（10分钟）
- **超时时间**：从 30 秒增加到 **60 秒**

### 方案3：检查 fastapi-mcp 的实际端点

**步骤**：
1. 访问 `http://11.0.1.110:30091/docs` 查看 FastAPI 自动生成的文档
2. 查看有哪些端点可用
3. 确认 `/sse` 端点的实际行为

### 方案4：使用 HTTP 模式（如果可用）

如果 `fastapi-mcp` 的 `mount_http()` 挂载了 HTTP 端点，可以尝试：

```
BaseURL: http://11.0.1.110:30091/mcp
或
BaseURL: http://11.0.1.110:30091
ToolsEndpoint: /mcp/tools
```

## 验证步骤

### 1. 验证服务可访问性

```bash
# 检查服务是否运行
kubectl get pods -n mcp -l app=ansible-mcp-server

# 检查服务端点
kubectl get svc -n mcp ansible-mcp-service

# 测试端口连通性
telnet 11.0.1.110 30091
```

### 2. 测试 SSE 端点

```bash
# 测试 GET 请求
curl -N -X GET "http://11.0.1.110:30091/sse" \
  -H "Accept: text/event-stream" \
  --max-time 10

# 测试 POST 请求（JSON-RPC）
curl -X POST "http://11.0.1.110:30091/sse" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"jsonrpc":"2.0","id":"test","method":"tools/list","params":{}}' \
  --max-time 10
```

### 3. 查看 FastAPI 文档

访问 `http://11.0.1.110:30091/docs`，查看：
- 有哪些端点
- `/sse` 端点的实际路径和参数
- 是否有其他 MCP 相关端点

### 4. 检查 Pod 日志

```bash
kubectl logs -n mcp -l app=ansible-mcp-server --tail=50
```

查看是否有错误信息或请求日志。

## 推荐配置

基于分析，推荐以下配置：

### 配置1：使用 SSE 端点（推荐先试这个）

```json
{
  "name": "ansible-mcp-server",
  "serverId": "ansible-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091/sse",
  "timeout": 60,
  "sseReadTimeout": 600,
  "toolsEndpoint": "",
  "enabled": true
}
```

**关键调整**：
- `timeout`: 30 → **60** 秒
- `sseReadTimeout`: 300 → **600** 秒

### 配置2：尝试 HTTP 端点（如果 SSE 不行）

```json
{
  "name": "ansible-mcp-server",
  "serverId": "ansible-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091",
  "timeout": 60,
  "sseReadTimeout": 300,
  "toolsEndpoint": "/mcp/tools",
  "enabled": true
}
```

## 总结

### ✅ 您的方案是可行的

1. **K8s 部署方式正确**：Deployment + Service + NodePort
2. **服务分离设计合理**：便于独立升级和维护
3. **配置基本正确**：BaseURL 指向 `/sse` 端点

### ⚠️ 需要优化的地方

1. **增加超时时间**：SSE 连接可能需要更长时间
2. **验证端点行为**：确认 `/sse` 端点的实际协议
3. **尝试不同配置**：如果 SSE 不行，尝试 HTTP 端点

### 🎯 下一步行动

1. 先尝试**增加超时时间**（最简单）
2. 如果还不行，**查看 FastAPI 文档**确认端点
3. 根据文档调整 **BaseURL 和 ToolsEndpoint**

**您的方案本身没有问题，主要是需要找到正确的端点配置！**

