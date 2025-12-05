package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// Cache 缓存接口
type Cache interface {
	// GetTools 获取工具列表（从缓存）
	GetTools(ctx context.Context, identifier string) ([]mcp.Tool, error)
	// SetTools 设置工具列表（写入缓存）
	SetTools(ctx context.Context, identifier string, tools []mcp.Tool, ttl time.Duration) error
	// DeleteTools 删除工具列表缓存
	DeleteTools(ctx context.Context, identifier string) error
	// IsAvailable 检查缓存是否可用
	IsAvailable() bool
}

// RedisCache Redis 缓存实现
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisCache 创建 Redis 缓存
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	// 测试连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: rdb,
		ctx:    ctx,
	}, nil
}

// GetTools 从 Redis 获取工具列表
func (r *RedisCache) GetTools(ctx context.Context, identifier string) ([]mcp.Tool, error) {
	key := fmt.Sprintf("mcp:tools:%s", identifier)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // 缓存不存在，返回 nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from Redis: %w", err)
	}

	var tools []mcp.Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools: %w", err)
	}

	return tools, nil
}

// SetTools 将工具列表写入 Redis
func (r *RedisCache) SetTools(ctx context.Context, identifier string, tools []mcp.Tool, ttl time.Duration) error {
	key := fmt.Sprintf("mcp:tools:%s", identifier)
	data, err := json.Marshal(tools)
	if err != nil {
		return fmt.Errorf("failed to marshal tools: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set to Redis: %w", err)
	}

	return nil
}

// DeleteTools 删除工具列表缓存
func (r *RedisCache) DeleteTools(ctx context.Context, identifier string) error {
	key := fmt.Sprintf("mcp:tools:%s", identifier)
	return r.client.Del(ctx, key).Err()
}

// IsAvailable 检查 Redis 是否可用
func (r *RedisCache) IsAvailable() bool {
	if r.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err() == nil
}

// MultiLevelCache 多级缓存（Redis -> 数据库降级）
type MultiLevelCache struct {
	redisCache Cache
	dbCache    Cache // 数据库缓存（持久化存储）
}

// NewMultiLevelCache 创建多级缓存
func NewMultiLevelCache(redisCache Cache, dbCache Cache) *MultiLevelCache {
	return &MultiLevelCache{
		redisCache: redisCache,
		dbCache:    dbCache,
	}
}

// GetTools 获取工具列表（先查 Redis，再查数据库）
func (m *MultiLevelCache) GetTools(ctx context.Context, identifier string) ([]mcp.Tool, error) {
	// 1. 先尝试从 Redis 获取
	if m.redisCache != nil && m.redisCache.IsAvailable() {
		tools, err := m.redisCache.GetTools(ctx, identifier)
		if err == nil && len(tools) > 0 {
			log.Printf("[Cache] Hit Redis cache for %s (%d tools)", identifier, len(tools))
			return tools, nil
		}
		// Redis 错误但不致命，继续尝试数据库
		if err != nil {
			log.Printf("[Cache] Redis error (non-fatal): %v, falling back to DB", err)
		} else if len(tools) == 0 {
			log.Printf("[Cache] Redis cache miss for %s (empty result), falling back to DB", identifier)
		}
	}

	// 2. Redis 未命中或不可用，从数据库获取
	if m.dbCache != nil {
		log.Printf("[Cache] Querying DB cache for %s", identifier)
		tools, err := m.dbCache.GetTools(ctx, identifier)
		if err == nil && len(tools) > 0 {
			log.Printf("[Cache] Hit DB cache for %s (%d tools)", identifier, len(tools))
			// 如果 Redis 可用，异步回写 Redis（不阻塞）
			if m.redisCache != nil && m.redisCache.IsAvailable() {
				go func() {
					if err := m.redisCache.SetTools(context.Background(), identifier, tools, 24*time.Hour); err != nil {
						log.Printf("[Cache] Failed to backfill Redis: %v", err)
					} else {
						log.Printf("[Cache] Backfilled Redis cache for %s", identifier)
					}
				}()
			}
			return tools, nil
		}
		if err != nil {
			log.Printf("[Cache] DB error: %v", err)
		} else if len(tools) == 0 {
			log.Printf("[Cache] DB cache miss for %s (empty result)", identifier)
		}
	}

	// 3. 两级缓存都未命中
	log.Printf("[Cache] Cache miss for %s in both Redis and DB", identifier)
	return nil, nil
}

// SetTools 设置工具列表（同时写入 Redis 和数据库）
func (m *MultiLevelCache) SetTools(ctx context.Context, identifier string, tools []mcp.Tool, ttl time.Duration) error {
	// 1. 写入数据库（持久化）
	if m.dbCache != nil {
		if err := m.dbCache.SetTools(ctx, identifier, tools, 0); err != nil {
			log.Printf("[Cache] Failed to write to DB: %v", err)
			// 数据库写入失败不影响 Redis 写入
		}
	}

	// 2. 写入 Redis（快速缓存）
	if m.redisCache != nil && m.redisCache.IsAvailable() {
		if err := m.redisCache.SetTools(ctx, identifier, tools, ttl); err != nil {
			log.Printf("[Cache] Failed to write to Redis: %v", err)
			// Redis 写入失败不影响数据库写入
		}
	}

	return nil
}

// DeleteTools 删除工具列表缓存（同时删除 Redis 和数据库）
func (m *MultiLevelCache) DeleteTools(ctx context.Context, identifier string) error {
	var lastErr error

	if m.redisCache != nil && m.redisCache.IsAvailable() {
		if err := m.redisCache.DeleteTools(ctx, identifier); err != nil {
			lastErr = err
		}
	}

	if m.dbCache != nil {
		if err := m.dbCache.DeleteTools(ctx, identifier); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// IsAvailable 检查缓存是否可用（Redis 或数据库任一可用即可）
func (m *MultiLevelCache) IsAvailable() bool {
	if m.redisCache != nil && m.redisCache.IsAvailable() {
		return true
	}
	if m.dbCache != nil && m.dbCache.IsAvailable() {
		return true
	}
	return false
}
