# Kubernetes MCP 智能体 + Web UI 综合开发指南

> 本指南整合了纯 MCP Agent（CLI 模式）与 Web UI 可视化方案，帮助你在现有 `k8s-mcp` 项目的基础上构建一个面向 LLM 的 Kubernetes 智能体，并逐步扩展到图形化管理界面。

## 核心特性

- **双模式工具调用**：
  - **Function Calling 模式**（优先）：使用标准的 `tools` 参数和 `tool_calls` 响应，更准确、更可靠
  - **Prompt-based 模式**（后备）：通过系统提示词让 LLM 输出结构化命令，兼容所有模型
- **智能模式选择**：根据模型能力自动选择使用哪种模式，用户无感知
- **多 Provider 支持**：支持 OpenAI、DashScope、Ollama 等多种 LLM Provider
- **远程 MCP 服务**：支持连接和管理多个远程 MCP 服务
- **工具缓存机制**：Redis + MySQL 双层缓存，提升工具发现性能

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
     - Function Calling 模式（优先）
     - Prompt-based 模式（后备）
     - 自动模式选择
   - Function Calling 功能详解
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
┌─────────────────────────────────────────────────────────────┐
│                    Vue3 Web UI                               │
│  ConfigPanel / Chat / Status & Tool Logs / Agent Config    │
└──────────────────────┬──────────────────────────────────────┘
                       │ REST / WebSocket
┌──────────────────────┴──────────────────────────────────────┐
│              Go Web Service (cmd/web)                        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  API Layer (internal/web/api)                        │  │
│  │  - 配置管理 API                                        │  │
│  │  - 对话 API                                            │  │
│  │  - Agent 管理 API                                      │  │
│  └──────────────────┬───────────────────────────────────┘  │
│                     │                                        │
│  ┌──────────────────┴───────────────────────────────────┐  │
│  │  LLM Orchestrator (internal/web/chat)                │  │
│  │                                                      │  │
│  │  ┌──────────────────────────────────────────────┐   │  │
│  │  │  模型能力检测 (internal/web/llm/capabilities)│   │  │
│  │  │  - 检测模型是否支持 Function Calling          │   │  │
│  │  │  - 自动选择策略                                │   │  │
│  │  └──────────────┬───────────────────────────────┘   │  │
│  │                 │                                     │  │
│  │  ┌──────────────┴───────────────┐                   │  │
│  │  │  策略分发                      │                   │  │
│  │  │                               │                   │  │
│  │  │  ┌──────────────┐  ┌──────────┴──────────┐      │  │
│  │  │  │ Function     │  │  Prompt-based        │      │  │
│  │  │  │ Calling     │  │  Mode                │      │  │
│  │  │  │ (优先)       │  │  (后备)               │      │  │
│  │  │  │             │  │                      │      │  │
│  │  │  │ - tools     │  │ - System Prompt      │      │  │
│  │  │  │ - tool_calls│  │ - JSON 解析          │      │  │
│  │  │  │ - 标准格式   │  │ - 文本解析           │      │  │
│  │  │  └────────────┘  └──────────────────────┘      │  │
│  │  └───────────────────────────────────────────────┘  │  │
│  └──────────────────┬───────────────────────────────────┘  │
│                     │                                        │
│  ┌──────────────────┴───────────────────────────────────┐  │
│  │  LLM Client (internal/web/llm)                     │  │
│  │  - Chat() - 基础对话                                 │  │
│  │  - ChatWithTools() - Function Calling 支持          │  │
│  │  - 多 Provider: OpenAI / DashScope / Ollama 等      │  │
│  └──────────────────┬───────────────────────────────────┘  │
│                     │                                        │
│  ┌──────────────────┴───────────────────────────────────┐  │
│  │  MCP Client (internal/web/mcpclient)                │  │
│  │  - ToolManager - 工具管理                            │  │
│  │  - 工具缓存 (Redis + MySQL)                          │  │
│  │  - 远程 MCP 服务支持                                  │  │
│  └──────────────────┬───────────────────────────────────┘  │
└─────────────────────┼──────────────────────────────────────┘
                      │ Go 内部调用
┌─────────────────────┴──────────────────────────────────────┐
│              MCP Server (pkg/mcp)                             │
│  - 工具注册/调用                                               │
│  - JSON-RPC 协议支持                                          │
└─────────────────────┬──────────────────────────────────────────┘
                      │ client-go / HTTP
┌─────────────────────┴──────────────────────────────────────────┐
│  Kubernetes Cluster / Remote MCP Services                      │
└─────────────────────────────────────────────────────────────────┘
```

**架构说明：**

- **前端**：Vue3 + Vite，REST + WebSocket，提供配置管理、对话界面、Agent 管理等。
- **后端**：`cmd/web`，负责配置存储、LLM 调度、MCP 工具协调。
- **LLM Orchestrator**：核心协调器，支持两种工具调用模式：
  - **Function Calling 模式**（优先）：使用标准的 `tools` 参数和 `tool_calls` 响应，更准确、更可靠。
  - **Prompt-based 模式**（后备）：通过系统提示词让 LLM 输出结构化命令，兼容不支持 Function Calling 的模型。
- **模型能力检测**：自动检测模型是否支持 Function Calling，根据模型能力自动选择策略，用户无感知。
- **MCP Server**：继续复用现有实现，可在同进程中运行，也支持远程 MCP 服务。

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
| `internal/web/store` | 持久化（MySQL/Redis），保存配置、会话、日志。 |
| `internal/web/chat` | Orchestrator：协调 LLM 与 MCP 工具，支持两种调用模式。 |
| `internal/web/llm` | LLM 客户端层，支持多 Provider 和 Function Calling。 |
| `internal/web/llm/capabilities.go` | 模型能力检测，自动判断是否支持 Function Calling。 |
| `internal/web/mcpclient` | MCP Server 封装，支持工具缓存、调用重试、远程 MCP 服务。 |
| `internal/web/worker` | 异步任务处理，支持工具刷新等后台任务。 |

#### 数据存储

- **MySQL**：持久化存储
  - 配置信息：K8s 配置、LLM 配置、Agent 配置、Remote MCP 配置
  - 会话数据：对话历史、消息记录
  - 工具元数据：工具列表、工具调用记录
- **Redis**：缓存层
  - 工具列表缓存：加速工具发现
  - 会话缓存：提升读取性能
  - 临时数据：Worker 任务状态等
- **安全**：敏感信息（如 API Key、kubeconfig）支持加密存储

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

系统支持两种工具调用模式，根据模型能力自动选择，用户无感知：

#### 2.4.1 Function Calling 模式（优先）

**适用场景**：模型支持 Function Calling（如 GPT-4、Qwen-Max、Qwen-Plus 等）

**工作原理**：
1. **工具转换**：将 MCP Tool 转换为标准的 Function Calling 格式（`tools` 参数）
2. **LLM 调用**：使用 `ChatWithTools()` 方法，传递 `tools` 参数给 LLM
3. **响应解析**：直接从 LLM 响应中提取 `tool_calls`（结构化数据，无需文本解析）
4. **工具执行**：根据 `tool_calls` 调用对应的 MCP 工具
5. **结果反馈**：将工具执行结果作为 `tool` 角色的消息反馈给 LLM
6. **循环处理**：重复步骤 2-5，直到 LLM 返回最终答案（无 `tool_calls`）

**优势**：
- ✅ **更准确**：结构化数据，避免文本解析错误
- ✅ **更可靠**：标准协议，减少格式错误
- ✅ **更高效**：直接提取工具调用，无需复杂的文本解析
- ✅ **支持多工具调用**：一次请求可以调用多个工具（如果模型支持）

**示例流程**：
```
用户: "列出 default 命名空间的 Pod"
  ↓
Orchestrator: 检测到模型支持 Function Calling
  ↓
LLM Request: {
  "messages": [...],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "list_pods",
        "description": "列出 Pod",
        "parameters": {...}
      }
    }
  ]
}
  ↓
LLM Response: {
  "content": "",
  "tool_calls": [
    {
      "id": "call_123",
      "type": "function",
      "function": {
        "name": "list_pods",
        "arguments": "{\"namespace\":\"default\"}"
      }
    }
  ]
}
  ↓
Orchestrator: 执行工具 list_pods(namespace="default")
  ↓
Tool Result: {"pods": [...]}
  ↓
LLM Request: {
  "messages": [
    ...,
    {"role": "assistant", "tool_calls": [...]},
    {"role": "tool", "tool_call_id": "call_123", "content": "{\"pods\": [...]}"}
  ]
}
  ↓
LLM Response: {
  "content": "default 命名空间共有 3 个 Pod：...",
  "tool_calls": null
}
  ↓
返回最终答案给用户
```

#### 2.4.2 Prompt-based 模式（后备）

**适用场景**：模型不支持 Function Calling（如部分 Ollama 模型、旧版模型等）

**工作原理**：
1. **系统提示词**：构建包含工具列表、参数说明、调用规范的 System Prompt
2. **LLM 调用**：使用 `Chat()` 方法，传递包含 System Prompt 的消息列表
3. **响应解析**：从 LLM 的文本响应中解析结构化命令（JSON 格式）
4. **工具执行**：根据解析结果调用对应的 MCP 工具
5. **结果反馈**：将工具执行结果作为用户消息反馈给 LLM
6. **循环处理**：重复步骤 2-5，直到 LLM 返回最终答案（action=respond）

**输出格式约定**：
```json
{
  "action": "call_tool" | "respond",
  "tool": "list_pods",
  "arguments": {"namespace": "default"},
  "thought": "需要获取 pod 列表",
  "reply": "..."
}
```

**优势**：
- ✅ **兼容性好**：适用于所有 LLM 模型
- ✅ **灵活性强**：可以通过提示词工程优化效果

**劣势**：
- ⚠️ **解析风险**：需要从文本中解析 JSON，可能出错
- ⚠️ **格式要求**：依赖 LLM 严格遵循输出格式

#### 2.4.3 自动模式选择

**检测机制**：
1. **模型能力映射表**：在 `internal/web/llm/capabilities.go` 中维护模型能力映射
2. **自动检测**：根据 LLM 配置（Provider + Model）查询能力表
3. **策略确定**：
   - 如果模型支持 Function Calling → 使用 `function_call` 策略
   - 如果模型不支持 → 使用 `prompt_based` 策略
4. **策略缓存**：Agent 配置中保存策略，避免重复检测

**支持的模型**（示例）：
- **DashScope/Qwen**：`qwen-max`、`qwen-plus`、`qwen-turbo` 等
- **OpenAI**：`gpt-4`、`gpt-4-turbo`、`gpt-3.5-turbo` 等
- **其他**：根据实际测试结果添加到能力映射表

**代码位置**：
- 模型能力检测：`internal/web/llm/capabilities.go`
- 策略确定：`internal/web/chat/orchestrator.go::getOrDetermineStrategy()`
- Function Calling 处理：`internal/web/chat/orchestrator.go::chatWithFunctionCalling()`
- Prompt-based 处理：`internal/web/chat/orchestrator.go::chatWithPromptBased()`

#### 2.4.4 错误处理

两种模式都支持完善的错误处理：
- **工具调用失败**：将错误信息反馈给 LLM，提示重试或提供替代方案
- **解析失败**：Prompt-based 模式下，如果 JSON 解析失败，提示 LLM 重新格式化输出
- **超时处理**：支持请求超时检测，避免长时间等待
- **权限错误**：识别权限相关错误，提供明确的错误提示

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

### 2.7 Function Calling 功能详解

#### 2.7.1 工具格式转换

MCP Tool 需要转换为 LLM 标准的 Function Calling 格式：

**MCP Tool 格式**：
```go
type Tool struct {
    Name        string
    Description string
    InputSchema InputSchema
}
```

**LLM Tool 格式**：
```json
{
  "type": "function",
  "function": {
    "name": "list_pods",
    "description": "列出 Pod",
    "parameters": {
      "type": "object",
      "properties": {
        "namespace": {
          "type": "string",
          "description": "命名空间"
        }
      },
      "required": ["namespace"]
    }
  }
}
```

转换逻辑位于：`internal/web/chat/orchestrator.go::convertToolsToLLMFormat()`

#### 2.7.2 工具调用消息格式

Function Calling 模式使用标准的消息格式：

**Assistant 消息（包含工具调用）**：
```json
{
  "role": "assistant",
  "content": "",
  "tool_calls": [
    {
      "id": "call_123",
      "type": "function",
      "function": {
        "name": "list_pods",
        "arguments": "{\"namespace\":\"default\"}"
      }
    }
  ]
}
```

**Tool 消息（工具执行结果）**：
```json
{
  "role": "tool",
  "tool_call_id": "call_123",
  "content": "{\"pods\": [...]}"
}
```

#### 2.7.3 多工具调用支持

如果模型支持 `ModelFeatureMultiToolCall`，可以在一次请求中调用多个工具：

```json
{
  "tool_calls": [
    {"id": "call_1", "function": {"name": "list_pods", ...}},
    {"id": "call_2", "function": {"name": "list_services", ...}}
  ]
}
```

系统会并行执行所有工具调用，然后将所有结果反馈给 LLM。

#### 2.7.4 添加新模型支持

在 `internal/web/llm/capabilities.go` 中添加模型能力：

```go
var modelCapabilities = map[string][]types.ModelFeature{
    // 新模型
    "provider:model-name": {
        types.ModelFeatureToolCall,           // 支持单工具调用
        types.ModelFeatureMultiToolCall,      // 支持多工具调用（可选）
    },
}
```

### 2.8 扩展方向

- **多集群**：支持多份 kubeconfig，前端可选择目标集群。
- **多模型**：内置 OpenAI、Azure、DashScope、Ollama 等 Provider。
- **权限隔离**：细分工具权限，限制高危操作。
- **告警/订阅**：结合 Prometheus/Alertmanager，LLM 主动推送。
- **审计导出**：对话/工具调用日志可导出或全文检索。
- **流式响应**：支持 Server-Sent Events (SSE) 实时推送 LLM 响应。
- **工具链优化**：支持工具调用链的优化和缓存。
- **模型能力自动发现**：通过 API 自动检测模型能力，无需手动配置。

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

