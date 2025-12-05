package mcpclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// RemoteClient 远程 MCP 服务客户端
type RemoteClient struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
	tools      []mcp.Tool
}

// RemoteMCPConfig 远程 MCP 服务配置
type RemoteMCPConfig struct {
	Name           string            `json:"name"`
	ServerID       string            `json:"serverId"`
	Type           string            `json:"type"` // http, websocket, stdio
	BaseURL        string            `json:"baseUrl"`
	Timeout        int               `json:"timeout"`
	SSEReadTimeout int               `json:"sseReadTimeout"`
	Headers        map[string]string `json:"headers"`
	ToolsEndpoint  string            `json:"toolsEndpoint"`
}

// NewRemoteClient 创建远程 MCP 客户端
// skipDiscovery: 如果为 true，跳过工具发现（用于工具列表已在缓存中的情况）
func NewRemoteClient(config RemoteMCPConfig) (*RemoteClient, error) {
	return NewRemoteClientWithOptions(config, false)
}

// NewRemoteClientWithOptions 创建远程 MCP 客户端（带选项）
func NewRemoteClientWithOptions(config RemoteMCPConfig, skipDiscovery bool) (*RemoteClient, error) {
	// 清理 baseURL（移除末尾空格和斜杠）
	baseURL := strings.TrimSpace(config.BaseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	client := &RemoteClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		headers: config.Headers,
		tools:   []mcp.Tool{}, // 初始化为空列表
	}

	// 如果跳过发现，直接返回（工具列表为空，但可以用于调用工具）
	if skipDiscovery {
		return client, nil
	}

	// 自动发现工具（如果失败，仍然返回客户端，工具列表为空）
	// 先尝试 SSE 流式协议（如果服务返回 text/event-stream）
	if err := client.discoverToolsSSE(config); err == nil {
		// SSE 方式成功
		return client, nil
	}

	// 如果 SSE 失败，尝试普通 HTTP 方式
	if err := client.discoverTools(config.ToolsEndpoint); err != nil {
		// 不立即返回错误，允许客户端创建，但工具列表为空
		// 这样可以在测试时提供更详细的错误信息
		client.tools = []mcp.Tool{}
		// 返回错误以便调用方知道工具发现失败
		return client, fmt.Errorf("failed to discover tools: %w", err)
	}

	return client, nil
}

// discoverToolsSSE 通过 SSE 流发现工具
// 支持两种模式：
// 1. Dify SSE 模式：先 GET 连接 SSE 端点，等待 endpoint 事件，然后 POST 到该端点
// 2. 直接 POST 模式：直接 POST JSON-RPC 请求，期望 SSE 响应
func (c *RemoteClient) discoverToolsSSE(config RemoteMCPConfig) error {
	url := strings.TrimSuffix(config.BaseURL, "/")

	// 先尝试 Dify SSE 模式（先 GET 连接，等待 endpoint 事件）
	if endpointURL, err := c.connectSSEAndGetEndpoint(url, config); err == nil && endpointURL != "" {
		// 成功获取端点，使用该端点发送请求
		return c.discoverToolsViaEndpoint(endpointURL, config)
	}

	// 如果 Dify SSE 模式失败，尝试直接 POST 模式（StreamableHTTP）
	return c.discoverToolsViaStreamableHTTP(url, config)
}

// connectSSEAndGetEndpoint 连接到 SSE 端点并获取实际的端点 URL（Dify SSE 模式）
func (c *RemoteClient) connectSSEAndGetEndpoint(url string, config RemoteMCPConfig) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create SSE connection request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	c.addAuthHeader(req, config.Headers)

	sseTimeout := 300 * time.Second
	if config.SSEReadTimeout > 0 {
		sseTimeout = time.Duration(config.SSEReadTimeout) * time.Second
	}

	client := &http.Client{
		Timeout: sseTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("SSE connection failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	// 检查 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return "", fmt.Errorf("not an SSE endpoint: Content-Type is %s", contentType)
	}

	// 解析 SSE 流，查找 endpoint 事件
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	var currentData strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			currentData.WriteString(data)
		} else if line == "" {
			// 空行表示事件结束
			if currentEvent == "endpoint" && currentData.Len() > 0 {
				endpointPath := strings.TrimSpace(currentData.String())
				// 构建完整 URL
				if strings.HasPrefix(endpointPath, "http://") || strings.HasPrefix(endpointPath, "https://") {
					return endpointPath, nil
				}
				// 相对路径，需要与 baseURL 组合
				if strings.HasPrefix(endpointPath, "/") {
					return url + endpointPath, nil
				}
				return url + "/" + endpointPath, nil
			}
			currentEvent = ""
			currentData.Reset()
		}
	}

	return "", fmt.Errorf("no endpoint event found in SSE stream")
}

// discoverToolsViaEndpoint 通过获取的端点发现工具
func (c *RemoteClient) discoverToolsViaEndpoint(endpointURL string, config RemoteMCPConfig) error {
	// 发送 initialize 请求
	initPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "initialize",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "k8s-mcp-web",
				"version": "1.0.0",
			},
		},
	}

	if err := c.postJSONRPC(endpointURL, initPayload, config); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// 发送 tools/list 请求
	toolsPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "tools-list",
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	tools, err := c.requestToolsViaSSE(endpointURL, toolsPayload, config)
	if err != nil {
		return err
	}

	c.tools = tools
	return nil
}

// discoverToolsViaStreamableHTTP 通过 StreamableHTTP 模式发现工具（直接 POST，支持 JSON 和 SSE 响应）
func (c *RemoteClient) discoverToolsViaStreamableHTTP(url string, config RemoteMCPConfig) error {
	// 发送 initialize 请求
	initPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "initialize",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "k8s-mcp-web",
				"version": "1.0.0",
			},
		},
	}

	// 尝试发送 initialize（可能返回 202 Accepted，这是正常的）
	_ = c.postJSONRPC(url, initPayload, config)

	// 发送 tools/list 请求
	toolsPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "tools-list",
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	tools, err := c.requestToolsViaStreamableHTTP(url, toolsPayload, config)
	if err != nil {
		return err
	}

	c.tools = tools
	return nil
}

// postJSONRPC 发送标准 JSON-RPC 请求（非 SSE）
func (c *RemoteClient) postJSONRPC(url string, payload map[string]interface{}, config RemoteMCPConfig) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.addAuthHeader(req, config.Headers)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// requestToolsViaSSE 通过 SSE 流请求工具列表（用于 Dify SSE 模式）
func (c *RemoteClient) requestToolsViaSSE(url string, payload map[string]interface{}, config RemoteMCPConfig) ([]mcp.Tool, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	c.addAuthHeader(req, config.Headers)

	sseTimeout := 300 * time.Second
	if config.SSEReadTimeout > 0 {
		sseTimeout = time.Duration(config.SSEReadTimeout) * time.Second
	}

	client := &http.Client{
		Timeout: sseTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	return c.parseSSEStream(resp.Body, "message")
}

// requestToolsViaStreamableHTTP 通过 StreamableHTTP 模式请求工具列表（支持 JSON 和 SSE 响应）
func (c *RemoteClient) requestToolsViaStreamableHTTP(url string, payload map[string]interface{}, config RemoteMCPConfig) ([]mcp.Tool, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// StreamableHTTP 支持 JSON 和 SSE 两种响应格式
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	c.addAuthHeader(req, config.Headers)

	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	// 处理 202 Accepted（表示请求已接受，响应将通过 SSE 流返回）
	if resp.StatusCode == http.StatusAccepted {
		// 读取 SSE 流
		sseTimeout := 300 * time.Second
		if config.SSEReadTimeout > 0 {
			sseTimeout = time.Duration(config.SSEReadTimeout) * time.Second
		}
		client := &http.Client{
			Timeout: sseTimeout,
		}
		// 重新发送请求以获取 SSE 流
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		c.addAuthHeader(req, config.Headers)
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get SSE stream: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	// 检查响应类型
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		// SSE 响应
		return c.parseSSEStream(resp.Body, "message")
	} else if strings.Contains(contentType, "application/json") {
		// JSON 响应
		return c.parseJSONResponse(resp.Body)
	}

	// 尝试自动检测
	return c.parseSSEStream(resp.Body, "")
}

// parseSSEStream 解析 SSE 流
// eventType: 要处理的事件类型，空字符串表示处理所有事件
func (c *RemoteClient) parseSSEStream(body io.Reader, eventType string) ([]mcp.Tool, error) {
	scanner := bufio.NewScanner(body)
	var currentEvent string
	var eventData strings.Builder
	var tools []mcp.Tool
	var lastError error

	for scanner.Scan() {
		line := scanner.Text()

		// SSE 格式: event: <type>
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			eventData.Reset()
		} else if strings.HasPrefix(line, "data: ") {
			// SSE 格式: data: {...}（可能多行）
			data := strings.TrimPrefix(line, "data: ")
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(data)
		} else if line == "" {
			// 空行表示事件结束，尝试解析
			if eventData.Len() > 0 {
				dataStr := strings.TrimSpace(eventData.String())
				if dataStr != "" {
					// 检查事件类型是否匹配
					if eventType != "" && currentEvent != eventType {
						currentEvent = ""
						eventData.Reset()
						continue
					}

					// 尝试解析 JSON-RPC 响应
					var rpcResponse struct {
						JSONRPC string          `json:"jsonrpc"`
						ID      interface{}     `json:"id"`
						Result  json.RawMessage `json:"result"`
						Error   *struct {
							Code    int    `json:"code"`
							Message string `json:"message"`
						} `json:"error"`
					}

					if err := json.Unmarshal([]byte(dataStr), &rpcResponse); err == nil {
						if rpcResponse.Error != nil {
							lastError = fmt.Errorf("MCP error (ID: %v): %s (code: %d)", rpcResponse.ID, rpcResponse.Error.Message, rpcResponse.Error.Code)
							currentEvent = ""
							eventData.Reset()
							continue
						}

						// 解析 result 字段
						if len(rpcResponse.Result) > 0 {
							var toolsResult struct {
								Tools []mcp.Tool `json:"tools"`
							}
							if err := json.Unmarshal(rpcResponse.Result, &toolsResult); err == nil {
								if len(toolsResult.Tools) > 0 {
									tools = append(tools, toolsResult.Tools...)
									// 找到工具列表，可以返回了
									if len(tools) > 0 {
										return tools, nil
									}
								}
							}
						}
					} else {
						// 尝试直接解析工具数组
						var directTools []mcp.Tool
						if err := json.Unmarshal([]byte(dataStr), &directTools); err == nil && len(directTools) > 0 {
							tools = append(tools, directTools...)
							return tools, nil
						}
					}
				}
			}
			currentEvent = ""
			eventData.Reset()
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("error reading SSE stream: %w", err)
	}

	if len(tools) == 0 {
		if lastError != nil {
			return nil, fmt.Errorf("no tools found in SSE stream. Last error: %w", lastError)
		}
		return nil, fmt.Errorf("no tools found in SSE stream")
	}

	return tools, nil
}

// parseJSONResponse 解析 JSON 响应
func (c *RemoteClient) parseJSONResponse(body io.Reader) ([]mcp.Tool, error) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 尝试解析 JSON-RPC 格式
	var rpcResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &rpcResponse); err == nil {
		if rpcResponse.Error != nil {
			return nil, fmt.Errorf("MCP error: %s (code: %d)", rpcResponse.Error.Message, rpcResponse.Error.Code)
		}

		// 解析 result 字段
		if len(rpcResponse.Result) > 0 {
			var toolsResult struct {
				Tools []mcp.Tool `json:"tools"`
			}
			if err := json.Unmarshal(rpcResponse.Result, &toolsResult); err == nil {
				if len(toolsResult.Tools) > 0 {
					return toolsResult.Tools, nil
				}
			}
		}
	}

	// 尝试直接解析工具数组
	var tools []mcp.Tool
	if err := json.Unmarshal(bodyBytes, &tools); err == nil && len(tools) > 0 {
		return tools, nil
	}

	// 尝试解析 {"tools": [...]} 格式
	var directResult struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(bodyBytes, &directResult); err == nil && len(directResult.Tools) > 0 {
		return directResult.Tools, nil
	}

	return nil, fmt.Errorf("failed to parse JSON response. Response body: %s", string(bodyBytes))
}

// discoverTools 自动发现工具（普通 HTTP 方式）
func (c *RemoteClient) discoverTools(endpoint string) error {
	if endpoint == "" {
		// 尝试多个常见的端点路径（按常见程度排序）
		endpoints := []string{
			"/mcp/tools",     // Dify 常用路径
			"/api/tools",     // 标准 API 路径
			"/tools",         // 简单路径
			"/v1/tools",      // 版本化路径
			"/api/v1/tools",  // 版本化 API 路径
			"/api/mcp/tools", // MCP API 路径
		}
		var lastErr error
		for _, ep := range endpoints {
			if err := c.tryDiscoverTools(ep); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return fmt.Errorf("failed to discover tools: tried common endpoints (%v) but all failed. Last error: %v", endpoints, lastErr)
	}

	return c.tryDiscoverTools(endpoint)
}

// tryDiscoverTools 尝试从指定端点发现工具
func (c *RemoteClient) tryDiscoverTools(endpoint string) error {
	// 如果 endpoint 是完整 URL，直接使用
	var url string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		url = endpoint
	} else {
		// 确保 endpoint 以 / 开头
		if !strings.HasPrefix(endpoint, "/") {
			endpoint = "/" + endpoint
		}
		// 构建完整 URL
		url = strings.TrimSuffix(c.baseURL, "/") + endpoint
	}

	// 先尝试 GET 请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request to %s: %w", url, err)
	}

	// 设置 Content-Type
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 添加认证头
	c.addAuthHeader(req, c.headers)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d from %s: %s", resp.StatusCode, url, bodyStr)
	}

	// 检查响应体是否为空
	if len(bodyStr) == 0 {
		return fmt.Errorf("empty response from %s. Please check if the endpoint path is correct", url)
	}

	// 尝试多种响应格式
	var tools []mcp.Tool

	// 格式1: 直接返回 {"tools": [...]}
	var directResult struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &directResult); err == nil && len(directResult.Tools) > 0 {
		tools = directResult.Tools
	} else {
		// 格式2: JSON-RPC 格式 {"jsonrpc": "2.0", "result": {"tools": [...]}}
		var rpcResponse struct {
			JSONRPC string `json:"jsonrpc"`
			Result  struct {
				Tools []mcp.Tool `json:"tools"`
			} `json:"result"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(bodyStr), &rpcResponse); err == nil {
			if rpcResponse.Error != nil {
				return fmt.Errorf("MCP error from %s: %s (code: %d)", url, rpcResponse.Error.Message, rpcResponse.Error.Code)
			}
			if len(rpcResponse.Result.Tools) > 0 {
				tools = rpcResponse.Result.Tools
			}
		} else {
			// 格式3: 直接返回工具数组 [...]
			if err := json.Unmarshal([]byte(bodyStr), &tools); err != nil {
				// 所有格式都失败
				return fmt.Errorf("failed to parse response from %s. Expected formats: {\"tools\": [...]}, JSON-RPC format, or tool array. Response body: %s", url, bodyStr)
			}
		}
	}

	if len(tools) == 0 {
		return fmt.Errorf("no tools found in response from %s. Response body: %s", url, bodyStr)
	}

	c.tools = tools
	return nil
}

// ListTools 列出所有工具
func (c *RemoteClient) ListTools() []mcp.Tool {
	return c.tools
}

// CallTool 调用工具（使用 JSON-RPC 协议）
func (c *RemoteClient) CallTool(name string, args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	// 构建 JSON-RPC 请求
	requestID := fmt.Sprintf("tool-call-%d", time.Now().UnixNano())
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 使用 BaseURL 发送 JSON-RPC 请求
	url := strings.TrimSuffix(c.baseURL, "/")
	log.Printf("[RemoteClient] Calling tool %s on %s with args: %+v", name, url, args)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	c.addAuthHeader(req, c.headers)

	// 使用25秒超时，确保在30秒整体超时前完成
	timeout := 25 * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	callStartTime := time.Now()
	resp, err := client.Do(req)
	callDuration := time.Since(callStartTime)
	if err != nil {
		log.Printf("[RemoteClient] ERROR: Tool %s call failed after %v: %v", name, callDuration, err)
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("[RemoteClient] Tool %s call completed in %v, status: %d", name, callDuration, resp.StatusCode)

	// 检查响应类型
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		// SSE 响应，需要解析流
		return c.parseToolCallSSE(resp.Body, requestID)
	}

	// JSON 响应
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tool call failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析 JSON-RPC 响应
	var rpcResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResponse.Error != nil {
		return nil, fmt.Errorf("MCP error: %s (code: %d)", rpcResponse.Error.Message, rpcResponse.Error.Code)
	}

	// 解析 result 字段
	var result mcp.ToolsCallResult
	if err := json.Unmarshal(rpcResponse.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	// 记录工具返回结果
	if len(result.Content) > 0 {
		resultText := result.Content[0].Text
		resultPreview := resultText
		if len(resultPreview) > 300 {
			resultPreview = resultPreview[:300] + "..."
		}
		log.Printf("[RemoteClient] Tool %s returned result (length: %d, preview: %s)", name, len(resultText), resultPreview)
	} else {
		log.Printf("[RemoteClient] WARNING: Tool %s returned empty result", name)
	}

	return &result, nil
}

// parseToolCallSSE 解析工具调用的 SSE 响应
func (c *RemoteClient) parseToolCallSSE(body io.Reader, requestID string) (*mcp.ToolsCallResult, error) {
	scanner := bufio.NewScanner(body)
	var eventData strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			// 忽略 event 类型，只处理 data
			eventData.Reset()
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(data)
		} else if line == "" {
			// 空行表示事件结束
			if eventData.Len() > 0 {
				dataStr := strings.TrimSpace(eventData.String())
				if dataStr != "" {
					// 解析 JSON-RPC 响应
					var rpcResponse struct {
						JSONRPC string          `json:"jsonrpc"`
						ID      interface{}     `json:"id"`
						Result  json.RawMessage `json:"result"`
						Error   *struct {
							Code    int    `json:"code"`
							Message string `json:"message"`
						} `json:"error"`
					}

					if err := json.Unmarshal([]byte(dataStr), &rpcResponse); err == nil {
						// 检查 ID 是否匹配
						if fmt.Sprintf("%v", rpcResponse.ID) == requestID {
							if rpcResponse.Error != nil {
								return nil, fmt.Errorf("MCP error: %s (code: %d)", rpcResponse.Error.Message, rpcResponse.Error.Code)
							}

							// 解析 result
							var result mcp.ToolsCallResult
							if err := json.Unmarshal(rpcResponse.Result, &result); err != nil {
								return nil, fmt.Errorf("failed to unmarshal result: %w", err)
							}
							return &result, nil
						}
					}
				}
			}
			eventData.Reset()
		}
	}

	return nil, fmt.Errorf("no valid response found in SSE stream for request ID: %s", requestID)
}

// addAuthHeader 添加认证头（现在由调用方传入 headers）
func (c *RemoteClient) addAuthHeader(req *http.Request, headers map[string]string) {
	for name, value := range headers {
		req.Header.Set(name, value)
	}
}
