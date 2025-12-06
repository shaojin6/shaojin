# Worker 模块实施计划

## 一、实施步骤

### 步骤1：创建 Worker 模块（已完成）
- ✅ `internal/web/worker/worker.go` - 核心 Worker 实现
- ✅ `internal/web/worker/tasks/tool_refresh_task.go` - 工具刷新任务

### 步骤2：集成到现有代码

#### 2.1 在 router.go 中初始化 Worker

```go
import (
    "github.com/your-org/k8s-mcp-agent/internal/web/worker"
    "github.com/your-org/k8s-mcp-agent/internal/web/worker/tasks"
)

func SetupRouter(...) {
    // ... 现有代码 ...
    
    // 初始化 Worker（5个工作线程，队列大小100）
    worker.InitWorker(5, 100)
    // 注意：在服务关闭时调用 worker.StopWorker()
    
    // ... 现有代码 ...
}
```

#### 2.2 替换现有的 go func() 调用

**替换位置1：POST /config/remote-mcp**
```go
// 原来的代码：
go func() {
    toolManagerMu.Lock()
    if toolManager != nil {
        toolManager.RefreshRemoteTools()
    }
    toolManagerMu.Unlock()
}()

// 替换为：
task := tasks.NewToolRefreshTask(&toolManagerMu, toolManager, "")
if err := worker.EnqueueTask(task); err != nil {
    log.Printf("Failed to enqueue tool refresh task: %v", err)
}
```

**替换位置2：PUT /config/remote-mcp/:identifier**
```go
// 原来的代码：
go func() {
    toolManagerMu.Lock()
    if toolManager != nil {
        toolManager.RefreshRemoteTools()
    }
    toolManagerMu.Unlock()
}()

// 替换为：
task := tasks.NewToolRefreshTask(&toolManagerMu, toolManager, req.ServerID)
if err := worker.EnqueueTask(task); err != nil {
    log.Printf("Failed to enqueue tool refresh task: %v", err)
}
```

**替换位置3：DELETE /config/remote-mcp/:identifier**
```go
// 原来的代码：
go func() {
    toolManagerMu.Lock()
    if toolManager != nil {
        toolManager.RefreshRemoteTools()
    }
    toolManagerMu.Unlock()
}()

// 替换为：
task := tasks.NewToolRefreshTask(&toolManagerMu, toolManager, "")
if err := worker.EnqueueTask(task); err != nil {
    log.Printf("Failed to enqueue tool refresh task: %v", err)
}
```

### 步骤3：添加优雅关闭

在服务关闭时停止 Worker：

```go
// 在 main.go 或服务关闭逻辑中
func gracefulShutdown() {
    // ... 其他关闭逻辑 ...
    worker.StopWorker()
}
```

## 二、配置建议

### Worker 参数配置

```go
// 从环境变量读取配置
workers := 5
if w := os.Getenv("WORKER_WORKERS"); w != "" {
    if n, err := strconv.Atoi(w); err == nil && n > 0 {
        workers = n
    }
}

queueSize := 100
if q := os.Getenv("WORKER_QUEUE_SIZE"); q != "" {
    if n, err := strconv.Atoi(q); err == nil && n > 0 {
        queueSize = n
    }
}

worker.InitWorker(workers, queueSize)
```

### 环境变量

```bash
# .env 或环境变量
WORKER_WORKERS=5          # Worker 线程数
WORKER_QUEUE_SIZE=100     # 任务队列大小
```

## 三、监控和日志

### 添加任务统计

```go
// internal/web/worker/stats.go
package worker

import (
    "sync/atomic"
    "time"
)

type Stats struct {
    TotalEnqueued int64
    TotalCompleted int64
    TotalFailed int64
    QueueLength int64
}

var stats Stats

func (w *Worker) GetStats() Stats {
    return Stats{
        TotalEnqueued: atomic.LoadInt64(&stats.TotalEnqueued),
        TotalCompleted: atomic.LoadInt64(&stats.TotalCompleted),
        TotalFailed: atomic.LoadInt64(&stats.TotalFailed),
        QueueLength: int64(len(w.taskQueue)),
    }
}
```

### 添加监控端点

```go
// 在 router.go 中添加
apiGroup.GET("/worker/stats", func(c *gin.Context) {
    if globalWorker != nil {
        stats := globalWorker.GetStats()
        c.JSON(http.StatusOK, stats)
    } else {
        c.JSON(http.StatusOK, gin.H{"status": "worker not initialized"})
    }
})
```

## 四、测试建议

### 单元测试

```go
// internal/web/worker/worker_test.go
func TestWorker(t *testing.T) {
    w := NewWorker(2, 10)
    w.Start()
    defer w.Stop()
    
    task := &TestTask{name: "test-task"}
    err := w.Enqueue(task)
    assert.NoError(t, err)
    
    // 等待任务完成
    time.Sleep(time.Second)
}
```

### 集成测试

测试 Worker 在真实场景下的表现：
1. 并发保存多个 MCP 配置
2. 观察任务队列长度
3. 验证任务是否正常执行

## 五、未来扩展

### 1. 添加更多任务类型

```go
// internal/web/worker/tasks/cache_refill_task.go
type CacheRefillTask struct {
    // 缓存回填任务
}

// internal/web/worker/tasks/notification_task.go
type NotificationTask struct {
    // 通知任务
}
```

### 2. 任务优先级

```go
type PriorityTask interface {
    Task
    Priority() int // 0-9, 数字越大优先级越高
}
```

### 3. 任务调度

```go
// 定期执行的任务
type ScheduledTask interface {
    Task
    Schedule() time.Duration
}
```

## 六、注意事项

1. **任务幂等性**：确保任务可以安全重试
2. **资源限制**：避免过多并发任务
3. **错误处理**：合理设置重试次数
4. **监控告警**：监控队列长度和处理时间
5. **优雅关闭**：确保任务完成后才关闭

## 七、性能考虑

### 当前方案（简单 Worker）
- **优点**：无额外开销，性能好
- **缺点**：无法跨进程，重启丢失任务

### 未来方案（消息队列）
- **优点**：可跨进程，任务持久化
- **缺点**：需要网络通信，有一定延迟

### 建议
- 当前阶段使用简单 Worker
- 当需要微服务拆分时，再引入消息队列
- 保持接口一致性，便于迁移

