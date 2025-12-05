package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/your-org/k8s-mcp-agent/internal/web/llm"
)

// contains 检查字符串是否包含子字符串（不区分大小写）
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// SessionStore 会话存储接口
type SessionStore interface {
	// GetSession 获取会话（包含完整消息）
	GetSession(ctx context.Context, sessionID string) (*SessionDoc, error)
	// GetSessions 获取会话列表（只返回元数据，不包含消息）
	GetSessions(ctx context.Context, agentID string, limit, skip int) ([]*SessionMeta, error)
	// SaveSession 保存会话
	SaveSession(ctx context.Context, session *SessionDoc) error
	// DeleteSession 删除会话
	DeleteSession(ctx context.Context, sessionID string) error
	// CountSessions 统计会话数量
	CountSessions(ctx context.Context, agentID string) (int64, error)
}

// SessionDoc MongoDB 会话文档
type SessionDoc struct {
	ID        string        `bson:"_id" json:"id"`
	AgentID   string        `bson:"agentId" json:"agentId"`
	Messages  []llm.Message `bson:"messages" json:"messages"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt" json:"updatedAt"`
}

// SessionMeta 会话元数据（用于列表查询，不包含消息）
type SessionMeta struct {
	ID           string    `bson:"_id" json:"id"`
	AgentID      string    `bson:"agentId" json:"agentId"`
	Title        string    `bson:"title,omitempty" json:"title,omitempty"`
	MessageCount int       `bson:"messageCount" json:"messageCount"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time `bson:"updatedAt" json:"updatedAt"`
}

// MongoSessionStore MongoDB 会话存储实现
type MongoSessionStore struct {
	client     *mongo.Client
	db         *mongo.Database
	collection *mongo.Collection
}

// NewMongoSessionStore 创建 MongoDB 会话存储
func NewMongoSessionStore(uri, dbName string) (*MongoSessionStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 为单节点 MongoDB 添加 directConnection=true 参数（如果还没有）
	// MongoDB URI 格式：mongodb://host:port/database?directConnection=true
	if !strings.Contains(uri, "directConnection") {
		if strings.Contains(uri, "?") {
			// 如果已经有查询参数，追加 directConnection
			uri = uri + "&directConnection=true"
		} else {
			// 如果没有查询参数，需要先确保有数据库名
			// 如果 URI 以 / 结尾，说明数据库名为空，需要添加数据库名
			if strings.HasSuffix(uri, "/") {
				uri = uri + "?directConnection=true"
			} else if !strings.Contains(uri, "/") {
				// 如果 URI 中没有 /，添加 / 和数据库名占位符
				uri = uri + "/?directConnection=true"
			} else {
				// URI 中已经有数据库名，直接添加查询参数
				uri = uri + "?directConnection=true"
			}
		}
	}

	// 构建客户端选项
	clientOptions := options.Client().ApplyURI(uri)
	// 设置连接超时
	clientOptions.SetConnectTimeout(10 * time.Second)
	// 设置服务器选择超时
	clientOptions.SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// 测试连接（使用新的上下文，避免超时）
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(dbName)
	collection := db.Collection("chat_sessions")

	store := &MongoSessionStore{
		client:     client,
		db:         db,
		collection: collection,
	}

	// 创建索引
	if err := store.createIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return store, nil
}

// createIndexes 创建索引
func (m *MongoSessionStore) createIndexes(ctx context.Context) error {
	// agentId + updatedAt 复合索引（用于按 Agent 查询并排序）
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "agentId", Value: 1},
			{Key: "updatedAt", Value: -1},
		},
	}
	_, err := m.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create agentId+updatedAt index: %w", err)
	}

	// updatedAt 索引（用于全局排序）
	indexModel2 := mongo.IndexModel{
		Keys: bson.D{{Key: "updatedAt", Value: -1}},
	}
	_, err = m.collection.Indexes().CreateOne(ctx, indexModel2)
	if err != nil {
		return fmt.Errorf("failed to create updatedAt index: %w", err)
	}

	return nil
}

// GetSession 获取会话（包含完整消息）
func (m *MongoSessionStore) GetSession(ctx context.Context, sessionID string) (*SessionDoc, error) {
	// 添加超时上下文
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var session SessionDoc
	err := m.collection.FindOne(queryCtx, bson.M{"_id": sessionID}).Decode(&session)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &session, nil
}

// GetSessions 获取会话列表（只返回元数据，不包含消息）
func (m *MongoSessionStore) GetSessions(ctx context.Context, agentID string, limit, skip int) ([]*SessionMeta, error) {
	// 添加超时上下文（5秒超时）
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if agentID != "" {
		filter["agentId"] = agentID
	}

	// 优化查询：只返回第一条消息用于生成标题，减少数据传输
	projection := bson.M{
		"_id":       1,
		"agentId":   1,
		"createdAt": 1,
		"updatedAt": 1,
		"messages":  bson.M{"$slice": 1}, // 只取第一条消息
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetProjection(projection)

	cursor, err := m.collection.Find(queryCtx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find sessions: %w", err)
	}
	defer cursor.Close(queryCtx)

	var sessions []*SessionMeta
	for cursor.Next(queryCtx) {
		var doc SessionDoc
		if err := cursor.Decode(&doc); err != nil {
			// 如果解码失败，跳过这条记录
			continue
		}

		// 提取第一条用户消息作为标题（由于 projection，messages 最多只有1条）
		title := "新会话"
		if len(doc.Messages) > 0 {
			msg := doc.Messages[0]
			if msg.Role == "user" {
				title = msg.Content
				if len(title) > 50 {
					title = title[:50] + "..."
				}
			}
		}

		// 注意：由于使用了 projection，无法获取准确的消息数量
		// 如果需要准确的消息数量，需要单独查询或使用聚合管道
		sessions = append(sessions, &SessionMeta{
			ID:           doc.ID,
			AgentID:      doc.AgentID,
			Title:        title,
			MessageCount: len(doc.Messages), // 这里最多是1（因为 projection）
			CreatedAt:    doc.CreatedAt,
			UpdatedAt:    doc.UpdatedAt,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return sessions, nil
}

// SaveSession 保存会话
func (m *MongoSessionStore) SaveSession(ctx context.Context, session *SessionDoc) error {
	// 提取第一条用户消息作为标题
	title := "新会话"
	for _, msg := range session.Messages {
		if msg.Role == "user" {
			title = msg.Content
			if len(title) > 50 {
				title = title[:50] + "..."
			}
			break
		}
	}

	// 确保 UpdatedAt 是最新的
	session.UpdatedAt = time.Now()

	opts := options.Replace().SetUpsert(true)
	_, err := m.collection.ReplaceOne(ctx, bson.M{"_id": session.ID}, session, opts)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

// DeleteSession 删除会话
func (m *MongoSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := m.collection.DeleteOne(ctx, bson.M{"_id": sessionID})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// CountSessions 统计会话数量
func (m *MongoSessionStore) CountSessions(ctx context.Context, agentID string) (int64, error) {
	// 添加超时上下文
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if agentID != "" {
		filter["agentId"] = agentID
	}
	count, err := m.collection.CountDocuments(queryCtx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	return count, nil
}

// Close 关闭连接
func (m *MongoSessionStore) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}
