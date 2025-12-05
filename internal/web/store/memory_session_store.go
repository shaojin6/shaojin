package store

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemorySessionStore 内存会话存储（降级方案）
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionDoc
}

// NewMemorySessionStore 创建内存会话存储
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*SessionDoc),
	}
}

// GetSession 获取会话（包含完整消息）
func (m *MemorySessionStore) GetSession(ctx context.Context, sessionID string) (*SessionDoc, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil
	}
	// 返回副本
	sessionCopy := *session
	return &sessionCopy, nil
}

// GetSessions 获取会话列表（只返回元数据，不包含消息）
func (m *MemorySessionStore) GetSessions(ctx context.Context, agentID string, limit, skip int) ([]*SessionMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*SessionDoc
	for _, session := range m.sessions {
		if agentID == "" || session.AgentID == agentID {
			sessions = append(sessions, session)
		}
	}

	// 按更新时间倒序排序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	// 分页
	start := skip
	end := skip + limit
	if start > len(sessions) {
		start = len(sessions)
	}
	if end > len(sessions) {
		end = len(sessions)
	}

	metas := make([]*SessionMeta, 0, end-start)
	for i := start; i < end; i++ {
		session := sessions[i]
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

		metas = append(metas, &SessionMeta{
			ID:           session.ID,
			AgentID:      session.AgentID,
			Title:        title,
			MessageCount: len(session.Messages),
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.UpdatedAt,
		})
	}

	return metas, nil
}

// SaveSession 保存会话
func (m *MemorySessionStore) SaveSession(ctx context.Context, session *SessionDoc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session.UpdatedAt = time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	m.sessions[session.ID] = session
	return nil
}

// DeleteSession 删除会话
func (m *MemorySessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	return nil
}

// CountSessions 统计会话数量
func (m *MemorySessionStore) CountSessions(ctx context.Context, agentID string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := int64(0)
	for _, session := range m.sessions {
		if agentID == "" || session.AgentID == agentID {
			count++
		}
	}
	return count, nil
}
