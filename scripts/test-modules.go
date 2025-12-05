package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/config"
	"github.com/your-org/k8s-mcp-agent/internal/web/cache"
	"github.com/your-org/k8s-mcp-agent/internal/web/llm"
	"github.com/your-org/k8s-mcp-agent/internal/web/mcpclient"
	"github.com/your-org/k8s-mcp-agent/internal/web/store"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

func main() {
	fmt.Println("=== MCP 智能体模块测试 ===\n")

	// 加载配置
	configFile := config.FindConfigFile()
	if configFile != "" {
		fmt.Printf("1. 加载配置文件: %s\n", configFile)
		if err := config.LoadEnvFile(configFile); err != nil {
			log.Fatalf("加载配置失败: %v", err)
		}
		fmt.Println("   ✓ 配置加载成功\n")
	}

	// 测试配置存储
	fmt.Println("2. 测试配置存储...")
	cfgStore := store.NewPersistentStore()

	// 获取默认 LLM
	llmConfig := cfgStore.GetDefaultLLMConfig()
	if llmConfig == nil {
		log.Fatal("   ✗ 未找到默认 LLM 配置")
	}
	fmt.Printf("   ✓ 找到默认 LLM: %s (Provider: %s, Model: %s)\n", llmConfig.Name, llmConfig.Provider, llmConfig.Model)

	// 获取 Agent
	agents := cfgStore.GetAllAgents()
	if len(agents) == 0 {
		log.Fatal("   ✗ 未找到 Agent 配置")
	}
	agent := &agents[0]
	fmt.Printf("   ✓ 找到 Agent: %s (MCP: %s)\n\n", agent.Name, agent.MCPServerID)

	// 测试缓存系统
	fmt.Println("3. 测试缓存系统...")
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "11.0.1.110:31202"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "difyai123456"
	}
	redisDB := 1
	if redisDBStr := os.Getenv("REDIS_DB"); redisDBStr != "" {
		fmt.Sscanf(redisDBStr, "%d", &redisDB)
	}

	var redisCache cache.Cache
	var err error
	if redisCache, err = cache.NewRedisCache(redisAddr, redisPassword, redisDB); err != nil {
		fmt.Printf("   ⚠ Redis 连接失败: %v (将跳过 Redis 测试)\n", err)
		redisCache = nil
	} else {
		fmt.Printf("   ✓ Redis 连接成功: %s\n", redisAddr)
	}

	// MySQL 缓存
	mysqlHost := os.Getenv("MYSQL_HOST")
	if mysqlHost == "" {
		mysqlHost = "11.0.1.110"
	}
	mysqlPort := os.Getenv("MYSQL_PORT")
	if mysqlPort == "" {
		mysqlPort = "30306"
	}
	mysqlUser := os.Getenv("MYSQL_USER")
	if mysqlUser == "" {
		mysqlUser = "root"
	}
	mysqlPassword := os.Getenv("MYSQL_PASSWORD")
	if mysqlPassword == "" {
		mysqlPassword = "canxixi"
	}
	mysqlDB := os.Getenv("MYSQL_DB")
	if mysqlDB == "" {
		mysqlDB = "mcp"
	}
	mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDB)

	fileCache := cache.NewDBCache(cfgStore)
	var dbCache cache.Cache = fileCache
	if mysqlCache, err := cache.NewMySQLCache(mysqlDSN, fileCache); err != nil {
		fmt.Printf("   ⚠ MySQL 连接失败: %v (将使用文件缓存)\n", err)
	} else {
		fmt.Printf("   ✓ MySQL 连接成功: %s:%s/%s\n", mysqlHost, mysqlPort, mysqlDB)
		dbCache = mysqlCache
	}

	toolsCache := cache.NewMultiLevelCache(redisCache, dbCache)
	fmt.Println("   ✓ 多级缓存初始化完成\n")

	// 测试工具列表获取
	fmt.Printf("4. 测试工具列表获取 (MCP: %s)...\n", agent.MCPServerID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startTime := time.Now()
	tools, err := toolsCache.GetTools(ctx, agent.MCPServerID)
	duration := time.Since(startTime)
	if err != nil {
		fmt.Printf("   ✗ 获取工具列表失败: %v (耗时: %v)\n", err, duration)
	} else if len(tools) == 0 {
		fmt.Printf("   ⚠ 工具列表为空 (耗时: %v)\n", duration)
	} else {
		fmt.Printf("   ✓ 获取到 %d 个工具 (耗时: %v)\n", len(tools), duration)
		for i, tool := range tools {
			if i < 3 {
				fmt.Printf("      - %s: %s\n", tool.Name, tool.Description)
			}
		}
		if len(tools) > 3 {
			fmt.Printf("      ... 还有 %d 个工具\n", len(tools)-3)
		}
	}
	fmt.Println()

	// 测试 LLM 连接
	fmt.Println("5. 测试 LLM 连接...")
	llmClient, err := llm.NewClient(llmConfig)
	if err != nil {
		log.Fatalf("   ✗ 创建 LLM 客户端失败: %v", err)
	}
	fmt.Println("   ✓ LLM 客户端创建成功")

	// 测试 LLM 调用
	fmt.Println("   测试 LLM 简单调用...")
	testMessages := []llm.Message{
		{Role: "user", Content: "你好"},
	}
	llmStartTime := time.Now()
	llmCtx, llmCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer llmCancel()

	// 注意：LLM.Chat 没有 context 参数，我们需要在超时后中断
	done := make(chan string, 1)
	errChan := make(chan error, 1)
	go func() {
		response, err := llmClient.Chat(testMessages)
		if err != nil {
			errChan <- err
		} else {
			done <- response
		}
	}()

	select {
	case response := <-done:
		llmDuration := time.Since(llmStartTime)
		fmt.Printf("   ✓ LLM 调用成功 (耗时: %v)\n", llmDuration)
		fmt.Printf("   响应: %s\n", response)
	case err := <-errChan:
		llmDuration := time.Since(llmStartTime)
		fmt.Printf("   ✗ LLM 调用失败 (耗时: %v): %v\n", llmDuration, err)
	case <-llmCtx.Done():
		llmDuration := time.Since(llmStartTime)
		fmt.Printf("   ✗ LLM 调用超时 (耗时: %v)\n", llmDuration)
	}
	fmt.Println()

	// 测试 MCP 工具调用
	fmt.Println("6. 测试 MCP 工具调用...")
	mcpServer := mcp.NewServer("test", "1.0.0")
	mcpClient := mcpclient.NewClient(mcpServer)
	toolManager := mcpclient.NewToolManager(mcpClient, cfgStore, toolsCache)

	// 刷新工具
	fmt.Println("   刷新工具列表...")
	refreshStart := time.Now()
	toolManager.RefreshRemoteTools()
	refreshDuration := time.Since(refreshStart)
	fmt.Printf("   ✓ 工具刷新完成 (耗时: %v)\n", refreshDuration)

	// 获取工具列表
	agentTools := toolManager.ListToolsForAgent(agent)
	if len(agentTools) == 0 {
		fmt.Println("   ⚠ Agent 没有可用工具")
	} else {
		fmt.Printf("   ✓ Agent 有 %d 个可用工具\n", len(agentTools))

		// 测试调用一个简单工具（如果有的话）
		if len(agentTools) > 0 {
			testTool := agentTools[0]
			fmt.Printf("   测试调用工具: %s\n", testTool.Name)
			toolStartTime := time.Now()
			toolCtx, toolCancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer toolCancel()

			toolDone := make(chan bool, 1)
			toolErrChan := make(chan error, 1)
			go func() {
				result, err := toolManager.CallTool(testTool.Name, map[string]interface{}{})
				if err != nil {
					toolErrChan <- err
				} else if result != nil {
					toolDone <- true
				} else {
					toolErrChan <- fmt.Errorf("工具返回空结果")
				}
			}()

			select {
			case <-toolDone:
				toolDuration := time.Since(toolStartTime)
				fmt.Printf("   ✓ 工具调用成功 (耗时: %v)\n", toolDuration)
			case err := <-toolErrChan:
				toolDuration := time.Since(toolStartTime)
				fmt.Printf("   ✗ 工具调用失败 (耗时: %v): %v\n", toolDuration, err)
			case <-toolCtx.Done():
				toolDuration := time.Since(toolStartTime)
				fmt.Printf("   ✗ 工具调用超时 (耗时: %v)\n", toolDuration)
			}
		}
	}
	fmt.Println()

	fmt.Println("=== 测试完成 ===")
}
