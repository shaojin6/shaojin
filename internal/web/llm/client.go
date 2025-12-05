package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/types"
)

// Client LLM 客户端接口
type Client interface {
	Chat(messages []Message) (string, error)
	TestConnection() error
}

// Message 对话消息
type Message struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// NewClient 根据配置创建 LLM 客户端
func NewClient(config *types.LLMConfig) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("LLM config is nil")
	}

	switch config.Provider {
	case "dashscope", "qwen", "tongyi", "bailian", "sfm", "modelstudio":
		// 百炼平台（Model Studio）使用与 DashScope 相同的 API
		return NewDashScopeClient(config), nil
	case "openai":
		return NewOpenAIClient(config), nil
	case "ollama":
		return NewOllamaClient(config), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}

// DashScopeClient 通义千问客户端
type DashScopeClient struct {
	config *types.LLMConfig
	client *http.Client
}

// NewDashScopeClient 创建通义千问客户端（支持 DashScope 和百炼平台）
func NewDashScopeClient(config *types.LLMConfig) *DashScopeClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		// 默认使用 DashScope API（百炼平台也使用相同的 API 端点）
		baseURL = "https://dashscope.aliyuncs.com/api/v1"
	}

	return &DashScopeClient{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Minute, // 设置为10分钟，因为 qwen3-max 等大模型可能需要更长时间
		},
	}
}

// DashScopeRequest 通义千问 API 请求
// 百炼平台和 DashScope 使用相同的 API 格式
type DashScopeRequest struct {
	Model      string                 `json:"model"`
	Input      InputData              `json:"input"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// InputData 输入数据
type InputData struct {
	Messages []Message `json:"messages"`
}

// DashScopeResponse 通义千问 API 响应
type DashScopeResponse struct {
	Output struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Chat 发送对话请求（支持 DashScope 和百炼平台）
func (c *DashScopeClient) Chat(messages []Message) (string, error) {
	// 验证配置
	if c.config.APIKey == "" {
		return "", fmt.Errorf("API Key is required but not configured")
	}

	baseURL := c.config.BaseURL
	if baseURL == "" {
		// 百炼平台和 DashScope 使用相同的 API 端点
		baseURL = "https://dashscope.aliyuncs.com/api/v1"
	}
	
	// 检查 BaseURL 是否已经包含完整路径
	// 如果 BaseURL 已经包含 "/services/aigc/text-generation/generation"，直接使用
	// 否则拼接路径
	var url string
	if strings.Contains(baseURL, "/services/aigc/text-generation/generation") {
		url = baseURL
	} else {
		// 确保 baseURL 不以 / 结尾
		baseURL = strings.TrimSuffix(baseURL, "/")
		url = fmt.Sprintf("%s/services/aigc/text-generation/generation", baseURL)
	}

	model := c.config.Model
	if model == "" {
		model = "qwen-plus"
	}

	log.Printf("[DashScopeClient] ========== LLM API Call ==========")
	log.Printf("[DashScopeClient] URL=%s", url)
	log.Printf("[DashScopeClient] Model=%s", model)
	log.Printf("[DashScopeClient] Messages Count=%d", len(messages))
	
	// 显示消息摘要
	for i, msg := range messages {
		contentPreview := msg.Content
		if len(contentPreview) > 150 {
			contentPreview = contentPreview[:150] + "..."
		}
		log.Printf("[DashScopeClient] Message[%d]: Role=%s, Length=%d, Preview=%s", 
			i, msg.Role, len(msg.Content), contentPreview)
	}

	reqBody := DashScopeRequest{
		Model: model,
		Input: InputData{
			Messages: messages,
		},
		Parameters: map[string]interface{}{
			"result_format": "message",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	req.Header.Set("X-DashScope-SSE", "disable")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var dashResp DashScopeResponse
	if err := json.Unmarshal(body, &dashResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if dashResp.Code != "" && dashResp.Code != "Success" {
		log.Printf("[DashScopeClient] ERROR: API error code=%s, message=%s", dashResp.Code, dashResp.Message)
		return "", fmt.Errorf("API error: %s - %s", dashResp.Code, dashResp.Message)
	}

	if len(dashResp.Output.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	responseContent := dashResp.Output.Choices[0].Message.Content
	responsePreview := responseContent
	if len(responsePreview) > 300 {
		responsePreview = responsePreview[:300] + "..."
	}
	log.Printf("[DashScopeClient] ✓ LLM API Response received: Length=%d, Preview=%s", len(responseContent), responsePreview)
	log.Printf("[DashScopeClient] ========== End LLM API Call ==========")
	
	return responseContent, nil
}

// TestConnection 测试连接
func (c *DashScopeClient) TestConnection() error {
	testMessages := []Message{
		{Role: "user", Content: "你好"},
	}
	_, err := c.Chat(testMessages)
	return err
}

// OpenAIClient OpenAI 兼容客户端
type OpenAIClient struct {
	config *types.LLMConfig
	client *http.Client
}

// NewOpenAIClient 创建 OpenAI 客户端
func NewOpenAIClient(config *types.LLMConfig) *OpenAIClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIClient{
		config: config,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// OpenAIRequest OpenAI API 请求
type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// OpenAIResponse OpenAI API 响应
type OpenAIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 发送对话请求
func (c *OpenAIClient) Chat(messages []Message) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", c.config.BaseURL)

	model := c.config.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	reqBody := OpenAIRequest{
		Model:    model,
		Messages: messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if openaiResp.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return openaiResp.Choices[0].Message.Content, nil
}

// TestConnection 测试连接
func (c *OpenAIClient) TestConnection() error {
	testMessages := []Message{
		{Role: "user", Content: "Hello"},
	}
	_, err := c.Chat(testMessages)
	return err
}

// OllamaClient Ollama 客户端
type OllamaClient struct {
	config *types.LLMConfig
	client *http.Client
}

// NewOllamaClient 创建 Ollama 客户端
func NewOllamaClient(config *types.LLMConfig) *OllamaClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaClient{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// OllamaRequest Ollama API 请求
type OllamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// OllamaResponse Ollama API 响应
type OllamaResponse struct {
	Message Message `json:"message"`
	Error   string  `json:"error"`
}

// Chat 发送对话请求
func (c *OllamaClient) Chat(messages []Message) (string, error) {
	url := fmt.Sprintf("%s/api/chat", c.config.BaseURL)

	model := c.config.Model
	if model == "" {
		model = "llama2"
	}

	reqBody := OllamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if ollamaResp.Error != "" {
		return "", fmt.Errorf("API error: %s", ollamaResp.Error)
	}

	return ollamaResp.Message.Content, nil
}

// TestConnection 测试连接
func (c *OllamaClient) TestConnection() error {
	testMessages := []Message{
		{Role: "user", Content: "Hello"},
	}
	_, err := c.Chat(testMessages)
	return err
}
