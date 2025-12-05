# MCP 服务集成指南

## 概述

本文档说明如何将部署在 Kubernetes 集群中的 MCP 服务集成到我们的 MCP 智能体平台。

## 需要提供的信息

为了便于开发集成，请提供以下信息：

### 1. 部署信息

- **命名空间**：MCP 服务部署在哪个命名空间？
- **Service 名称**：Kubernetes Service 的名称是什么？
- **端口**：服务暴露的端口号是多少？
- **访问方式**：
  - [ ] ClusterIP（集群内访问）
  - [ ] NodePort（节点端口）
  - [ ] LoadBalancer（负载均衡器）
  - [ ] Ingress（入口）

### 2. MCP 服务协议信息

- **通信协议**：
  - [ ] HTTP/REST API
  - [ ] WebSocket
  - [ ] stdio（标准输入输出）
  - [ ] 其他：___________

- **API 端点**（如果是 HTTP）：
  - 基础 URL：`http://service-name.namespace.svc.cluster.local:port`
  - 或外部访问地址：`https://your-domain.com`

- **认证方式**：
  - [ ] 无认证
  - [ ] Bearer Token
  - [ ] API Key
  - [ ] 其他：___________

### 3. MCP 工具信息

请提供你的 MCP 服务提供的工具列表：

```json
{
  "tools": [
    {
      "name": "tool_name",
      "description": "工具描述",
      "parameters": {
        "param1": "类型和说明"
      }
    }
  ]
}
```

或者提供工具列表的 API 端点（如果有）。

### 4. 当前 Dify 集成方式

请描述当前在 Dify 中是如何集成的：

- Dify 配置截图或配置内容
- 连接方式（HTTP、WebSocket 等）
- 认证信息如何传递

### 5. 测试信息

- **测试环境访问方式**：如何访问你的 MCP 服务进行测试？
- **示例请求**：提供一个可以成功调用的示例请求

## 集成方案

根据你提供的信息，我们可以实现以下集成方式：

### 方案 A：HTTP/REST API 集成（推荐）

如果你的 MCP 服务通过 HTTP 暴露，我们可以：

1. **创建远程 MCP 客户端**
   - 实现 HTTP 客户端连接到你的 MCP 服务
   - 支持工具列表查询
   - 支持工具调用

2. **集成到平台**
   - 在配置面板中添加"远程 MCP 服务"配置
   - 支持多个远程 MCP 服务
   - 工具自动发现和注册

3. **统一管理**
   - 所有工具（本地 + 远程）统一展示
   - LLM 可以调用所有工具

### 方案 B：WebSocket 集成

如果使用 WebSocket：

1. 实现 WebSocket 客户端
2. 建立持久连接
3. 实时工具调用

### 方案 C：Kubernetes Service 直接访问

如果服务在集群内：

1. 通过 Service DNS 名称访问
2. 使用集群内网络，无需外部暴露
3. 支持 ServiceAccount 认证

## 快速开始

### 步骤 1：填写信息

请按照上面的"需要提供的信息"部分，提供你的 MCP 服务信息。

### 步骤 2：测试连接

我们可以先创建一个测试脚本，验证连接是否正常。

### 步骤 3：开发集成

根据你提供的信息，我会开发相应的集成代码。

### 步骤 4：部署和测试

集成完成后，在你的环境中测试。

## 示例配置

### 示例 1：HTTP API 集成

```yaml
# 配置示例
remote_mcp_services:
  - name: "k8s-cluster-mcp"
    type: "http"
    base_url: "http://k8s-mcp-service.dify.svc.cluster.local:8080"
    auth:
      type: "bearer_token"
      token: "your-token"
    tools_auto_discover: true
```

### 示例 2：WebSocket 集成

```yaml
remote_mcp_services:
  - name: "k8s-cluster-mcp"
    type: "websocket"
    url: "ws://k8s-mcp-service.dify.svc.cluster.local:8080/ws"
    auth:
      type: "api_key"
      api_key: "your-api-key"
```

## 下一步

请提供以下信息，我会开始开发：

1. ✅ 部署信息（命名空间、Service 名称、端口）
2. ✅ 通信协议（HTTP/WebSocket/stdio）
3. ✅ API 端点或访问地址
4. ✅ 认证方式
5. ✅ 工具列表或工具发现方式
6. ✅ 当前 Dify 集成配置（如果有）

提供这些信息后，我会立即开始开发集成功能！

