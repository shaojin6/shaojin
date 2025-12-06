package tasks

import (
	"context"
	"fmt"
	"sync"

	"github.com/your-org/k8s-mcp-agent/internal/web/mcpclient"
)

// ToolRefreshTask 工具刷新任务
type ToolRefreshTask struct {
	toolManagerMu *sync.RWMutex
	toolManager   *mcpclient.ToolManager
	identifier    string // 可选：只刷新特定服务，空字符串表示刷新所有
}

// NewToolRefreshTask 创建工具刷新任务
func NewToolRefreshTask(toolManagerMu *sync.RWMutex, toolManager *mcpclient.ToolManager, identifier string) *ToolRefreshTask {
	return &ToolRefreshTask{
		toolManagerMu: toolManagerMu,
		toolManager:   toolManager,
		identifier:    identifier,
	}
}

func (t *ToolRefreshTask) Name() string {
	if t.identifier != "" {
		return fmt.Sprintf("tool-refresh-%s", t.identifier)
	}
	return "tool-refresh-all"
}

func (t *ToolRefreshTask) Execute(ctx context.Context) error {
	if t.toolManager == nil {
		return fmt.Errorf("tool manager is nil")
	}

	t.toolManagerMu.Lock()
	defer t.toolManagerMu.Unlock()

	if t.toolManager == nil {
		return fmt.Errorf("tool manager is nil (after lock)")
	}

	// 刷新所有远程工具
	t.toolManager.RefreshRemoteTools()
	return nil
}

func (t *ToolRefreshTask) RetryCount() int {
	return 2 // 工具刷新失败可以重试2次
}

