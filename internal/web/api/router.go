package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/k8s-mcp-agent/internal/k8s"
	"github.com/your-org/k8s-mcp-agent/internal/web/cache"
	"github.com/your-org/k8s-mcp-agent/internal/web/chat"
	"github.com/your-org/k8s-mcp-agent/internal/web/llm"
	"github.com/your-org/k8s-mcp-agent/internal/web/mcpclient"
	"github.com/your-org/k8s-mcp-agent/internal/web/store"
	"github.com/your-org/k8s-mcp-agent/internal/web/types"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

var (
	orchestrator   *chat.Orchestrator
	orchestratorMu sync.RWMutex
	toolManager    *mcpclient.ToolManager
	toolManagerMu  sync.RWMutex
)

// API 调用统计
type APIStats struct {
	mu                sync.RWMutex
	totalRequests     int64
	totalErrors       int64
	requestsByPath    map[string]int64
	errorsByPath      map[string]int64
	lastRequestTime   time.Time
	lastErrorTime     time.Time
	lastRequestPath   string
	lastErrorPath     string
}

var apiStats = &APIStats{
	requestsByPath: make(map[string]int64),
	errorsByPath:   make(map[string]int64),
}

// recordRequest 记录 API 请求
func recordRequest(path string, isError bool) {
	apiStats.mu.Lock()
	defer apiStats.mu.Unlock()
	
	apiStats.totalRequests++
	apiStats.requestsByPath[path]++
	apiStats.lastRequestTime = time.Now()
	apiStats.lastRequestPath = path
	
	if isError {
		apiStats.totalErrors++
		apiStats.errorsByPath[path]++
		apiStats.lastErrorTime = time.Now()
		apiStats.lastErrorPath = path
	}
}

// getAPIStats 获取 API 统计信息
func getAPIStats() map[string]interface{} {
	apiStats.mu.RLock()
	defer apiStats.mu.RUnlock()
	
	// 获取调用最多的前 5 个接口
	topPaths := make([]map[string]interface{}, 0)
	type pathCount struct {
		path  string
		count int64
	}
	paths := make([]pathCount, 0, len(apiStats.requestsByPath))
	for path, count := range apiStats.requestsByPath {
		paths = append(paths, pathCount{path: path, count: count})
	}
	
	// 简单排序（取前5个）
	for i := 0; i < len(paths) && i < 5; i++ {
		maxIdx := i
		for j := i + 1; j < len(paths); j++ {
			if paths[j].count > paths[maxIdx].count {
				maxIdx = j
			}
		}
		paths[i], paths[maxIdx] = paths[maxIdx], paths[i]
		topPaths = append(topPaths, map[string]interface{}{
			"path":  paths[i].path,
			"count": paths[i].count,
		})
	}
	
	result := map[string]interface{}{
		"totalRequests": apiStats.totalRequests,
		"totalErrors":   apiStats.totalErrors,
		"topPaths":      topPaths,
	}
	
	if !apiStats.lastRequestTime.IsZero() {
		result["lastRequestTime"] = apiStats.lastRequestTime.Unix()
		result["lastRequestPath"] = apiStats.lastRequestPath
	}
	
	if !apiStats.lastErrorTime.IsZero() {
		result["lastErrorTime"] = apiStats.lastErrorTime.Unix()
		result["lastErrorPath"] = apiStats.lastErrorPath
	}
	
	return result
}

// SetupRouter 初始化 HTTP 路由
func SetupRouter(mcpServer *mcp.Server, k8sClient *k8s.Client) *gin.Engine {
	r := gin.Default()

	// CORS 中间件（开发环境）
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	
	// API 统计中间件（排除状态接口自身，避免循环统计）
	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 排除状态接口和日志接口，避免统计自身
		if path != "/api/status" && path != "/api/logs" {
			c.Next()
			// 在响应完成后记录统计
			isError := c.Writer.Status() >= 400
			recordRequest(path, isError)
		} else {
			c.Next()
		}
	})

	// 使用持久化存储，确保配置在重启后保留
	cfgStore := store.GetPersistentStore()
	mcpClient := mcpclient.NewClient(mcpServer)

	// 初始化缓存系统（Redis + MySQL + 本地持久化）
	var toolsCache cache.Cache
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "11.0.1.110:31202" // 默认使用用户提供的 Redis 地址
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "difyai123456" // 默认使用用户提供的密码
	}
	redisDB := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		fmt.Sscanf(dbStr, "%d", &redisDB)
	}

	// 尝试连接 Redis
	redisCache, err := cache.NewRedisCache(redisAddr, redisPassword, redisDB)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v (will use DB cache only)", err)
		redisCache = nil
	} else {
		log.Printf("Redis cache connected: %s", redisAddr)
	}

	// 创建持久化文件缓存（兜底）
	fileCache := cache.NewDBCache(cfgStore)

	// 初始化 MySQL 缓存（可选）
	var dbCache cache.Cache = fileCache
	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
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
		mysqlDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
			mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDB)
	}

	if mysqlDSN != "" {
		mysqlCache, err := cache.NewMySQLCache(mysqlDSN, fileCache)
		if err != nil {
			log.Printf("Warning: Failed to connect to MySQL cache: %v (fallback to file cache)", err)
		} else {
			log.Printf("MySQL cache connected.")
			dbCache = mysqlCache
		}
		
		// 初始化 MySQL 存储（用于 Agent 配置，包括 SystemPrompt）
		mysqlStore, err := store.NewMySQLStore(mysqlDSN)
		if err != nil {
			log.Printf("Warning: Failed to connect to MySQL store for agents: %v (will use file storage)", err)
		} else {
			log.Printf("MySQL store for agents connected.")
			cfgStore.SetMySQLStore(mysqlStore)
			// 可选：从文件迁移数据到 MySQL（一次性操作）
			// mysqlStore.MigrateFromFileStore(cfgStore)
		}
	}

	// 创建多级缓存（Redis -> MySQL -> 文件）
	toolsCache = cache.NewMultiLevelCache(redisCache, dbCache)

	// 初始化 ToolManager（传入缓存）
	toolManagerMu.Lock()
	toolManager = mcpclient.NewToolManager(mcpClient, cfgStore, toolsCache)
	toolManagerMu.Unlock()

	// 初始化 MongoDB 会话存储
	var sessionStore store.SessionStore
	mongoURI := os.Getenv("MONGODB_URI")
	mongoDB := os.Getenv("MONGODB_DB")
	if mongoDB == "" {
		mongoDB = "mcp"
	}

	if mongoURI == "" {
		mongoHost := os.Getenv("MONGODB_HOST")
		if mongoHost == "" {
			mongoHost = "11.0.1.110"
			log.Printf("MONGODB_HOST not set, using default: %s", mongoHost)
		} else {
			log.Printf("MONGODB_HOST loaded from env: %s", mongoHost)
		}
		mongoPort := os.Getenv("MONGODB_PORT")
		if mongoPort == "" {
			mongoPort = "30792"
			log.Printf("MONGODB_PORT not set, using default: %s", mongoPort)
		} else {
			log.Printf("MONGODB_PORT loaded from env: %s", mongoPort)
		}
		log.Printf("MONGODB_DB: %s", mongoDB)
		// MongoDB URI 格式：mongodb://host:port/database?directConnection=true
		// 数据库名必须在查询参数之前
		mongoURI = fmt.Sprintf("mongodb://%s:%s/%s?directConnection=true", mongoHost, mongoPort, mongoDB)
		log.Printf("Constructed MongoDB URI: %s", mongoURI)
	} else {
		log.Printf("MONGODB_URI loaded from env: %s", mongoURI)
		log.Printf("MONGODB_DB: %s", mongoDB)
	}

	log.Printf("Attempting to connect to MongoDB: %s (database: %s)", mongoURI, mongoDB)
	mongoStore, err := store.NewMongoSessionStore(mongoURI, mongoDB)
	if err != nil {
		log.Printf("Warning: Failed to connect to MongoDB: %v (using memory store as fallback)", err)
		sessionStore = store.NewMemorySessionStore() // 使用内存存储作为降级
	} else {
		log.Printf("MongoDB session store connected: %s/%s", mongoURI, mongoDB)
		sessionStore = mongoStore
	}

	// 将 sessionStore 保存到全局变量，供 getOrchestrator 使用
	globalSessionStore := sessionStore

	// 确保至少存在一个默认 Agent（优先绑定 kubernetes-mcp-server）
	ensureDefaultAgent(cfgStore)

	// 初始化 Orchestrator（延迟初始化，需要 LLM 配置）

	// 健康检查（最先注册）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, types.StatusResponse{Status: "ok"})
	})

	// 认证相关（必须在 API 路由组之前）
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/login", func(c *gin.Context) {
			var req struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// 简单的用户名密码验证（admin/admin）
			if req.Username == "admin" && req.Password == "admin" {
				// 设置 session（这里简化处理，实际应该使用 JWT 或 session）
				c.SetCookie("auth_token", "authenticated", 3600*24, "/", "", false, true)
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"message": "Login successful",
				})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "Invalid username or password",
				})
			}
		})

		authGroup.POST("/logout", func(c *gin.Context) {
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		authGroup.GET("/check", func(c *gin.Context) {
			token, err := c.Cookie("auth_token")
			if err == nil && token == "authenticated" {
				c.JSON(http.StatusOK, gin.H{"authenticated": true})
			} else {
				c.JSON(http.StatusOK, gin.H{"authenticated": false})
			}
		})
	}

	// API 路由组
	apiGroup := r.Group("/api")
	{
		// 配置管理
		apiGroup.GET("/config", func(c *gin.Context) {
			resp := types.ConfigResponse{
				K8s:        cfgStore.GetAllK8sConfigs(),
				LLM:        cfgStore.GetAllLLMConfigs(),
				RemoteMCPs: cfgStore.GetAllRemoteMCPs(),
				Agents:     cfgStore.GetAllAgents(),
			}
			c.JSON(http.StatusOK, resp)
		})

		// K8s 配置管理
		apiGroup.GET("/config/k8s", func(c *gin.Context) {
			configs := cfgStore.GetAllK8sConfigs()
			c.JSON(http.StatusOK, configs)
		})

		apiGroup.GET("/config/k8s/:id", func(c *gin.Context) {
			id := c.Param("id")
			config := cfgStore.GetK8sConfig(id)
			if config == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "K8s config not found"})
				return
			}
			c.JSON(http.StatusOK, config)
		})

		apiGroup.POST("/config/k8s", func(c *gin.Context) {
			var req types.K8sConfig
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			cfgStore.SetK8sConfig(req)
			c.Status(http.StatusNoContent)
		})

		apiGroup.PUT("/config/k8s/:id", func(c *gin.Context) {
			id := c.Param("id")
			var req types.K8sConfig
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			req.ID = id // 确保ID一致
			cfgStore.SetK8sConfig(req)
			c.Status(http.StatusNoContent)
		})

		apiGroup.DELETE("/config/k8s/:id", func(c *gin.Context) {
			id := c.Param("id")
			cfgStore.DeleteK8sConfig(id)
			c.Status(http.StatusNoContent)
		})

		// LLM 配置管理
		apiGroup.GET("/config/llm", func(c *gin.Context) {
			configs := cfgStore.GetAllLLMConfigs()
			c.JSON(http.StatusOK, configs)
		})

		apiGroup.GET("/config/llm/:id", func(c *gin.Context) {
			id := c.Param("id")
			config := cfgStore.GetLLMConfig(id)
			if config == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "LLM config not found"})
				return
			}
			c.JSON(http.StatusOK, config)
		})

		apiGroup.POST("/config/llm", func(c *gin.Context) {
			var req types.LLMConfig
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			cfgStore.SetLLMConfig(req)
			// 重置 Orchestrator，以便使用新配置
			ResetOrchestrator()
			c.Status(http.StatusNoContent)
		})

		apiGroup.PUT("/config/llm/:id", func(c *gin.Context) {
			id := c.Param("id")
			var req types.LLMConfig
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			req.ID = id // 确保ID一致
			cfgStore.SetLLMConfig(req)
			// 重置 Orchestrator，以便使用新配置
			ResetOrchestrator()
			c.Status(http.StatusNoContent)
		})

		apiGroup.DELETE("/config/llm/:id", func(c *gin.Context) {
			id := c.Param("id")
			cfgStore.DeleteLLMConfig(id)
			// 重置 Orchestrator
			ResetOrchestrator()
			c.Status(http.StatusNoContent)
		})

		// 连通性测试（支持 GET 和 POST）
		testK8sHandler := func(c *gin.Context) {
			// 检查是否有启用的 K8s 配置
			enabledConfigs := cfgStore.GetEnabledK8sConfigs()
			if len(enabledConfigs) == 0 {
				c.JSON(http.StatusOK, types.TestResponse{
					Status:  "failed",
					Message: "没有启用的 K8s 配置。请在 K8s 配置页面启用至少一个集群配置。",
				})
				return
			}

			// 注意：当前系统仍使用启动时从环境变量创建的全局 k8sClient
			// 这个测试检查的是全局客户端，而不是 Web UI 配置的客户端
			// TODO: 未来应该根据 Web UI 配置动态创建客户端
			if k8sClient == nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "failed",
					"message": "Kubernetes client not initialized. Please check your .env configuration.",
				})
				return
			}

			// 尝试获取 API 版本信息来测试连接
			version, err := k8sClient.Clientset.ServerVersion()
			if err != nil {
				c.JSON(http.StatusOK, types.TestResponse{
					Status:  "failed",
					Message: err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, types.TestResponse{
				Status:  "ok",
				Message: "Connected to Kubernetes API: " + version.String(),
			})
		}
		apiGroup.GET("/test-k8s", testK8sHandler)
		apiGroup.POST("/test-k8s", testK8sHandler)

		// 测试 LLM 连接（支持指定 ID 或使用默认配置）
		apiGroup.POST("/test-llm", func(c *gin.Context) {
			var req struct {
				ID string `json:"id"`
			}
			c.ShouldBindJSON(&req)

			var llmConfig *types.LLMConfig
			if req.ID != "" {
				llmConfig = cfgStore.GetLLMConfig(req.ID)
				if llmConfig == nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("LLM config with ID %s not found", req.ID)})
					return
				}
			} else {
				llmConfig = cfgStore.GetDefaultLLMConfig()
				if llmConfig == nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "llm config not set"})
					return
				}
			}

			llmName := llmConfig.Name
			if llmName == "" {
				llmName = llmConfig.Provider
			}
			if llmName == "" {
				llmName = "未命名 LLM"
			}

			// 创建 LLM 客户端并测试
			llmClient, err := llm.NewClient(llmConfig)
			if err != nil {
				c.JSON(http.StatusOK, types.TestResponse{
					Status:  "failed",
					Message: fmt.Sprintf("%s 连接失败: %v", llmName, err),
				})
				return
			}

			if err := llmClient.TestConnection(); err != nil {
				c.JSON(http.StatusOK, types.TestResponse{
					Status:  "failed",
					Message: fmt.Sprintf("%s 连接测试失败: %v", llmName, err),
				})
				return
			}

			c.JSON(http.StatusOK, types.TestResponse{
				Status:  "ok",
				Message: fmt.Sprintf("%s 连接测试成功", llmName),
			})
		})

		// 工具相关
		apiGroup.GET("/tools", func(c *gin.Context) {
			if mcpClient == nil {
				log.Println("ERROR: mcpClient is nil")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP client not initialized"})
				return
			}
			tools := mcpClient.ListTools()
			c.JSON(http.StatusOK, gin.H{"tools": tools})
		})

		apiGroup.POST("/tools/call", func(c *gin.Context) {
			// 添加 panic 恢复
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC in tool call: %v", r)
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":   fmt.Sprintf("Internal server error: %v", r),
						"message": "A panic occurred while calling the tool",
					})
				}
			}()

			var req types.ToolCallRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				log.Printf("ERROR: Failed to parse request body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			if req.Name == "" {
				log.Printf("ERROR: Tool name is empty")
				c.JSON(http.StatusBadRequest, gin.H{"error": "tool name is required"})
				return
			}

			if mcpClient == nil {
				log.Println("ERROR: mcpClient is nil")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "MCP client not initialized"})
				return
			}

			log.Printf("Calling tool: %s with args: %+v", req.Name, req.Arguments)

			result, err := mcpClient.CallTool(req.Name, req.Arguments)
			if err != nil {
				log.Printf("ERROR calling tool %s: %v", req.Name, err)
				// 确保错误信息完整
				errorMsg := err.Error()
				if errorMsg == "" {
					errorMsg = "Unknown error occurred"
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   errorMsg,
					"tool":    req.Name,
					"message": "Failed to call tool",
				})
				return
			}

			if result == nil {
				log.Printf("ERROR: Tool %s returned nil result", req.Name)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Tool returned nil result",
					"tool":    req.Name,
					"message": "Failed to call tool",
				})
				return
			}

			c.JSON(http.StatusOK, result)
		})

		// 远程 MCP 服务配置
		apiGroup.GET("/config/remote-mcp", func(c *gin.Context) {
			mcps := cfgStore.GetAllRemoteMCPs()
			c.JSON(http.StatusOK, mcps)
		})

		apiGroup.POST("/config/remote-mcp", func(c *gin.Context) {
			var req types.RemoteMCPConfig
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// 检查名称或服务器标识符是否已存在
			allMcps := cfgStore.GetAllRemoteMCPs()
			for _, existing := range allMcps {
				if existing.Name == req.Name {
					c.JSON(http.StatusConflict, gin.H{
						"error": fmt.Sprintf("名称 \"%s\" 已存在", req.Name),
					})
					return
				}
				if existing.ServerID == req.ServerID && req.ServerID != "" {
					c.JSON(http.StatusConflict, gin.H{
						"error": fmt.Sprintf("服务器标识符 \"%s\" 已存在", req.ServerID),
					})
					return
				}
			}

			// 使用持久化存储（会自动保存到文件）
			cfgStore.SetRemoteMCP(req)
			// 刷新 ToolManager 以加载新的远程 MCP 工具
			toolManagerMu.Lock()
			if toolManager != nil {
				toolManager.RefreshRemoteTools()
			}
			toolManagerMu.Unlock()
			c.Status(http.StatusNoContent)
		})

		apiGroup.PUT("/config/remote-mcp/:identifier", func(c *gin.Context) {
			identifier := c.Param("identifier")
			var req types.RemoteMCPConfig
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			// 确保使用正确的标识符
			if req.ServerID == "" {
				req.ServerID = identifier
			}
			// 使用持久化存储（会自动保存到文件）
			cfgStore.SetRemoteMCP(req)
			// 刷新 ToolManager 以加载新的远程 MCP 工具
			toolManagerMu.Lock()
			if toolManager != nil {
				toolManager.RefreshRemoteTools()
			}
			toolManagerMu.Unlock()
			c.Status(http.StatusNoContent)
		})

		apiGroup.DELETE("/config/remote-mcp/:identifier", func(c *gin.Context) {
			identifier := c.Param("identifier")
			// 使用持久化存储（会自动保存到文件）
			cfgStore.DeleteRemoteMCP(identifier)
			// 刷新 ToolManager 以移除已删除的远程 MCP 工具
			toolManagerMu.Lock()
			if toolManager != nil {
				toolManager.RefreshRemoteTools()
			}
			toolManagerMu.Unlock()
			c.Status(http.StatusNoContent)
		})

		// 测试端点路径（不保存配置，仅测试）
		apiGroup.POST("/config/remote-mcp/test-endpoint", func(c *gin.Context) {
			var req struct {
				BaseURL       string            `json:"baseUrl"`
				ToolsEndpoint string            `json:"toolsEndpoint"`
				Headers       map[string]string `json:"headers"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// 尝试多个端点路径
			var endpoints []string
			if req.ToolsEndpoint != "" {
				endpoints = []string{req.ToolsEndpoint}
			} else {
				// 基于 baseURL 尝试不同的路径
				basePath := ""
				if strings.Contains(req.BaseURL, "/mcp") {
					basePath = "/mcp"
				}
				endpoints = []string{
					basePath + "/tools",
					basePath + "/api/tools",
					"/api/tools",
					"/tools",
					"/v1/tools",
					"/api/v1/tools",
					"/api/mcp/tools",
				}
			}

			results := []map[string]interface{}{}
			var successEndpoint string
			var successToolsCount int
			var successToolsList []mcp.Tool

			for _, ep := range endpoints {
				client, err := mcpclient.NewRemoteClient(mcpclient.RemoteMCPConfig{
					BaseURL:       req.BaseURL,
					ToolsEndpoint: ep,
					Headers:       req.Headers,
					Timeout:       30,
				})

				if err == nil && client != nil {
					tools := client.ListTools()
					if len(tools) > 0 {
						successEndpoint = ep
						successToolsCount = len(tools)
						successToolsList = tools
						break
					}
					results = append(results, map[string]interface{}{
						"endpoint": ep,
						"status":   "ok",
						"message":  "连接成功，但未找到工具",
					})
				} else {
					results = append(results, map[string]interface{}{
						"endpoint": ep,
						"status":   "failed",
						"message":  err.Error(),
					})
				}
			}

			if successEndpoint != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":    "ok",
					"endpoint":  successEndpoint,
					"tools":     successToolsCount,
					"toolsList": successToolsList,
					"message":   fmt.Sprintf("成功！端点 %s 可用，找到 %d 个工具", successEndpoint, successToolsCount),
					"details":   results,
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"status":  "failed",
					"message": "所有端点测试失败",
					"details": results,
				})
			}
		})

		// 获取远程 MCP 服务的工具列表（带缓存）
		apiGroup.GET("/config/remote-mcp/:identifier/tools", func(c *gin.Context) {
			identifier := c.Param("identifier")
			forceRefresh := c.Query("refresh") == "true" // 支持 ?refresh=true 强制刷新

			mcpConfig := cfgStore.GetRemoteMCP(identifier)
			if mcpConfig == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Remote MCP not found"})
				return
			}

			ctx := c.Request.Context()

			// 1. 检查多级缓存（Redis -> DB，如果不需要强制刷新）
			if !forceRefresh && toolsCache != nil {
				cachedTools, err := toolsCache.GetTools(ctx, identifier)
				if err == nil && len(cachedTools) > 0 {
					c.JSON(http.StatusOK, gin.H{
						"tools":      cachedTools,
						"count":      len(cachedTools),
						"cached":     true,
						"lastUpdate": mcpConfig.ToolsLastUpdate,
					})
					return
				}
				// 缓存未命中且不需要强制刷新，直接返回空列表（不触发远程请求）
				if !forceRefresh {
					c.JSON(http.StatusOK, gin.H{
						"tools":      []interface{}{},
						"count":      0,
						"cached":     false,
						"lastUpdate": mcpConfig.ToolsLastUpdate,
					})
					return
				}
			}

			// 2. 缓存未命中且需要强制刷新，从远程 MCP 获取
			remoteClient, err := mcpclient.NewRemoteClient(mcpclient.RemoteMCPConfig{
				Name:           mcpConfig.Name,
				ServerID:       mcpConfig.ServerID,
				Type:           mcpConfig.Type,
				BaseURL:        mcpConfig.BaseURL,
				Timeout:        mcpConfig.Timeout,
				SSEReadTimeout: mcpConfig.SSEReadTimeout,
				Headers:        mcpConfig.Headers,
				ToolsEndpoint:  mcpConfig.ToolsEndpoint,
			})

			if err != nil {
				// 如果客户端已创建但工具发现失败，仍然返回工具列表（可能为空）
				if remoteClient != nil {
					tools := remoteClient.ListTools()
					// 更新缓存（Redis + 数据库）
					if toolsCache != nil && len(tools) > 0 {
						toolsCache.SetTools(ctx, identifier, tools, 24*time.Hour)
					}
					c.JSON(http.StatusOK, gin.H{
						"tools": tools,
						"count": len(tools),
						"error": err.Error(),
					})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			tools := remoteClient.ListTools()
			// 更新缓存（Redis + 数据库）
			if toolsCache != nil && len(tools) > 0 {
				if err := toolsCache.SetTools(ctx, identifier, tools, 24*time.Hour); err != nil {
					log.Printf("Warning: Failed to update cache: %v", err)
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"tools":      tools,
				"count":      len(tools),
				"cached":     false,
				"lastUpdate": mcpConfig.ToolsLastUpdate,
			})
		})

		apiGroup.POST("/config/remote-mcp/:identifier/test", func(c *gin.Context) {
			identifier := c.Param("identifier")
			mcpConfig := cfgStore.GetRemoteMCP(identifier)
			if mcpConfig == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Remote MCP not found"})
				return
			}

			// 创建远程客户端并测试
			remoteClient, err := mcpclient.NewRemoteClient(mcpclient.RemoteMCPConfig{
				Name:           mcpConfig.Name,
				ServerID:       mcpConfig.ServerID,
				Type:           mcpConfig.Type,
				BaseURL:        mcpConfig.BaseURL,
				Timeout:        mcpConfig.Timeout,
				SSEReadTimeout: mcpConfig.SSEReadTimeout,
				Headers:        mcpConfig.Headers,
				ToolsEndpoint:  mcpConfig.ToolsEndpoint,
			})

			// 即使工具发现失败，也检查连接是否正常
			if err != nil {
				errorMsg := err.Error()

				// 如果客户端已创建（连接成功但工具发现失败）
				if remoteClient != nil {
					tools := remoteClient.ListTools()
					if len(tools) > 0 {
						// 有工具，说明连接成功
						c.JSON(http.StatusOK, gin.H{
							"status":  "ok",
							"message": fmt.Sprintf("Connected successfully. Found %d tools", len(tools)),
							"tools":   tools,
							"count":   len(tools),
						})
						return
					}
					// 连接成功但工具列表为空
					c.JSON(http.StatusOK, gin.H{
						"status":  "partial",
						"message": fmt.Sprintf("Connection OK but tool discovery failed: %s. Please check the tools endpoint path.", errorMsg),
						"tools":   []interface{}{},
						"count":   0,
					})
					return
				}

				// 连接失败
				c.JSON(http.StatusOK, gin.H{
					"status":  "failed",
					"message": errorMsg,
					"tools":   []interface{}{},
					"count":   0,
				})
				return
			}

			// 成功获取工具列表
			tools := remoteClient.ListTools()
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"message": fmt.Sprintf("Connected successfully. Found %d tools", len(tools)),
				"tools":   tools,
				"count":   len(tools),
			})
		})

		// Agent 配置
		apiGroup.GET("/config/agents", func(c *gin.Context) {
			agents := cfgStore.GetAllAgents()
			c.JSON(http.StatusOK, agents)
		})

		apiGroup.POST("/config/agents", func(c *gin.Context) {
			var req struct {
				Name         string `json:"name"`
				Description  string `json:"description"`
				MCPServerID  string `json:"mcpServerId"`
				LLMID        string `json:"llmId"`
				SystemPrompt string `json:"systemPrompt"`
				Enabled      bool   `json:"enabled"`
				IsDefault    bool   `json:"isDefault"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.Name == "" || req.MCPServerID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name and mcpServerId are required"})
				return
			}
			if cfgStore.GetRemoteMCP(req.MCPServerID) == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "MCP server not found"})
				return
			}
			agent := cfgStore.SetAgent(types.AgentConfig{
				Name:         req.Name,
				Description:  req.Description,
				MCPServerID:  req.MCPServerID,
				LLMID:        req.LLMID,
				SystemPrompt: req.SystemPrompt,
				Enabled:      req.Enabled,
				IsDefault:    req.IsDefault,
			})
			c.JSON(http.StatusOK, agent)
		})

		apiGroup.PUT("/config/agents/:id", func(c *gin.Context) {
			agentID := c.Param("id")
			existing := cfgStore.GetAgent(agentID)
			if existing == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
				return
			}

			var req struct {
				Name         string `json:"name"`
				Description  string `json:"description"`
				MCPServerID  string `json:"mcpServerId"`
				LLMID        string `json:"llmId"`
				SystemPrompt string `json:"systemPrompt"`
				Enabled      bool   `json:"enabled"`
				IsDefault    bool   `json:"isDefault"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.Name == "" || req.MCPServerID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name and mcpServerId are required"})
				return
			}
			if cfgStore.GetRemoteMCP(req.MCPServerID) == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "MCP server not found"})
				return
			}

			agent := cfgStore.SetAgent(types.AgentConfig{
				ID:           agentID,
				Name:         req.Name,
				Description:  req.Description,
				MCPServerID:  req.MCPServerID,
				LLMID:        req.LLMID,
				SystemPrompt: req.SystemPrompt,
				Enabled:      req.Enabled,
				IsDefault:    req.IsDefault,
				CreatedAt:    existing.CreatedAt,
			})
			c.JSON(http.StatusOK, agent)
		})

		apiGroup.DELETE("/config/agents/:id", func(c *gin.Context) {
			agentID := c.Param("id")
			if cfgStore.GetAgent(agentID) == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
				return
			}
			cfgStore.DeleteAgent(agentID)
			c.Status(http.StatusNoContent)
		})

		// 对话接口
		apiGroup.POST("/chat", func(c *gin.Context) {
			var req types.ChatRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			agentID := req.AgentID
			var agent *types.AgentConfig
			if agentID != "" {
				agent = cfgStore.GetAgent(agentID)
				if agent == nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "指定的智能体不存在"})
					return
				}
			} else {
				agent = cfgStore.GetDefaultAgent()
				if agent == nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "请先创建并启用智能体"})
					return
				}
			}
			if !agent.Enabled {
				c.JSON(http.StatusBadRequest, gin.H{"error": "当前智能体已被禁用"})
				return
			}
			if agent.MCPServerID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "智能体未关联任何 MCP 服务"})
				return
			}

			// 获取 LLM 配置（始终使用用户配置的默认 LLM）
			llmConfig := cfgStore.GetDefaultLLMConfig()
			if llmConfig == nil {
				log.Printf("[Chat API] ERROR: No default LLM config found")
				c.JSON(http.StatusBadRequest, gin.H{"error": "LLM config not set. Please configure LLM first."})
				return
			}
			// 检查 LLM 配置是否启用
			if !llmConfig.Enabled {
				log.Printf("[Chat API] ERROR: LLM config %s is disabled", llmConfig.ID)
				c.JSON(http.StatusBadRequest, gin.H{"error": "LLM 配置已被禁用，请先启用 LLM 配置才能进行智能对话。"})
				return
			}
			// 验证 LLM 配置
			if llmConfig.APIKey == "" {
				log.Printf("[Chat API] ERROR: LLM config missing APIKey")
				c.JSON(http.StatusBadRequest, gin.H{"error": "LLM API Key is not configured. Please check your LLM configuration."})
				return
			}
			log.Printf("[Chat API] Using configured default LLM: %s (ID: %s, Provider: %s, Model: %s, BaseURL: %s)",
				llmConfig.Name, llmConfig.ID, llmConfig.Provider, llmConfig.Model, llmConfig.BaseURL)

			// 验证 MCP 服务配置
			mcpConfig := cfgStore.GetRemoteMCP(agent.MCPServerID)
			if mcpConfig == nil {
				log.Printf("[Chat API] ERROR: MCP server %s not found for agent %s", agent.MCPServerID, agent.Name)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("MCP 服务 %s 不存在或未启用", agent.MCPServerID)})
				return
			}
			if !mcpConfig.Enabled {
				log.Printf("[Chat API] ERROR: MCP server %s is disabled", agent.MCPServerID)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("MCP 服务 %s 已被禁用", agent.MCPServerID)})
				return
			}
			log.Printf("[Chat API] Agent %s bound to MCP server: %s (BaseURL: %s)", agent.Name, mcpConfig.Name, mcpConfig.BaseURL)

			// 确保工具列表已刷新（在聊天前刷新，确保工具可用）
			refreshStart := time.Now()
			toolManagerMu.Lock()
			if toolManager != nil {
				log.Printf("[Chat API] Refreshing remote tools before chat for agent %s (MCP: %s)", agent.Name, agent.MCPServerID)
				toolManager.RefreshRemoteTools()
				refreshDuration := time.Since(refreshStart)
				log.Printf("[Chat API] Tool refresh completed in %v", refreshDuration)
				if refreshDuration > 10*time.Second {
					log.Printf("[Chat API] WARNING: Tool refresh took too long (%v), this may cause timeout issues", refreshDuration)
				}
				// 立即检查工具列表
				tools := toolManager.ListToolsForAgent(agent)
				if len(tools) == 0 {
					log.Printf("[Chat API] WARNING: No tools found for agent %s after refresh", agent.Name)
				} else {
					log.Printf("[Chat API] Found %d tools for agent %s: %v", len(tools), agent.Name, getToolNamesForLog(tools))
				}
			}
			toolManagerMu.Unlock()

			// 获取或创建 Orchestrator
			orchStart := time.Now()
			orch := getOrchestrator(llmConfig, mcpClient, globalSessionStore)
			orchDuration := time.Since(orchStart)
			if orch == nil {
				log.Printf("[Chat API] ERROR: Failed to create Orchestrator after %v", orchDuration)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize LLM client. Please check LLM configuration."})
				return
			}
			log.Printf("[Chat API] Orchestrator created successfully in %v", orchDuration)

			// 检测并保存 Agent Strategy（如果未设置）
			if agent.Strategy == "" {
				// 检测模型能力
				supportsFC := llm.SupportsFunctionCalling(llmConfig.Provider, llmConfig.Model)
				if supportsFC {
					agent.Strategy = "function_call"
					log.Printf("[Chat API] Agent %s: Model %s supports Function Calling, setting strategy to function_call", 
						agent.Name, llmConfig.Model)
				} else {
					agent.Strategy = "prompt_based"
					log.Printf("[Chat API] Agent %s: Model %s does not support Function Calling, setting strategy to prompt_based", 
						agent.Name, llmConfig.Model)
				}
				
				// 保存 Strategy 到数据库
				agent.UpdatedAt = time.Now().Unix()
				cfgStore.SetAgent(*agent)
				log.Printf("[Chat API] Saved Agent Strategy: %s for agent %s", agent.Strategy, agent.Name)
			} else {
				log.Printf("[Chat API] Agent %s using stored strategy: %s", agent.Name, agent.Strategy)
			}

			// 处理对话（添加超时上下文，10分钟超时，因为 qwen3-max 等大模型可能需要更长时间）
			log.Printf("[Chat API] Processing chat request: agent=%s, session=%s, message=%s, strategy=%s", 
				agent.Name, req.SessionID, req.Message, agent.Strategy)
			ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
			defer cancel()
			response, err := orch.Chat(ctx, req.SessionID, req.Message, agent, llmConfig)
			if err != nil {
				log.Printf("[Chat API] ERROR in chat: %v", err)
				errMsg := err.Error()

				// 检查是否是超时错误
				if ctx.Err() == context.DeadlineExceeded {
					c.JSON(http.StatusRequestTimeout, gin.H{
						"error":   "请求超时",
						"message": errMsg,
						"type":    "timeout",
					})
				} else {
					// 根据错误信息判断错误类型
					errorType := "unknown"
					if strings.Contains(errMsg, "LLM") || strings.Contains(errMsg, "llm") {
						errorType = "llm"
					} else if strings.Contains(errMsg, "工具") || strings.Contains(errMsg, "tool") || strings.Contains(errMsg, "MCP") {
						errorType = "tool"
					} else if strings.Contains(errMsg, "超时") || strings.Contains(errMsg, "timeout") {
						errorType = "timeout"
					}

					c.JSON(http.StatusInternalServerError, gin.H{
						"error":   errMsg,
						"message": errMsg,
						"type":    errorType,
					})
				}
				return
			}

			response.AgentID = agent.ID
			c.JSON(http.StatusOK, response)
		})

		// 获取所有会话列表（支持分页）
		apiGroup.GET("/sessions", func(c *gin.Context) {
			llmConfig := cfgStore.GetDefaultLLMConfig()
			if llmConfig == nil {
				c.JSON(http.StatusOK, gin.H{
					"sessions": []interface{}{},
					"total":    0,
				})
				return
			}

			orch := getOrchestrator(llmConfig, mcpClient, globalSessionStore)
			if orch == nil {
				c.JSON(http.StatusOK, gin.H{
					"sessions": []interface{}{},
					"total":    0,
				})
				return
			}

			agentID := c.Query("agentId")
			limit := 50 // 默认每页50条
			skip := 0
			if limitStr := c.Query("limit"); limitStr != "" {
				fmt.Sscanf(limitStr, "%d", &limit)
			}
			if skipStr := c.Query("skip"); skipStr != "" {
				fmt.Sscanf(skipStr, "%d", &skip)
			}

			// 获取会话列表（只返回元数据，不包含消息）
			// 添加超时上下文，避免长时间等待
			ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
			defer cancel()
			sessions, total, err := orch.GetSessions(ctx, agentID, limit, skip)
			if err != nil {
				log.Printf("ERROR getting sessions: %v", err)
				// 返回空列表而不是错误，避免前端卡住
				c.JSON(http.StatusOK, gin.H{
					"sessions": []interface{}{},
					"total":    0,
					"limit":    limit,
					"skip":     skip,
				})
				return
			}

			// sessions 是 []*store.SessionMeta 类型
			sessionList := make([]map[string]interface{}, 0, len(sessions))
			for _, sessionMeta := range sessions {
				sessionList = append(sessionList, map[string]interface{}{
					"id":           sessionMeta.ID,
					"agentId":      sessionMeta.AgentID,
					"title":        sessionMeta.Title,
					"createdAt":    sessionMeta.CreatedAt.Unix(),
					"updatedAt":    sessionMeta.UpdatedAt.Unix(),
					"messageCount": sessionMeta.MessageCount,
				})
			}

			c.JSON(http.StatusOK, gin.H{
				"sessions": sessionList,
				"total":    total,
				"limit":    limit,
				"skip":     skip,
			})
		})

		// 获取会话详情
		apiGroup.GET("/sessions/:sessionId", func(c *gin.Context) {
			sessionID := c.Param("sessionId")
			if sessionID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
				return
			}

			llmConfig := cfgStore.GetDefaultLLMConfig()
			if llmConfig == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "LLM config not set"})
				return
			}

			orch := getOrchestrator(llmConfig, mcpClient, globalSessionStore)
			if orch == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize orchestrator"})
				return
			}

			session, err := orch.GetSession(c.Request.Context(), sessionID)
			if err != nil {
				log.Printf("ERROR getting session: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if session == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
				return
			}

			// 转换为前端格式
			messages := make([]map[string]interface{}, 0, len(session.Messages))
			for _, msg := range session.Messages {
				messages = append(messages, map[string]interface{}{
					"role":    msg.Role,
					"content": msg.Content,
				})
			}

			c.JSON(http.StatusOK, map[string]interface{}{
				"id":        session.ID,
				"agentId":   session.AgentID,
				"messages":  messages,
				"createdAt": session.CreatedAt.Unix(),
				"updatedAt": session.UpdatedAt.Unix(),
			})
		})

		// 获取当前默认 LLM 信息
		apiGroup.GET("/current-llm", func(c *gin.Context) {
			llmConfig := cfgStore.GetDefaultLLMConfig()
			if llmConfig == nil {
				c.JSON(http.StatusOK, gin.H{
					"configured": false,
					"message":    "未配置 LLM",
				})
				return
			}

			llmName := llmConfig.Name
			if llmName == "" {
				llmName = llmConfig.Provider
			}

			c.JSON(http.StatusOK, gin.H{
				"configured": true,
				"id":         llmConfig.ID,
				"name":       llmName,
				"provider":   llmConfig.Provider,
				"model":      llmConfig.Model,
				"enabled":    llmConfig.Enabled,
			})
		})

		// 状态查询
		apiGroup.GET("/status", func(c *gin.Context) {
			toolsCount := 0
			if mcpClient != nil {
				toolsCount = len(mcpClient.ListTools())
			}
			
			// 检查是否有启用的 K8s 配置
			enabledK8sConfig := cfgStore.GetDefaultK8sConfig()
			k8sEnabled := enabledK8sConfig != nil
			k8sConnected := k8sClient != nil && k8sEnabled
			
			// 获取 API 调用统计
			apiStats := getAPIStats()
			
			status := map[string]interface{}{
				"k8s": map[string]interface{}{
					"connected": k8sConnected,
					"enabled":   k8sEnabled,
				},
				"llm": map[string]interface{}{
					"configured": cfgStore.GetDefaultLLMConfig() != nil,
				},
				"mcp": map[string]interface{}{
					"tools": toolsCount,
				},
				"api": apiStats,
			}
			c.JSON(http.StatusOK, status)
		})

		// 日志查看
		apiGroup.GET("/logs", func(c *gin.Context) {
			// 获取查询参数
			linesStr := c.DefaultQuery("lines", "100") // 默认返回最后100行
			lines := 100
			if n, err := fmt.Sscanf(linesStr, "%d", &lines); err == nil && n == 1 {
				if lines > 1000 {
					lines = 1000 // 最多返回1000行
				}
				if lines < 1 {
					lines = 1
				}
			}

			filter := c.Query("filter") // 可选的过滤关键词

			// 读取日志文件（优先使用当前目录，如果不存在则尝试项目根目录）
			logFile := "service.log"
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				// 尝试项目根目录
				if cwd, err := os.Getwd(); err == nil {
					// 如果当前在子目录，尝试上级目录
					possiblePaths := []string{
						filepath.Join(cwd, "service.log"),
						filepath.Join(filepath.Dir(cwd), "service.log"),
						"./service.log",
					}
					for _, path := range possiblePaths {
						if _, err := os.Stat(path); err == nil {
							logFile = path
							break
						}
					}
				}
			}
			file, err := os.Open(logFile)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("无法打开日志文件: %v", err),
				})
				return
			}
			defer file.Close()

			// 读取文件内容
			content, err := os.ReadFile(logFile)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("无法读取日志文件: %v", err),
				})
				return
			}

			// 按行分割
			allLines := strings.Split(string(content), "\n")
			
			// 应用过滤
			var filteredLines []string
			if filter != "" {
				filterLower := strings.ToLower(filter)
				for _, line := range allLines {
					if strings.Contains(strings.ToLower(line), filterLower) {
						filteredLines = append(filteredLines, line)
					}
				}
			} else {
				filteredLines = allLines
			}

			// 获取最后 N 行
			start := 0
			if len(filteredLines) > lines {
				start = len(filteredLines) - lines
			}
			resultLines := filteredLines[start:]

			c.JSON(http.StatusOK, gin.H{
				"lines":  resultLines,
				"total":  len(allLines),
				"filtered": len(filteredLines),
				"showing": len(resultLines),
			})
		})
	}

	// 静态文件服务（必须在 API 路由之后）
	staticDir := "./static"
	r.Static("/assets", filepath.Join(staticDir, "assets"))
	r.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))

	// SPA 路由：所有非 API 请求返回 index.html（必须在最后）
	r.NoRoute(func(c *gin.Context) {
		// 如果是 API 请求，返回 404
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}
		// 否则返回前端页面
		c.File(filepath.Join(staticDir, "index.html"))
	})

	return r
}

// getOrchestrator 获取或创建 Orchestrator
func getOrchestrator(llmConfig *types.LLMConfig, mcpClient *mcpclient.Client, sessionStore store.SessionStore) *chat.Orchestrator {
	orchestratorMu.Lock()
	defer orchestratorMu.Unlock()

	// 注意：不再在这里刷新工具，因为已经在 Chat API 中刷新过了
	// 避免重复刷新导致延迟
	toolManagerMu.RLock()
	tm := toolManager
	toolManagerMu.RUnlock()

	if tm == nil {
		log.Printf("ERROR: ToolManager not initialized")
		return nil
	}

	// 如果 Orchestrator 已存在且配置未变化，直接返回
	if orchestrator != nil {
		log.Printf("[getOrchestrator] Reusing existing orchestrator")
		return orchestrator
	}

	// 创建 LLM 客户端
	log.Printf("[getOrchestrator] Creating new LLM client: Provider=%s, Model=%s", llmConfig.Provider, llmConfig.Model)
	llmClient, err := llm.NewClient(llmConfig)
	if err != nil {
		log.Printf("ERROR: Failed to create LLM client: %v", err)
		return nil
	}
	log.Printf("[getOrchestrator] LLM client created successfully")

	// 创建 Orchestrator（使用 ToolManager 和 SessionStore）
	orchestrator = chat.NewOrchestrator(llmClient, tm, sessionStore)
	log.Printf("[getOrchestrator] Orchestrator created successfully")
	return orchestrator
}

// ResetOrchestrator 重置 Orchestrator（当配置更新时调用）
func ResetOrchestrator() {
	orchestratorMu.Lock()
	defer orchestratorMu.Unlock()
	orchestrator = nil
}

// getToolNamesForLog 获取工具名称列表（用于日志）
func getToolNamesForLog(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

// ensureDefaultAgent 确保至少存在一个默认 Agent（优先使用 kubernetes-mcp-server）
func ensureDefaultAgent(cfgStore *store.PersistentStore) {
	agents := cfgStore.GetAllAgents()
	if len(agents) > 0 {
		return
	}

	remoteMcps := cfgStore.GetRemoteMCPs()
	if len(remoteMcps) == 0 {
		return
	}

	var target *types.RemoteMCPConfig
	for _, mcp := range remoteMcps {
		if mcp.ServerID == "kubernetes-mcp-server" {
			target = &mcp
			break
		}
	}
	if target == nil {
		target = &remoteMcps[0]
	}

	cfgStore.SetAgent(types.AgentConfig{
		Name:        fmt.Sprintf("%s Agent", target.Name),
		Description: fmt.Sprintf("自动为 %s 创建的默认智能体", target.Name),
		MCPServerID: target.ServerID,
		Enabled:     true,
		IsDefault:   true,
	})
}
