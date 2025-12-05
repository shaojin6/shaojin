package server

import (
	"fmt"

	"github.com/your-org/k8s-mcp-agent/internal/k8s"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
	"k8s.io/client-go/kubernetes"
)

// RegisterK8sTools 注册所有 Kubernetes 工具到 MCP 服务器
func RegisterK8sTools(server *mcp.Server, clientset *kubernetes.Clientset) {
	// 注册 list_pods 工具
	server.RegisterTool(mcp.Tool{
		Name:        "list_pods",
		Description: "列出指定命名空间中的所有 Pods，包括名称、状态、就绪情况和年龄。如果不指定命名空间，则查询所有命名空间",
		InputSchema: mcp.ToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes 命名空间名称（可选，留空表示查询所有命名空间）",
				},
			},
		},
		Required: []string{},
	}, func(arguments map[string]interface{}) (*mcp.ToolsCallResult, error) {
		namespace := ""
		if ns, ok := arguments["namespace"].(string); ok && ns != "" {
			namespace = ns
		}

		result, err := k8s.ListPods(clientset, namespace)
		if err != nil {
			return nil, err
		}

		return &mcp.ToolsCallResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	})

	// 注册 get_deployment_status 工具
	server.RegisterTool(mcp.Tool{
		Name:        "get_deployment_status",
		Description: "获取指定 Deployment 的详细状态信息，包括副本数、就绪副本数和条件",
		InputSchema: mcp.ToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes 命名空间名称",
				},
				"deployment": map[string]interface{}{
					"type":        "string",
					"description": "Deployment 名称",
				},
			},
		},
		Required: []string{"namespace", "deployment"},
	}, func(arguments map[string]interface{}) (*mcp.ToolsCallResult, error) {
		namespace, ok := arguments["namespace"].(string)
		if !ok || namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}

		deployment, ok := arguments["deployment"].(string)
		if !ok || deployment == "" {
			return nil, fmt.Errorf("deployment is required")
		}

		result, err := k8s.GetDeploymentStatus(clientset, namespace, deployment)
		if err != nil {
			return nil, err
		}

		return &mcp.ToolsCallResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	})

	// 注册 restart_pod 工具
	server.RegisterTool(mcp.Tool{
		Name:        "restart_pod",
		Description: "重启指定的 Pod（通过删除 Pod 实现，如果由 Deployment 管理，会自动重新创建）",
		InputSchema: mcp.ToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes 命名空间名称",
				},
				"pod": map[string]interface{}{
					"type":        "string",
					"description": "Pod 名称",
				},
			},
		},
		Required: []string{"namespace", "pod"},
	}, func(arguments map[string]interface{}) (*mcp.ToolsCallResult, error) {
		namespace, ok := arguments["namespace"].(string)
		if !ok || namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}

		pod, ok := arguments["pod"].(string)
		if !ok || pod == "" {
			return nil, fmt.Errorf("pod is required")
		}

		result, err := k8s.RestartPod(clientset, namespace, pod)
		if err != nil {
			return nil, err
		}

		return &mcp.ToolsCallResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	})

	// 注册 get_pod_logs 工具
	server.RegisterTool(mcp.Tool{
		Name:        "get_pod_logs",
		Description: "获取指定 Pod 的日志，可以指定要获取的行数",
		InputSchema: mcp.ToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes 命名空间名称",
				},
				"pod": map[string]interface{}{
					"type":        "string",
					"description": "Pod 名称",
				},
				"tailLines": map[string]interface{}{
					"type":        "number",
					"description": "要获取的日志行数（默认 100）",
				},
			},
		},
		Required: []string{"namespace", "pod"},
	}, func(arguments map[string]interface{}) (*mcp.ToolsCallResult, error) {
		namespace, ok := arguments["namespace"].(string)
		if !ok || namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}

		pod, ok := arguments["pod"].(string)
		if !ok || pod == "" {
			return nil, fmt.Errorf("pod is required")
		}

		tailLines := int64(100)
		if tl, ok := arguments["tailLines"].(float64); ok {
			tailLines = int64(tl)
		}

		result, err := k8s.GetPodLogs(clientset, namespace, pod, tailLines)
		if err != nil {
			return nil, err
		}

		return &mcp.ToolsCallResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	})

	// 注册 list_deployments 工具
	server.RegisterTool(mcp.Tool{
		Name:        "list_deployments",
		Description: "列出指定命名空间中的所有 Deployments。如果不指定命名空间，则查询所有命名空间",
		InputSchema: mcp.ToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes 命名空间名称（可选，留空表示查询所有命名空间）",
				},
			},
		},
		Required: []string{},
	}, func(arguments map[string]interface{}) (*mcp.ToolsCallResult, error) {
		namespace := ""
		if ns, ok := arguments["namespace"].(string); ok && ns != "" {
			namespace = ns
		}

		result, err := k8s.ListDeployments(clientset, namespace)
		if err != nil {
			return nil, err
		}

		return &mcp.ToolsCallResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	})

	// 注册 get_service_info 工具
	server.RegisterTool(mcp.Tool{
		Name:        "get_service_info",
		Description: "获取指定 Service 的详细信息，包括端口、类型和选择器",
		InputSchema: mcp.ToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes 命名空间名称",
				},
				"service": map[string]interface{}{
					"type":        "string",
					"description": "Service 名称",
				},
			},
		},
		Required: []string{"namespace", "service"},
	}, func(arguments map[string]interface{}) (*mcp.ToolsCallResult, error) {
		namespace, ok := arguments["namespace"].(string)
		if !ok || namespace == "" {
			return nil, fmt.Errorf("namespace is required")
		}

		service, ok := arguments["service"].(string)
		if !ok || service == "" {
			return nil, fmt.Errorf("service is required")
		}

		result, err := k8s.GetServiceInfo(clientset, namespace, service)
		if err != nil {
			return nil, err
		}

		return &mcp.ToolsCallResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: result,
				},
			},
		}, nil
	})
}

