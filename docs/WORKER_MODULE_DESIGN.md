# Worker 模块设计方案

## 一、需求分析

### 当前异步操作场景

1. **工具刷新任务** (`RefreshRemoteTools`)
   - 触发时机：添加/更新/删除 MCP 配置
   - 特点：可能耗时较长（SSE 连接可能需要几分钟）
   - 频率：中等（用户操作触发）

2. **缓存回填任务** (Redis 缓存回填)
   - 触发时机：从 MySQL 加载工具后
   - 特点：轻量级，但需要异步执行
   - 频率：高（每次工具加载）

3. **数据库重连任务** (MySQL 重连循环)
   - 触发时机：连接断开后
   - 特点：后台持续运行
   - 频率：低（异常情况）

4. **未来可能的异步任务**
   - 工具列表定期刷新
   - 配置变更通知
   - 日志聚合
   - 指标收集

## 二、架构设计

### 方案对比

#### 方案1：简单 Worker 模块（推荐用于当前阶段）

**优点：**
- 实现简单，无需额外依赖
- 适合单体应用
- 易于维护和调试

**缺点：**
- 无法跨进程/跨机器
- 任务状态难以追踪
- 重启后任务丢失

**适用场景：**
- 当前单体部署
- 任务不需要持久化
- 任务失败可接受

#### 方案2：基于消息队列的 Worker（推荐用于微服务阶段）

**优点：**
- 可独立部署为微服务
- 任务持久化，重启不丢失
- 支持任务重试和优先级
- 易于水平扩展

**缺点：**
- 需要消息队列中间件（Redis/RabbitMQ/Kafka）
- 系统复杂度增加
- 需要处理消息序列化

**适用场景：**
- 微服务架构
- 需要任务持久化
- 需要任务状态追踪
- 需要高可用性

#### 方案3：混合方案（推荐用于过渡阶段）

**优点：**
- 当前使用简单方案，预留消息队列接口
- 平滑过渡到微服务架构
- 灵活性高

**缺点：**
- 需要维护两套代码
- 迁移需要一定工作量

## 三、推荐实现方案

### 阶段1：简单 Worker 模块（当前）

```go
// internal/web/worker/worker.go
package worker

import (
    "context"
    "log"
    "sync"
    "time"
)

// Task 任务接口
type Task interface {
    Execute(ctx context.Context) error
    Name() string
    RetryCount() int
}

// Worker 工作器
type Worker struct {
    taskQueue    chan Task
    workers      int
    wg           sync.WaitGroup
    ctx          context.Context
    cancel       context.CancelFunc
    maxRetries   int
    retryDelay   time.Duration
}

// NewWorker 创建新的工作器
func NewWorker(workers int, queueSize int) *Worker {
    ctx, cancel := context.WithCancel(context.Background())
    return &Worker{
        taskQueue:  make(chan Task, queueSize),
        workers:    workers,
        ctx:        ctx,
        cancel:     cancel,
        maxRetries: 3,
        retryDelay: time.Second * 5,
    }
}

// Start 启动工作器
func (w *Worker) Start() {
    for i := 0; i < w.workers; i++ {
        w.wg.Add(1)
        go w.workerLoop(i)
    }
    log.Printf("[Worker] Started %d workers", w.workers)
}

// Stop 停止工作器
func (w *Worker) Stop() {
    close(w.taskQueue)
    w.cancel()
    w.wg.Wait()
    log.Printf("[Worker] Stopped")
}

// Enqueue 添加任务到队列
func (w *Worker) Enqueue(task Task) error {
    select {
    case w.taskQueue <- task:
        log.Printf("[Worker] Enqueued task: %s", task.Name())
        return nil
    case <-w.ctx.Done():
        return w.ctx.Err()
    default:
        log.Printf("[Worker] WARNING: Task queue full, dropping task: %s", task.Name())
        return ErrQueueFull
    }
}

// workerLoop 工作循环
func (w *Worker) workerLoop(id int) {
    defer w.wg.Done()
    log.Printf("[Worker] Worker %d started", id)
    
    for {
        select {
        case task, ok := <-w.taskQueue:
            if !ok {
                log.Printf("[Worker] Worker %d stopped", id)
                return
            }
            w.executeTask(task, id)
        case <-w.ctx.Done():
            log.Printf("[Worker] Worker %d stopped (context cancelled)", id)
            return
        }
    }
}

// executeTask 执行任务
func (w *Worker) executeTask(task Task, workerID int) {
    startTime := time.Now()
    log.Printf("[Worker] Worker %d executing task: %s", workerID, task.Name())
    
    var err error
    retries := 0
    
    for retries <= w.maxRetries {
        err = task.Execute(w.ctx)
        if err == nil {
            duration := time.Since(startTime)
            log.Printf("[Worker] Task %s completed in %v (worker %d)", 
                task.Name(), duration, workerID)
            return
        }
        
        retries++
        if retries <= w.maxRetries {
            log.Printf("[Worker] Task %s failed (attempt %d/%d): %v, retrying in %v", 
                task.Name(), retries, w.maxRetries, err, w.retryDelay)
            time.Sleep(w.retryDelay)
        }
    }
    
    duration := time.Since(startTime)
    log.Printf("[Worker] Task %s failed after %d retries in %v (worker %d): %v", 
        task.Name(), retries-1, duration, workerID, err)
}

var (
    ErrQueueFull = errors.New("task queue is full")
)

// 全局工作器实例
var globalWorker *Worker

// InitWorker 初始化全局工作器
func InitWorker(workers int, queueSize int) {
    globalWorker = NewWorker(workers, queueSize)
    globalWorker.Start()
}

// EnqueueTask 添加任务（便捷方法）
func EnqueueTask(task Task) error {
    if globalWorker == nil {
        return errors.New("worker not initialized")
    }
    return globalWorker.Enqueue(task)
}

// StopWorker 停止全局工作器
func StopWorker() {
    if globalWorker != nil {
        globalWorker.Stop()
    }
}
```

### 阶段2：任务实现示例

```go
// internal/web/worker/tasks/tool_refresh_task.go
package tasks

import (
    "context"
    "fmt"
    "github.com/your-org/k8s-mcp-agent/internal/web/mcpclient"
)

// ToolRefreshTask 工具刷新任务
type ToolRefreshTask struct {
    toolManager *mcpclient.ToolManager
    identifier  string // 可选：只刷新特定服务
}

func NewToolRefreshTask(toolManager *mcpclient.ToolManager, identifier string) *ToolRefreshTask {
    return &ToolRefreshTask{
        toolManager: toolManager,
        identifier:  identifier,
    }
}

func (t *ToolRefreshTask) Name() string {
    if t.identifier != "" {
        return fmt.Sprintf("tool-refresh-%s", t.identifier)
    }
    return "tool-refresh-all"
}

func (t *ToolRefreshTask) Execute(ctx context.Context) error {
    if t.identifier != "" {
        // 刷新特定服务
        return t.toolManager.RefreshRemoteToolsForService(ctx, t.identifier)
    }
    // 刷新所有服务
    t.toolManager.RefreshRemoteTools()
    return nil
}

func (t *ToolRefreshTask) RetryCount() int {
    return 2 // 工具刷新失败可以重试2次
}
```

### 阶段3：消息队列接口（为微服务做准备）

```go
// internal/web/worker/queue/queue.go
package queue

// Queue 消息队列接口
type Queue interface {
    // Enqueue 添加任务
    Enqueue(taskType string, payload []byte) error
    
    // Dequeue 获取任务
    Dequeue() (taskType string, payload []byte, err error)
    
    // Ack 确认任务完成
    Ack(id string) error
    
    // Nack 任务失败
    Nack(id string) error
}

// RedisQueue Redis 实现的队列
type RedisQueue struct {
    // 实现细节
}

// RabbitMQQueue RabbitMQ 实现的队列
type RabbitMQQueue struct {
    // 实现细节
}
```

## 四、使用示例

### 在 router.go 中使用

```go
// 初始化
import "github.com/your-org/k8s-mcp-agent/internal/web/worker"
import "github.com/your-org/k8s-mcp-agent/internal/web/worker/tasks"

// 在 SetupRouter 中初始化
func SetupRouter(...) {
    // 初始化 worker（5个工作线程，队列大小100）
    worker.InitWorker(5, 100)
    defer worker.StopWorker()
    
    // ...
    
    // 替换原来的 go func()
    apiGroup.PUT("/config/remote-mcp/:identifier", func(c *gin.Context) {
        // ... 保存配置 ...
        cfgStore.SetRemoteMCP(req)
        
        // 使用 worker 异步刷新工具
        task := tasks.NewToolRefreshTask(toolManager, req.ServerID)
        if err := worker.EnqueueTask(task); err != nil {
            log.Printf("Failed to enqueue tool refresh task: %v", err)
        }
        
        c.Status(http.StatusNoContent)
    })
}
```

## 五、微服务部署方案

### 独立 Worker 服务

```go
// cmd/worker/main.go
package main

import (
    "github.com/your-org/k8s-mcp-agent/internal/web/worker"
    "github.com/your-org/k8s-mcp-agent/internal/web/worker/queue"
)

func main() {
    // 从环境变量读取配置
    queueType := os.Getenv("QUEUE_TYPE") // redis, rabbitmq, etc.
    
    var q queue.Queue
    switch queueType {
    case "redis":
        q = queue.NewRedisQueue(...)
    case "rabbitmq":
        q = queue.NewRabbitMQQueue(...)
    }
    
    // 启动 worker 服务
    workerService := worker.NewService(q)
    workerService.Start()
    
    // 优雅关闭
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
    <-signalChan
    workerService.Stop()
}
```

### API 服务发送任务

```go
// 在 API 服务中
apiGroup.PUT("/config/remote-mcp/:identifier", func(c *gin.Context) {
    // ... 保存配置 ...
    
    // 发送任务到消息队列
    task := &tasks.ToolRefreshTask{
        ServerID: req.ServerID,
    }
    payload, _ := json.Marshal(task)
    queue.Enqueue("tool-refresh", payload)
    
    c.Status(http.StatusNoContent)
})
```

## 六、优势总结

### 当前阶段（单体应用）

1. **代码组织**：所有异步逻辑集中管理
2. **可维护性**：统一的错误处理和日志
3. **可扩展性**：易于添加新任务类型
4. **资源控制**：可以限制并发数量

### 微服务阶段

1. **独立部署**：Worker 服务可以独立扩展
2. **高可用**：任务持久化，服务重启不丢失
3. **监控追踪**：可以追踪任务状态和性能
4. **负载均衡**：多个 Worker 实例处理任务

## 七、实施建议

### 第一阶段（当前）
1. 实现简单 Worker 模块
2. 迁移现有的 `go func()` 到 Worker
3. 添加任务监控和日志

### 第二阶段（准备微服务）
1. 定义消息队列接口
2. 实现 Redis 队列（利用现有 Redis）
3. 保持简单 Worker 作为 fallback

### 第三阶段（微服务）
1. 独立部署 Worker 服务
2. API 服务通过消息队列发送任务
3. 实现任务状态追踪和重试机制

## 八、注意事项

1. **任务幂等性**：确保任务可以安全重试
2. **资源限制**：避免过多并发任务导致资源耗尽
3. **错误处理**：合理设置重试次数和延迟
4. **监控告警**：监控任务队列长度和处理时间
5. **优雅关闭**：确保任务完成后才关闭服务

