package main

import (
	"io"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/your-org/k8s-mcp-agent/internal/config"
	"github.com/your-org/k8s-mcp-agent/internal/k8s"
	"github.com/your-org/k8s-mcp-agent/internal/server"
	"github.com/your-org/k8s-mcp-agent/internal/web/api"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

func main() {
	// 设置日志输出：同时输出到控制台和日志文件
	logFile := setupLogging()
	if logFile != nil {
		defer logFile.Close()
	}
	// 加载配置文件（.env）
	configFile := config.FindConfigFile()
	if configFile != "" {
		log.Printf("Loading config from: %s", configFile)
		if err := config.LoadEnvFile(configFile); err != nil {
			log.Printf("Warning: Failed to load config file: %v (will use environment variables)", err)
		} else {
			log.Printf("Config file loaded successfully")
			// 调试：显示加载的配置（不显示敏感信息）
			if apiServer := os.Getenv("K8S_API_SERVER"); apiServer != "" {
				log.Printf("K8S_API_SERVER: %s", apiServer)
			}
			if token := os.Getenv("K8S_API_TOKEN"); token != "" {
				log.Printf("K8S_API_TOKEN: (set, length: %d)", len(token))
			}
		}
	} else {
		// 尝试加载当前目录的 .env
		if cwd, err := os.Getwd(); err == nil {
			envFile := cwd + string(os.PathSeparator) + ".env"
			if _, err := os.Stat(envFile); err == nil {
				log.Printf("Loading config from current directory: %s", envFile)
				if err := config.LoadEnvFile(envFile); err != nil {
					log.Printf("Warning: Failed to load config file: %v", err)
				} else {
					log.Printf("Config file loaded successfully")
				}
			} else {
				log.Printf("No .env file found, using environment variables only")
			}
		}
	}

	// 初始化 Kubernetes 客户端（允许失败，Web 服务仍可启动）
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Printf("Warning: Failed to create Kubernetes client: %v (Web service will start but K8s features will be unavailable)", err)
		k8sClient = nil
	}

	// 创建 MCP 服务器
	mcpServer := mcp.NewServer("k8s-mcp-agent", "1.0.0")

	// 注册所有 Kubernetes 工具（如果客户端可用）
	if k8sClient != nil {
		server.RegisterK8sTools(mcpServer, k8sClient.Clientset)
	} else {
		log.Printf("Warning: Skipping K8s tools registration due to client initialization failure")
	}

	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化路由
	router := api.SetupRouter(mcpServer, k8sClient)

	// 启动 Web 服务
	addr := os.Getenv("MCP_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("Starting MCP Web service on %s", addr)
	log.Printf("Logs are being written to: service.log")
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start web service: %v", err)
	}
}

// setupLogging 设置日志输出到文件和控制台
func setupLogging() *os.File {
	// 创建日志文件
	logFile, err := os.OpenFile("service.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Warning: Failed to open log file: %v (logs will only go to console)", err)
		return nil
	}

	// 设置日志输出：同时输出到文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return logFile
}

