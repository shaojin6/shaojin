<template>
  <div class="log-viewer">
    <el-card>
      <template #header>
        <div class="log-header">
          <span>📋 系统日志</span>
          <div class="log-controls">
            <el-input
              v-model="filterText"
              placeholder="过滤日志（如：LLM、Orchestrator）"
              clearable
              style="width: 250px; margin-right: 10px;"
              @clear="loadLogs"
              @keyup.enter="loadLogs"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select v-model="linesCount" style="width: 120px; margin-right: 10px;" @change="loadLogs">
              <el-option label="最后50行" :value="50" />
              <el-option label="最后100行" :value="100" />
              <el-option label="最后200行" :value="200" />
              <el-option label="最后500行" :value="500" />
            </el-select>
            <el-button type="primary" @click="loadLogs" :loading="loading">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
            <el-button @click="autoRefresh = !autoRefresh" :type="autoRefresh ? 'success' : 'default'">
              <el-icon><VideoPlay v-if="!autoRefresh" /><VideoPause v-else /></el-icon>
              {{ autoRefresh ? '停止自动刷新' : '自动刷新' }}
            </el-button>
          </div>
        </div>
      </template>

      <div class="log-info" v-if="logInfo">
        <el-text type="info" size="small">
          总共 {{ logInfo.total }} 行日志
          <span v-if="filterText">，过滤后 {{ logInfo.filtered }} 行</span>
          ，显示最后 {{ logInfo.showing }} 行
        </el-text>
      </div>

      <div class="log-content" ref="logContainer">
        <div v-if="loading" class="loading">
          <el-icon class="is-loading"><Loading /></el-icon>
          <span>加载日志中...</span>
        </div>
        <div v-else-if="error" class="error">
          <el-alert :title="error" type="error" :closable="false" />
        </div>
        <div v-else-if="logs.length === 0" class="empty">
          <el-empty description="暂无日志" />
        </div>
        <pre v-else class="log-lines">{{ logText }}</pre>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Search, Refresh, VideoPlay, VideoPause, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'

const API_BASE = '/api'

const logs = ref([])
const loading = ref(false)
const error = ref('')
const filterText = ref('')
const linesCount = ref(100)
const autoRefresh = ref(false)
const logInfo = ref(null)
const logContainer = ref(null)
let refreshTimer = null

const logText = computed(() => {
  return logs.value.join('\n')
})

const loadLogs = async () => {
  loading.value = true
  error.value = ''
  try {
    const params = {
      lines: linesCount.value
    }
    if (filterText.value) {
      params.filter = filterText.value
    }
    
    const response = await axios.get(`${API_BASE}/logs`, { params })
    logs.value = response.data.lines || []
    logInfo.value = {
      total: response.data.total || 0,
      filtered: response.data.filtered || 0,
      showing: response.data.showing || 0
    }
    
    // 滚动到底部
    await nextTick()
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  } catch (err) {
    error.value = err.response?.data?.error || err.message || '加载日志失败'
    ElMessage.error(error.value)
  } finally {
    loading.value = false
  }
}

watch(autoRefresh, (newVal) => {
  if (newVal) {
    refreshTimer = setInterval(() => {
      loadLogs()
    }, 3000) // 每3秒刷新一次
  } else {
    if (refreshTimer) {
      clearInterval(refreshTimer)
      refreshTimer = null
    }
  }
})

onMounted(() => {
  loadLogs()
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style scoped>
.log-viewer {
  height: 100%;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.log-controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.log-info {
  margin-bottom: 10px;
  padding: 8px;
  background-color: #f5f7fa;
  border-radius: 4px;
}

.log-content {
  height: calc(100vh - 250px);
  min-height: 500px;
  overflow-y: auto;
  background-color: #1e1e1e;
  border-radius: 4px;
  padding: 15px;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
}

.log-lines {
  margin: 0;
  padding: 0;
  color: #d4d4d4;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.log-lines {
  /* 高亮不同类型的日志 */
  color: #d4d4d4;
}

/* 日志行样式 */
.log-lines {
  /* ERROR 日志 */
  background: transparent;
}

.loading,
.error,
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #909399;
}

.loading .el-icon {
  font-size: 32px;
  margin-bottom: 10px;
}

/* 滚动条样式 */
.log-content::-webkit-scrollbar {
  width: 8px;
}

.log-content::-webkit-scrollbar-track {
  background: #2d2d2d;
  border-radius: 4px;
}

.log-content::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 4px;
}

.log-content::-webkit-scrollbar-thumb:hover {
  background: #777;
}

@media (max-width: 768px) {
  .log-header {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .log-controls {
    width: 100%;
    flex-direction: column;
  }
  
  .log-controls > * {
    width: 100%;
  }
}
</style>

