# 快速集成指南

## 你需要提供的信息

为了快速开发集成，请提供以下信息：

### 方式一：使用信息收集脚本（推荐）

运行脚本自动收集信息：

```powershell
.\scripts\collect-mcp-info.ps1
```

脚本会引导你输入所有必要信息，并生成配置文件。

### 方式二：手动提供信息

请提供以下信息：

#### 1. 基本信息

```
命名空间: ___________
Service 名称: ___________
端口: ___________
```

#### 2. 访问地址

- **集群内访问**：`http://<service-name>.<namespace>.svc.cluster.local:<port>`
- **外部访问**：`http://<your-domain>:<port>`

#### 3. 认证信息

- 认证方式：Bearer Token / API Key / 无认证
- Token/Key 值：___________

#### 4. API 端点

- 工具列表端点：`/api/tools`（默认）或其他
- 工具调用端点：`/api/tools/call`（默认）或其他

#### 5. 当前 Dify 配置（如果有）

请提供 Dify 中的 MCP 服务配置截图或配置内容。

## 示例

### 示例 1：集群内 HTTP 服务

```
命名空间: dify
Service 名称: k8s-mcp-service
端口: 8080
访问地址: http://k8s-mcp-service.dify.svc.cluster.local:8080
认证方式: Bearer Token
Token: sk-xxxxxxxxxxxxx
工具端点: /api/tools
```

### 示例 2：外部 HTTP 服务

```
命名空间: (不适用)
Service 名称: (不适用)
端口: 9090
访问地址: https://mcp.example.com
认证方式: API Key
API Key: your-api-key-here
工具端点: /api/v1/tools
```

## 提供信息后

提供信息后，我会：

1. ✅ 创建远程 MCP 客户端
2. ✅ 实现工具自动发现
3. ✅ 集成到配置面板
4. ✅ 支持工具统一调用
5. ✅ 更新 Web UI

## 测试

集成完成后，我们可以：

1. 测试连接你的 MCP 服务
2. 验证工具列表获取
3. 测试工具调用
4. 在 Web UI 中验证

## 联系

请将信息通过以下方式提供：

1. 运行 `collect-mcp-info.ps1` 脚本，将生成的 JSON 文件发给我
2. 或直接在对话中提供上述信息
3. 或提供 Dify 配置截图

收到信息后，我会立即开始开发！

