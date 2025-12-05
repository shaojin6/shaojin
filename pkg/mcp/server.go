package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
)

// Server MCP 服务器
type Server struct {
	tools       map[string]ToolHandler
	toolDefs    map[string]Tool
	mu          sync.RWMutex
	initialized bool
	serverInfo  ServerInfo
}

// ToolHandler 工具处理函数类型
type ToolHandler func(arguments map[string]interface{}) (*ToolsCallResult, error)

// NewServer 创建新的 MCP 服务器
func NewServer(name, version string) *Server {
	return &Server{
		tools:    make(map[string]ToolHandler),
		toolDefs: make(map[string]Tool),
		serverInfo: ServerInfo{
			Name:    name,
			Version: version,
		},
	}
}

// RegisterTool 注册工具
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = handler
	s.toolDefs[tool.Name] = tool
}

// ListTools 返回所有注册的工具（供 Web 层调用）
func (s *Server) ListTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.toolDefs))
	for _, tool := range s.toolDefs {
		tools = append(tools, tool)
	}
	return tools
}

// CallTool 直接调用指定工具（供 Web 层调用）
func (s *Server) CallTool(name string, args map[string]interface{}) (*ToolsCallResult, error) {
	s.mu.RLock()
	handler, exists := s.tools[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return handler(args)
}

// HandleRequest 处理请求
func (s *Server) HandleRequest(req *Request) *Response {
	var resp *Response

	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(req)
	default:
		resp = &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}

	return resp
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(req *Request) *Response {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: s.serverInfo,
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsList 处理工具列表请求
func (s *Server) handleToolsList(req *Request) *Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.toolDefs))
	for _, tool := range s.toolDefs {
		tools = append(tools, tool)
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolsListResult{
			Tools: tools,
		},
	}
}

// handleToolsCall 处理工具调用请求
func (s *Server) handleToolsCall(req *Request) *Response {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var params ToolsCallParams
	if err := unmarshalParams(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    InvalidParams,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	handler, exists := s.tools[params.Name]
	if !exists {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Tool not found: %s", params.Name),
			},
		}
	}

	result, err := handler(params.Arguments)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ToolsCallResult{
				Content: []Content{
					{
						Type: "text",
						Text: fmt.Sprintf("Error: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// ProcessStream 处理输入流（从 stdin 读取 JSON-RPC 请求）
func (s *Server) ProcessStream(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	encoder := json.NewEncoder(output)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding request: %v", err)
			continue
		}

		resp := s.HandleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
			continue
		}
	}

	return nil
}

// unmarshalParams 将参数解组到目标结构
func unmarshalParams(params interface{}, target interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
