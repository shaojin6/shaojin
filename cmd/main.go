package main

import (
	"log"
	"os"

	"github.com/your-org/k8s-mcp-agent/internal/k8s"
	"github.com/your-org/k8s-mcp-agent/internal/server"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

func main() {
	// 初始化 Kubernetes 客户端
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// 创建 MCP 服务器
	mcpServer := mcp.NewServer("k8s-mcp-agent", "1.0.0")

	// 注册所有 Kubernetes 工具
	server.RegisterK8sTools(mcpServer, k8sClient.Clientset)

	// 处理标准输入/输出流（MCP 协议通常通过 stdio 通信）
	log.Println("Kubernetes MCP Agent started. Waiting for requests...")
	if err := mcpServer.ProcessStream(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("Error processing stream: %v", err)
	}
}

