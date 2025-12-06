package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	// 重连配置
	maxReconnectAttempts = 3              // 最大重连次数
	reconnectInterval    = 1 * time.Minute // 重连间隔（每分钟探测一次）
	pingTimeout          = 5 * time.Second // Ping 超时时间
	initialPingTimeout   = 5 * time.Second // 初始连接 Ping 超时
)

// MySQLConnectionManager MySQL 连接管理器（单例模式，所有模块共享）
type MySQLConnectionManager struct {
	dsn           string
	db            *sql.DB
	mu            sync.RWMutex
	isConnected   bool
	reconnectMu   sync.Mutex
	reconnectStop chan struct{}
	stopOnce      sync.Once
}

var (
	mysqlManagerInstance *MySQLConnectionManager
	mysqlManagerOnce     sync.Once
)

// GetMySQLConnectionManager 获取 MySQL 连接管理器单例
func GetMySQLConnectionManager(dsn string) *MySQLConnectionManager {
	mysqlManagerOnce.Do(func() {
		mysqlManagerInstance = &MySQLConnectionManager{
			dsn:           dsn,
			isConnected:   false,
			reconnectStop: make(chan struct{}),
		}
	})
	return mysqlManagerInstance
}

// GetDB 获取数据库连接（线程安全）
func (m *MySQLConnectionManager) GetDB() (*sql.DB, error) {
	m.mu.RLock()
	if m.isConnected && m.db != nil {
		// 快速检查连接是否有效
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := m.db.PingContext(ctx)
		cancel()
		if err == nil {
			m.mu.RUnlock()
			return m.db, nil
		}
		// 连接失效，标记为未连接
		m.mu.RUnlock()
		m.mu.Lock()
		m.isConnected = false
		m.mu.Unlock()
	} else {
		m.mu.RUnlock()
	}

	// 尝试重连
	return m.ensureConnection()
}

// ensureConnection 确保连接可用（带重连机制）
func (m *MySQLConnectionManager) ensureConnection() (*sql.DB, error) {
	m.reconnectMu.Lock()
	defer m.reconnectMu.Unlock()

	// 再次检查（双重检查锁定）
	m.mu.RLock()
	if m.isConnected && m.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := m.db.PingContext(ctx)
		cancel()
		if err == nil {
			m.mu.RUnlock()
			return m.db, nil
		}
		m.mu.RUnlock()
	} else {
		m.mu.RUnlock()
	}

	// 尝试连接
	return m.connect()
}

// connect 建立连接
func (m *MySQLConnectionManager) connect() (*sql.DB, error) {
	log.Printf("[MySQLManager] Attempting to connect to MySQL...")

	db, err := sql.Open("mysql", m.dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// 设置连接池参数
	db.SetConnMaxLifetime(30 * time.Minute) // 连接最大生存时间（保持长连接）
	db.SetMaxIdleConns(5)                   // 最大空闲连接数
	db.SetMaxOpenConns(20)                  // 最大打开连接数

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), initialPingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	// 更新状态
	m.mu.Lock()
	m.db = db
	m.isConnected = true
	m.mu.Unlock()

	log.Printf("[MySQLManager] MySQL connection established successfully")
	return db, nil
}

// StartReconnectLoop 启动重连循环（后台 goroutine）
func (m *MySQLConnectionManager) StartReconnectLoop() {
	go m.reconnectLoop()
}

// reconnectLoop 重连循环（每分钟探测一次）
func (m *MySQLConnectionManager) reconnectLoop() {
	ticker := time.NewTicker(reconnectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndReconnect()
		case <-m.reconnectStop:
			log.Printf("[MySQLManager] Reconnect loop stopped")
			return
		}
	}
}

// checkAndReconnect 检查连接并重连
func (m *MySQLConnectionManager) checkAndReconnect() {
	m.mu.RLock()
	connected := m.isConnected
	db := m.db
	m.mu.RUnlock()

	// 如果已连接，检查连接是否有效
	if connected && db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		err := db.PingContext(ctx)
		cancel()
		if err == nil {
			// 连接正常，无需重连
			return
		}
		log.Printf("[MySQLManager] Connection check failed: %v, attempting to reconnect...", err)
	}

	// 连接失效或未连接，尝试重连
	m.reconnectMu.Lock()
	defer m.reconnectMu.Unlock()

	// 再次检查（避免并发重连）
	m.mu.RLock()
	if m.isConnected && m.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := m.db.PingContext(ctx)
		cancel()
		if err == nil {
			m.mu.RUnlock()
			return
		}
	}
	m.mu.RUnlock()

	// 关闭旧连接
	m.mu.Lock()
	if m.db != nil {
		m.db.Close()
		m.db = nil
	}
	m.isConnected = false
	m.mu.Unlock()

	// 尝试重连（最多重试 maxReconnectAttempts 次）
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		log.Printf("[MySQLManager] Reconnect attempt %d/%d", attempt, maxReconnectAttempts)
		
		_, err := m.connect()
		if err == nil {
			log.Printf("[MySQLManager] Reconnected successfully on attempt %d", attempt)
			return
		}

		log.Printf("[MySQLManager] Reconnect attempt %d failed: %v", attempt, err)
		
		// 如果不是最后一次尝试，等待一段时间再重试
		if attempt < maxReconnectAttempts {
			time.Sleep(5 * time.Second)
		}
	}

	log.Printf("[MySQLManager] Failed to reconnect after %d attempts, will retry in next cycle", maxReconnectAttempts)
}

// IsConnected 检查是否已连接
func (m *MySQLConnectionManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected && m.db != nil
}

// Close 关闭连接管理器
func (m *MySQLConnectionManager) Close() error {
	m.stopOnce.Do(func() {
		close(m.reconnectStop)
	})

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db != nil {
		err := m.db.Close()
		m.db = nil
		m.isConnected = false
		return err
	}

	return nil
}

// Ping 测试连接
func (m *MySQLConnectionManager) Ping(ctx context.Context) error {
	m.mu.RLock()
	db := m.db
	connected := m.isConnected
	m.mu.RUnlock()

	if !connected || db == nil {
		return fmt.Errorf("MySQL connection not available")
	}

	return db.PingContext(ctx)
}

