# Kubernetes MCP 智能体 + Web UI 综合开发指南

> 本指南整合了纯 MCP Agent（CLI 模式）与 Web UI 可视化方案，帮助你在现有 `k8s-mcp` 项目的基础上构建一个面向 LLM 的 Kubernetes 智能体，并逐步扩展到图形化管理界面。

---

## 目录

1. MCP Agent（CLI）基础能力
   - 架构概览
   - 模块说明
   - 工具扩展示例
   - 安全 / 部署建议
2. Web UI + LLM 可视化方案
   - 目标与总体架构
   - 模块划分（前端 / 后端 / 数据存储）
   - API 设计
   - LLM Orchestrator 策略
   - 开发流程
   - 扩展方向
3. CLI 与 Web UI 的协同注意点

---

## 1. MCP Agent（CLI 模式）

### 1.1 架构概览

```
LLM / MCP Client
      │ JSON-RPC (stdio)
      ▼
mcp.Server (pkg/mcp)
      │ 调度工具
      ▼
Handlers (internal/server/handlers.go)
      │ 调用
      ▼
Kubernetes Toolset (internal/k8s)
      │ client-go
      ▼
Kubernetes API Server
```

- 入口：`cmd/main.go`，负责初始化 K8s 客户端、创建 MCP Server、注册工具、处理 stdin/stdout。
- 协议：JSON-RPC 2.0 + MCP 规范，适配 Claude Desktop 等 MCP 客户端。

### 1.2 核心模块

| 模块 | 路径 | 说明 |
|------|------|------|
| 协议定义 | `pkg/mcp/types.go` | MCP Request/Response/Error/Tool 等结构体。 |
| Server | `pkg/mcp/server.go` | 工具注册、请求调度、流式处理。 |
| Kubernetes 客户端 | `internal/k8s/client.go` | 支持 in-cluster 与 kubeconfig。 |
| 工具实现 | `internal/k8s/tools.go` | `list_pods`、`get_deployment_status`、`restart_pod` 等。 |
| 工具注册 | `internal/server/handlers.go` | 封装参数校验/错误处理，将工具暴露给 MCP。 |

### 1.3 工具扩展示例

1. 在 `internal/k8s/tools.go` 中实现业务逻辑，返回结构化 JSON 字符串。
2. 在 `handlers.go` 中注册新工具：
   - 编写描述（Description）与 InputSchema。
   - 校验参数、调用工具函数、构造 `ToolsCallResult`。
3. 更新 RBAC（`deploy/rbac.yaml`）以匹配新资源权限。

### 1.4 安全与部署

- **RBAC 最小权限**：仅授予 `get/list/delete` 等必要操作，尽量使用 Role + RoleBinding 限制命名空间。
- **命名空间隔离**：生产环境不要使用 ClusterRole + `*` 权限。
- **部署方式**：
  - 本地：直接运行二进制，使用 kubeconfig。
  - 集群：通过 Deployment + ServiceAccount + ConfigMap（存配置）。
- **日志 & 审计**：建议引入 `klog` / `zap`，并与 Prometheus/ELK 集成。

---

## 2. Web UI + LLM 可视化方案

> 目标：在 CLI 能力之上，提供图形化配置界面与内置 LLM 调度器，实现“对话式运维 + 自动工具调用”体验。

### 2.1 总体架构

```
┌──────────────────────────┐
│      Vue3 Web UI         │
│  ConfigPanel / Chat /    │
│  Status & Tool Logs      │
└──────────┬──────────────┘
           │ REST / WS
┌──────────┴──────────────┐
│   Go Web Service         │
│  - 配置管理               │
│  - LLM Orchestrator       │
│  - MCP 工具协调           │
└──────────┬──────────────┘
           │ Go 内部调用
┌──────────┴──────────────┐
│   MCP Server (pkg/mcp)   │
│  - 工具注册/调用         │
└──────────┬──────────────┘
           │ client-go
┌──────────┴──────────────┐
│ Kubernetes Cluster (VM) │
└──────────────────────────┘
```

- 前端：Vue3 + Vite，REST + WebSocket。
- 后端：新增 `cmd/web`，负责配置存储、LLM 调度、调用 MCP 工具。
- MCP Server：继续复用现有实现，可在同进程中运行。

### 2.2 模块划分

#### 前端（Vue3）

| 模块 | 功能 |
|------|------|
| ConfigPanel | 上传/粘贴 kubeconfig、配置命名空间、填写 LLM 参数。 |
| StatusCard | 显示 K8s / LLM / MCP 状态。 |
| ChatWindow | 对话窗口，展示消息、工具调用过程。 |
| ToolLogs | 最近工具调用记录。 |
| SessionSidebar | 切换/管理对话（可选）。 |

技术建议：Vue3 + Vite、Pinia、Element Plus/Naive UI、Axios、WebSocket/EventSource。

#### 后端（Go）

| 模块 | 说明 |
|------|------|
| `internal/web/api` | HTTP 层（Gin/Fiber/Chi），提供 REST & WebSocket。 |
| `internal/web/config` | 处理 kubeconfig/LLM 配置，支持加密存储。 |
| `internal/web/store` | 持久化（SQLite/BoltDB/JSON），保存配置、会话、日志。 |
| `internal/web/chat` | Orchestrator：协调 LLM 与 MCP 工具。 |
| `internal/web/mcpclient` | MCP Server 封装，支持工具缓存、调用重试。 |

#### 数据存储

- `configs.db`：K8s 与 LLM 配置（AES-GCM 加密）。
- `sessions.db`：会话内容、工具调用结果。
- 也可采用 BoltDB/Badger 等内嵌 KV 库。

### 2.3 API 设计

#### 配置管理

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/config/k8s` | 保存 K8s 连接（kubeconfig 上传 / 手动模式）。 |
| POST | `/api/config/llm` | 保存 LLM 配置（provider/baseUrl/model/apiKey）。 |
| GET  | `/api/config`     | 获取当前配置（敏感信息脱敏）。 |
| POST | `/api/test-k8s`   | 校验 K8s 是否可连通。 |
| POST | `/api/test-llm`   | 校验 LLM 接口可用性。 |

示例：保存 kubeconfig
```json
POST /api/config/k8s
{
  "mode": "kubeconfig",
  "content": "<base64 kubeconfig>",
  "namespace": "default"
}
```

示例：保存 LLM 配置
```json
POST /api/config/llm
{
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "model": "qwen2:7b",
  "apiKey": ""
}
```

#### 对话 & 工具

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/chat`        | 发送用户问题，由后端调度 LLM + 工具。 |
| GET  | `/api/history`     | 拉取某个 session 的对话记录。 |
| GET  | `/api/tools`       | MCP 工具列表（元数据）。 |
| WS   | `/api/chat/stream` | （可选）实时推送生成/工具执行步骤。 |

`POST /api/chat` 响应示例：
```json
{
  "sessionId": "abc123",
  "reply": "default 命名空间共有 3 个 Pod",
  "steps": [
    {"type": "llm", "text": "需要列出 default 的 Pod"},
    {
      "type": "tool",
      "tool": "list_pods",
      "arguments": {"namespace": "default"},
      "result": {"pods": [...]}
    }
  ]
}
```

#### 状态 & 日志

| 方法 | 路径 | 描述 |
|------|------|------|
| GET  | `/api/status`     | 返回 K8s / LLM / MCP 状态（时间戳、版本等）。 |
| GET  | `/api/logs/tools` | 工具调用历史（分页）。 |

### 2.4 LLM Orchestrator 策略

1. **System Prompt**：列出可用工具、参数、调用规范，要求 LLM 在需要外部数据时输出结构化命令。
2. **输出格式约定**：
   ```json
   {
     "action": "call_tool" | "respond",
     "tool": "list_pods",
     "arguments": {"namespace": "default"},
     "thought": "需要获取 pod 列表",
     "reply": "..."
   }
   ```
3. **执行流程**：
   - action=call_tool → 调用 MCP 工具 → 将结果加入上下文 → 再次询问 LLM 输出最终答案。
   - action=respond → 直接返回给用户。
4. **错误处理**：将错误信息（脱敏）反馈给 LLM，提示重试或退出。

### 2.5 配置存储与安全

- **敏感信息加密**：kubeconfig、token、LLM API Key 统一 AES-GCM 加密，密钥通过环境变量/系统安全存储提供。
- **访问控制**：默认仅监听 `localhost`，如需远程访问可增加 Basic Auth / API Token。
- **日志脱敏**：ToolLogs、LLM prompt 中避免泄露证书或密钥。
- **证书校验**：存储 kubeconfig 中的 CA 数据，强制 TLS 校验。

### 2.6 开发流程建议

1. **搭建 Web 服务骨架**：新建 `cmd/web/main.go`，引入 Gin/Fiber/Chi。
2. **实现配置 API**：`/api/config/*`、`/api/test-*`，先用内存存储。
3. **封装 MCP Client**：复用 `pkg/mcp`，提供 `ListTools`、`CallTool` 方法。
4. **集成 LLM**：以 Ollama 为例，实现 `/api/chat` → LLM → MCP → 结果的闭环。
5. **开发 Vue 前端**：`npm create vite@latest web-ui -- --template vue-ts`，完成 ConfigPanel + StatusCard + ChatWindow。
6. **持久化 & 安全**：接入 SQLite/BoltDB，添加加密、鉴权。
7. **打包部署**：`npm run build` -> `web-ui/dist`，由 Go 或 Nginx 提供静态资源。

### 2.7 扩展方向

- 多集群：支持多份 kubeconfig，前端可选择目标集群。
- 多模型：内置 OpenAI、Azure、DashScope 等 Provider。
- 权限隔离：细分工具权限，限制高危操作。
- 告警/订阅：结合 Prometheus/Alertmanager，LLM 主动推送。
- 审计导出：对话/工具调用日志可导出或全文检索。

---

## 3. CLI 与 Web UI 协同注意点

| 主题 | CLI / MCP Agent | Web UI 模式 |
|------|-----------------|-------------|
| 通信方式 | JSON-RPC stdio | REST / WebSocket |
| LLM 调度 | 外部 MCP 客户端负责 | 内嵌 Orchestrator |
| 用户群体 | 偏技术人员 (CLI) | 更广用户 (图形界面) |
| 部署 | 可本地可集群 | 建议与 Agent 靠近或同集群 |
| 安全重点 | kubeconfig / RBAC | 再加 Web 鉴权 / 加密存储 |

推荐策略：保持 CLI Agent 的轻量与标准兼容性，同时通过 Web UI 打包更友好的体验。Web 层复用 MCP 工具，不重复造轮子。

---

本指南可以作为从纯 MCP Agent 到 Web UI 智能体的路线图。若需要更细的接口定义、前端组件示例或后端脚手架，可在此基础上继续扩展。祝项目顺利！ ✨

