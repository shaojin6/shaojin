package worker

import (
	"context"
	"errors"
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
	taskQueue  chan Task
	workers    int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	maxRetries int
	retryDelay time.Duration
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
	maxRetries := task.RetryCount()
	if maxRetries <= 0 {
		maxRetries = w.maxRetries
	}
	retries := 0

	for retries <= maxRetries {
		err = task.Execute(w.ctx)
		if err == nil {
			duration := time.Since(startTime)
			log.Printf("[Worker] Task %s completed in %v (worker %d)",
				task.Name(), duration, workerID)
			return
		}

		retries++
		if retries <= maxRetries {
			log.Printf("[Worker] Task %s failed (attempt %d/%d): %v, retrying in %v",
				task.Name(), retries, maxRetries, err, w.retryDelay)
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

// GetStats 获取 Worker 统计信息
func GetStats() map[string]interface{} {
	if globalWorker == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}
	return map[string]interface{}{
		"status":       "running",
		"workers":      globalWorker.workers,
		"queue_size":   cap(globalWorker.taskQueue),
		"queue_length": len(globalWorker.taskQueue),
	}
}

