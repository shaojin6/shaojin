package types

// K8sConfig 定义 Kubernetes 连接方式
type K8sConfig struct {
	ID         string `json:"id"`                 // 唯一标识符
	Name       string `json:"name"`               // 显示名称
	Mode       string `json:"mode"`               // kubeconfig / manual
	Content    string `json:"content"`            // base64 kubeconfig
	Namespace  string `json:"namespace"`          // 默认命名空间
	Server     string `json:"server"`             // 手动模式 API server
	Token      string `json:"token"`              // 手动模式 Token
	Username   string `json:"username,omitempty"` // 手动模式用户名
	Password   string `json:"password,omitempty"` // 手动模式密码
	Insecure   bool   `json:"insecure,omitempty"` // 跳过TLS验证
	CAFile     string `json:"caFile,omitempty"`   // CA文件路径
	CAData     string `json:"caData,omitempty"`   // 可选 CA
	Enabled    bool   `json:"enabled"`            // 是否启用
	LastUpdate int64  `json:"lastUpdate"`         // unix 秒
}

// LLMConfig 定义本地 LLM 连接信息
type LLMConfig struct {
	ID         string `json:"id"`       // 唯一标识符
	Name       string `json:"name"`     // 显示名称
	Provider   string `json:"provider"` // dashscope, openai, ollama
	BaseURL    string `json:"baseUrl"`
	Model      string `json:"model"`
	APIKey     string `json:"apiKey,omitempty"`
	Enabled    bool   `json:"enabled"`   // 是否启用（默认启用）
	IsDefault  bool   `json:"isDefault"` // 是否为默认LLM
	LastUpdate int64  `json:"lastUpdate"`
}

// ConfigResponse 用于 GET /api/config（已在下方重新定义）

// StatusResponse 用于健康检查
type StatusResponse struct {
	Status string `json:"status"`
}

// TestResponse 用于连通性测试
type TestResponse struct {
	Status  string `json:"status"` // ok / failed
	Message string `json:"message"`
}

// ToolCallRequest 工具调用请求
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Message   string `json:"message"`
	AgentID   string `json:"agentId,omitempty"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	SessionID string     `json:"sessionId"`
	AgentID   string     `json:"agentId,omitempty"`
	Reply     string     `json:"reply"`
	Steps     []ChatStep `json:"steps,omitempty"`
}

// ChatStep 对话步骤
type ChatStep struct {
	Type      string                 `json:"type"` // llm / tool
	Text      string                 `json:"text,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
}

// RemoteMCPConfig 远程 MCP 服务配置
type RemoteMCPConfig struct {
	Name            string            `json:"name"`                    // 显示名称
	ServerID        string            `json:"serverId"`                // 服务器唯一标识符（小写字母、数字、下划线、连字符，最多24字符）
	Type            string            `json:"type"`                    // http, websocket, stdio
	BaseURL         string            `json:"baseUrl"`                 // 服务终点 URL
	Icon            string            `json:"icon,omitempty"`          // 图标（可选）
	Timeout         int               `json:"timeout"`                 // 超时时间（秒），默认30
	SSEReadTimeout  int               `json:"sseReadTimeout"`          // SSE 读取超时时间（秒），默认300
	Headers         map[string]string `json:"headers,omitempty"`       // HTTP 请求头
	ToolsEndpoint   string            `json:"toolsEndpoint,omitempty"` // 工具端点（可选）
	Enabled         bool              `json:"enabled"`
	LastUpdate      int64             `json:"lastUpdate"`
	Tools           []Tool            `json:"tools,omitempty"`           // 缓存的工具列表
	ToolsLastUpdate int64             `json:"toolsLastUpdate,omitempty"` // 工具列表最后更新时间
}

// AgentConfig 智能体配置
type AgentConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	MCPServerID  string `json:"mcpServerId"`
	LLMID        string `json:"llmId,omitempty"`
	SystemPrompt string `json:"systemPrompt,omitempty"` // 自定义系统提示词（可选）
	Enabled      bool   `json:"enabled"`
	IsDefault    bool   `json:"isDefault"`
	CreatedAt    int64  `json:"createdAt,omitempty"`
	UpdatedAt    int64  `json:"updatedAt,omitempty"`
}

// Tool 工具定义（简化版，用于持久化）
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ConfigResponse 用于 GET /api/config
type ConfigResponse struct {
	K8s        []K8sConfig       `json:"k8s,omitempty"` // 多个K8s集群配置
	LLM        []LLMConfig       `json:"llm,omitempty"` // 多个LLM配置
	RemoteMCPs []RemoteMCPConfig `json:"remoteMcps,omitempty"`
	Agents     []AgentConfig     `json:"agents,omitempty"`
}
