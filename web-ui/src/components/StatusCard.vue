<template>
  <el-card class="status-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>📊 系统状态</span>
        <el-button text @click="$emit('refresh')" :icon="Refresh" size="small">
          刷新
        </el-button>
      </div>
    </template>

    <div v-if="status" class="status-main-content">
      <!-- API 接口列表 -->
      <el-card class="api-endpoints-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <span>🔗 API 接口地址</span>
          </div>
        </template>

        <el-table :data="apiEndpoints" style="width: 100%" size="small" max-height="400">
          <el-table-column label="接口地址" width="280">
            <template #default="scope">
              <el-tag :type="getMethodType(scope.row.method)" size="small" style="margin-right: 8px;">
                {{ scope.row.method }}
              </el-tag>
              <el-text type="primary" style="font-family: monospace; font-size: 12px;">
                {{ scope.row.path }}
              </el-text>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="说明" />
        </el-table>
      </el-card>

      <!-- API 接口调用统计 -->
      <el-card v-if="status && status.api" class="api-stats-card" shadow="hover" style="margin-top: 20px;">
        <template #header>
          <div class="card-header">
            <span>📡 API 接口调用统计</span>
          </div>
        </template>

      <div class="api-stats-content">
        <div class="api-stats-row">
          <div class="api-stat-item">
            <span class="stat-label">总请求数：</span>
            <el-text type="primary" style="font-weight: 600; font-size: 16px;">
              {{ status.api.totalRequests || 0 }}
            </el-text>
          </div>
          <div class="api-stat-item">
            <span class="stat-label">错误数：</span>
            <el-text :type="(status.api.totalErrors || 0) > 0 ? 'danger' : 'success'" style="font-weight: 600; font-size: 16px;">
              {{ status.api.totalErrors || 0 }}
            </el-text>
          </div>
          <div class="api-stat-item">
            <span class="stat-label">成功率：</span>
            <el-text 
              :type="getSuccessRate(status.api) >= 95 ? 'success' : getSuccessRate(status.api) >= 80 ? 'warning' : 'danger'" 
              style="font-weight: 600; font-size: 16px;"
            >
              {{ getSuccessRate(status.api).toFixed(1) }}%
            </el-text>
          </div>
        </div>

        <div v-if="status.api.lastRequestTime" class="api-stats-row" style="margin-top: 12px; padding-top: 12px; border-top: 1px solid #eee;">
          <div class="api-stat-item">
            <span class="stat-label">最后请求：</span>
            <el-text type="info" size="small">
              {{ formatTime(status.api.lastRequestTime) }}
            </el-text>
            <el-text type="info" size="small" style="margin-left: 8px;">
              ({{ status.api.lastRequestPath }})
            </el-text>
          </div>
        </div>

        <div v-if="status.api.lastErrorTime" class="api-stats-row" style="margin-top: 8px;">
          <div class="api-stat-item">
            <span class="stat-label">最后错误：</span>
            <el-text type="danger" size="small">
              {{ formatTime(status.api.lastErrorTime) }}
            </el-text>
            <el-text type="danger" size="small" style="margin-left: 8px;">
              ({{ status.api.lastErrorPath }})
            </el-text>
          </div>
        </div>

        <div v-if="status.api.topPaths && status.api.topPaths.length > 0" class="api-top-paths" style="margin-top: 16px; padding-top: 16px; border-top: 1px solid #eee;">
          <div class="stat-label" style="margin-bottom: 8px; font-weight: 500;">热门接口（Top 5）：</div>
          <div class="top-paths-list">
            <div 
              v-for="(item, index) in status.api.topPaths" 
              :key="index"
              class="top-path-item"
            >
              <el-text type="info" size="small" style="font-family: monospace;">
                {{ item.path }}
              </el-text>
              <el-tag size="small" type="primary" style="margin-left: 8px;">
                {{ item.count }} 次
              </el-tag>
            </div>
          </div>
        </div>
      </div>
      </el-card>

      <!-- MCP 服务列表 -->
      <el-card v-if="status" class="mcp-services-card" shadow="hover" style="margin-top: 20px;">
      <template #header>
        <div class="card-header">
          <span>MCP 服务</span>
        </div>
      </template>

      <el-table :data="mcpServices" style="width: 100%">
        <el-table-column label="服务名称" width="200">
          <template #default="scope">
            <div style="display: flex; align-items: center; gap: 8px;">
              <div
                :style="{
                  width: '32px',
                  height: '32px',
                  borderRadius: '4px',
                  backgroundColor: scope.row.iconColor || '#6366f1',
                  color: 'white',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontWeight: 'bold'
                }"
              >
                {{ scope.row.icon || 'M' }}
              </div>
              <span style="font-weight: 500;">{{ scope.row.name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'running' ? 'success' : 'info'">
              {{ scope.row.status === 'running' ? '运行中' : '未运行' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="scope">
            <el-button size="small" @click="handleTest(scope.row)">测试</el-button>
          </template>
        </el-table-column>
      </el-table>
      </el-card>
    </div>

    <el-skeleton v-else :rows="3" animated />
  </el-card>
</template>

<script setup>
import { ref } from 'vue'
import { Refresh, Check, Close, Warning, Tools } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

defineProps({
  status: {
    type: Object,
    default: null
  }
})

defineEmits(['refresh'])

// 计算成功率
const getSuccessRate = (apiStats) => {
  if (!apiStats || !apiStats.totalRequests || apiStats.totalRequests === 0) {
    return 100
  }
  const successCount = apiStats.totalRequests - (apiStats.totalErrors || 0)
  return (successCount / apiStats.totalRequests) * 100
}

// 格式化时间
const formatTime = (timestamp) => {
  if (!timestamp) return '-'
  const date = new Date(timestamp * 1000)
  const now = new Date()
  const diff = Math.floor((now - date) / 1000) // 秒数差
  
  if (diff < 60) {
    return `${diff} 秒前`
  } else if (diff < 3600) {
    return `${Math.floor(diff / 60)} 分钟前`
  } else if (diff < 86400) {
    return `${Math.floor(diff / 3600)} 小时前`
  } else {
    return date.toLocaleString('zh-CN', { 
      month: 'short', 
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }
}

// 获取 HTTP 方法的标签类型
const getMethodType = (method) => {
  const methodMap = {
    'GET': 'success',
    'POST': 'primary',
    'PUT': 'warning',
    'DELETE': 'danger'
  }
  return methodMap[method] || 'info'
}

// 固定的 MCP 服务列表
const mcpServices = ref([
  {
    name: 'k8s-mcp服务',
    description: 'Kubernetes 集群管理 MCP 服务',
    icon: 'K',
    iconColor: '#409eff',
    status: 'running'
  },
  {
    name: 'ansible-mcp服务',
    description: 'Ansible 自动化运维 MCP 服务',
    icon: 'A',
    iconColor: '#67c23a',
    status: 'running'
  },
  {
    name: '监控mcp服务',
    description: '系统监控和告警 MCP 服务',
    icon: 'M',
    iconColor: '#e6a23c',
    status: 'running'
  }
])

// API 接口列表
const apiEndpoints = ref([
  { method: 'GET', path: '/api/status', description: '获取系统状态（K8s、LLM、MCP 工具状态）' },
  { method: 'GET', path: '/api/config', description: '获取所有配置（K8s、LLM、MCP、智能体）' },
  { method: 'GET', path: '/api/config/agents', description: '获取所有智能体配置列表' },
  { method: 'POST', path: '/api/config/agents', description: '创建新的智能体配置' },
  { method: 'PUT', path: '/api/config/agents/:id', description: '更新指定智能体配置' },
  { method: 'DELETE', path: '/api/config/agents/:id', description: '删除指定智能体配置' },
  { method: 'GET', path: '/api/config/llm', description: '获取所有 LLM 配置列表' },
  { method: 'POST', path: '/api/config/llm', description: '创建新的 LLM 配置' },
  { method: 'GET', path: '/api/config/k8s', description: '获取所有 K8s 配置列表' },
  { method: 'POST', path: '/api/config/k8s', description: '创建新的 K8s 配置' },
  { method: 'GET', path: '/api/config/remote-mcp', description: '获取所有远程 MCP 服务配置' },
  { method: 'POST', path: '/api/config/remote-mcp', description: '创建新的远程 MCP 服务配置' },
  { method: 'POST', path: '/api/chat', description: '发送对话请求，进行智能问答' },
  { method: 'GET', path: '/api/sessions', description: '获取会话列表（支持分页）' },
  { method: 'GET', path: '/api/sessions/:sessionId', description: '获取指定会话的详细信息' },
  { method: 'GET', path: '/api/tools', description: '获取所有可用的 MCP 工具列表' },
  { method: 'POST', path: '/api/tools/call', description: '调用指定的 MCP 工具' },
  { method: 'POST', path: '/api/test-k8s', description: '测试 Kubernetes 连接' },
  { method: 'POST', path: '/api/test-llm', description: '测试 LLM 连接' },
  { method: 'GET', path: '/api/logs', description: '获取系统日志（支持过滤和分页）' },
  { method: 'GET', path: '/api/current-llm', description: '获取当前默认 LLM 配置信息' }
])

const handleTest = (service) => {
  ElMessage.info(`正在测试 ${service.name}...`)
  // TODO: 实现服务测试逻辑
}
</script>

<style scoped>
.status-card {
  margin: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}

.status-main-content {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.api-stats-content {
  padding: 8px 0;
}

.api-stats-row {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
  align-items: center;
}

.api-stat-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stat-label {
  color: #606266;
  font-size: 14px;
}

.api-top-paths {
  margin-top: 16px;
}

.top-paths-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.top-path-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: #f5f7fa;
  border-radius: 4px;
}
</style>

