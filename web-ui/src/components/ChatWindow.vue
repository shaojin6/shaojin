<template>
  <div class="chat-container">
    <!-- 左侧历史记录面板 -->
    <div class="history-sidebar" :style="{ width: sidebarWidth + 'px' }">
      <div class="history-header">
        <span>📜 历史记录</span>
        <div style="display: flex; gap: 4px;">
          <el-button 
            text 
            size="small" 
            @click="createNewSession"
            title="新建对话"
          >
            <el-icon><Plus /></el-icon>
          </el-button>
          <el-button 
            text 
            size="small" 
            @click="loadSessions"
            :loading="loadingSessions"
            title="刷新"
          >
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>
      </div>
      <div class="history-list" v-loading="loadingSessions">
        <div v-if="sessions.length === 0" class="empty-history">
          <el-text type="info" size="small">暂无历史记录</el-text>
        </div>
        <div
          v-for="session in sessions"
          :key="session.id"
          :class="['history-item', { active: session.id === currentSessionId }]"
          @click="loadSession(session.id)"
        >
          <div class="history-title">{{ session.title }}</div>
          <div class="history-meta">
            <el-text type="info" size="small">
              {{ formatTime(session.updatedAt) }} · {{ session.messageCount || 0 }} 条消息
            </el-text>
          </div>
        </div>
      </div>
    </div>

    <!-- 拖拽分隔条 -->
    <div 
      class="resizer" 
      @mousedown="startResize"
      :class="{ resizing: isResizing }"
    ></div>

    <!-- 右侧对话区域 -->
    <el-card class="chat-window" shadow="hover">
      <!-- 固定头部 -->
      <template #header>
        <div class="card-header">
          <span>💬 对话窗口</span>
          <div class="agent-selector">
            <el-select
              v-model="selectedAgentId"
              placeholder="选择智能体"
              size="small"
              style="width: 220px"
              :loading="loadingAgents"
              @change="handleAgentChange"
            >
              <el-option
                v-for="agent in agents"
                :key="agent.id"
                :label="agent.name + (agent.enabled ? '' : '（停用）')"
                :value="agent.id"
                :disabled="!agent.enabled"
              />
            </el-select>
          </div>
          <div class="header-info">
            <el-text v-if="currentLLM" type="success" size="small">
              <el-icon><ChatDotRound /></el-icon>
              使用 LLM: {{ currentLLM.name || currentLLM.provider }} ({{ currentLLM.model }})
            </el-text>
            <el-text v-else type="warning" size="small">
              <el-icon><Warning /></el-icon>
              未配置 LLM，请先配置 LLM
            </el-text>
            <el-text type="info" size="small" style="margin-left: 10px">
              会话 ID: {{ currentSessionId }}
            </el-text>
          </div>
        </div>
      </template>

      <!-- 消息列表 -->
      <div class="messages-container" ref="messagesContainer">
        <div v-if="messages.length === 0" class="empty-state">
          <el-empty description="开始对话吧！我可以帮你进行 K8s 集群运维和 Ansible 自动化运维" />
        </div>

        <div
          v-for="(msg, index) in messages"
          :key="index"
          :class="['message', msg.role]"
        >
          <div class="message-avatar">
            <el-icon v-if="msg.role === 'user'" :size="20">
              <User />
            </el-icon>
            <el-icon v-else :size="20">
              <ChatDotRound />
            </el-icon>
          </div>
          <div class="message-content">
            <!-- 显示工具调用标记（像 dify 那样） -->
            <div v-if="msg.steps && msg.steps.length > 0" class="tool-badge">
              <el-text type="info" size="small">
                已使用 
                <span v-for="(step, idx) in msg.steps.filter(s => s.type === 'tool')" :key="idx">
                  <strong>{{ step.tool }}</strong><span v-if="idx < msg.steps.filter(s => s.type === 'tool').length - 1">、</span>
                </span>
                <span v-if="msg.steps.filter(s => s.type === 'tool').length > 0"> &gt;</span>
              </el-text>
            </div>
            <div class="message-text">{{ msg.content }}</div>
            
            <!-- 显示工具调用步骤 -->
            <div v-if="msg.steps && msg.steps.length > 0" class="message-steps">
              <div class="steps-header">
                <el-text type="info" size="small">
                  <el-icon><Tools /></el-icon>
                  执行步骤 ({{ msg.steps.length }})
                </el-text>
              </div>
              <div class="steps-content">
                <div
                  v-for="(step, stepIndex) in msg.steps"
                  :key="stepIndex"
                  class="step-item"
                >
                  <div class="step-header">
                    <el-tag :type="step.type === 'tool' ? 'warning' : 'info'" size="small">
                      {{ step.type === 'tool' ? '🔧 工具调用' : '💭 LLM 思考' }}
                    </el-tag>
                    <span v-if="step.tool" class="step-tool">
                      {{ step.tool }}({{ formatArgs(step.arguments) }})
                    </span>
                    <span v-else-if="step.text" class="step-text">
                      {{ step.text.substring(0, 100) }}{{ step.text.length > 100 ? '...' : '' }}
                    </span>
                  </div>
                  <div v-if="step.result" class="step-result">
                    <el-text type="success" size="small">✓ 执行结果:</el-text>
                    <pre>{{ formatResult(step.result) }}</pre>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 加载中 -->
        <div v-if="loading" class="message assistant">
          <div class="message-avatar">
            <el-icon :size="20">
              <ChatDotRound />
            </el-icon>
          </div>
          <div class="message-content">
            <el-text type="info">正在思考...</el-text>
          </div>
        </div>
      </div>

      <!-- 输入框 -->
      <div class="input-container">
        <el-input
          v-model="inputMessage"
          type="textarea"
          :rows="3"
          placeholder="输入你的问题，例如：列出 default 命名空间的所有 Pods，或使用 Ansible 部署应用"
          @keydown.ctrl.enter="sendMessage"
          @keydown.enter.exact="handleEnterKey"
          :disabled="loading"
          :autosize="{ minRows: 3, maxRows: 6 }"
          clearable
        />
        <div class="input-actions">
          <el-text type="info" size="small">Enter 或 Ctrl + Enter 发送</el-text>
          <div style="display: flex; gap: 8px;">
            <el-button
              v-if="loading"
              type="danger"
              @click="stopRequest"
              :icon="Close"
            >
              停止
            </el-button>
            <el-button
              type="primary"
              @click="sendMessage"
              :loading="loading"
              :disabled="!inputMessage.trim() || loading"
            >
              发送
            </el-button>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, nextTick, watch, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { User, ChatDotRound, Warning, Refresh, Plus, Tools, Close } from '@element-plus/icons-vue'
import { sendChat, getSessions, getSession } from '../api/chat'
import { getAgents } from '../api/agents'
import { getCurrentLLM } from '../api/config'

const props = defineProps({
  sessionId: {
    type: String,
    required: true
  }
})

const messages = ref([])
const inputMessage = ref('')
const loading = ref(false)
const messagesContainer = ref(null)
const currentLLM = ref(null)
const agents = ref([])
const selectedAgentId = ref('')
const loadingAgents = ref(false)
const sessions = ref([])
const loadingSessions = ref(false)
const currentSessionId = ref(props.sessionId)
const sidebarWidth = ref(280) // 默认宽度
const isResizing = ref(false)
const startX = ref(0)
const startWidth = ref(280)
const abortController = ref(null) // 用于取消请求

// 加载当前 LLM 信息
const loadCurrentLLM = async () => {
  try {
    const llmInfo = await getCurrentLLM()
    if (llmInfo.configured) {
      currentLLM.value = llmInfo
    } else {
      currentLLM.value = null
    }
  } catch (error) {
    console.error('获取 LLM 信息失败:', error)
    currentLLM.value = null
  }
}

const selectDefaultAgent = (list) => {
  if (!Array.isArray(list) || list.length === 0) {
    selectedAgentId.value = ''
    return
  }
  const enabledAgents = list.filter(agent => agent.enabled)
  const defaultAgent = enabledAgents.find(agent => agent.isDefault) || enabledAgents[0]
  selectedAgentId.value = defaultAgent ? defaultAgent.id : ''
}

const loadAgents = async () => {
  loadingAgents.value = true
  try {
    const data = await getAgents()
    agents.value = Array.isArray(data) ? data : []
    selectDefaultAgent(agents.value)
    if (selectedAgentId.value) {
      handleAgentChange()
    } else {
      sessions.value = []
    }
  } catch (error) {
    console.error('加载智能体失败:', error)
    agents.value = []
  } finally {
    loadingAgents.value = false
  }
}

const handleAgentChange = () => {
  if (!selectedAgentId.value) return
  // 切换智能体时静默创建新对话，不显示提示
  createNewSession(false)
}

// 新建对话
const createNewSession = (showMessage = true) => {
  if (!selectedAgentId.value) {
    if (showMessage) {
    ElMessage.warning('请先选择一个智能体')
    }
    return
  }
  // 生成新的会话ID
  currentSessionId.value = `${selectedAgentId.value}-session-${Date.now()}`
  // 清空消息
  messages.value = []
  // 刷新会话列表
  loadSessions()
  // 只有用户主动点击"+"按钮时才显示提示
  if (showMessage) {
  ElMessage.success('已创建新对话')
  }
}

// 加载会话列表
const loadSessions = async () => {
  if (!selectedAgentId.value) {
    sessions.value = []
    loadingSessions.value = false
    return
  }
  loadingSessions.value = true
  try {
    const result = await getSessions(selectedAgentId.value, 50, 0)
    // 兼容新格式（对象）和旧格式（数组）
    if (Array.isArray(result)) {
      sessions.value = result
    } else if (result && result.sessions) {
      sessions.value = result.sessions || []
    } else {
      sessions.value = []
    }
  } catch (error) {
    console.error('加载会话列表失败:', error)
    sessions.value = [] // 确保设置为空数组
    if (error.code === 'ECONNABORTED' || error.message?.includes('timeout')) {
      ElMessage.error('加载历史记录超时，请稍后重试')
    } else {
      ElMessage.error('加载历史记录失败: ' + (error.response?.data?.error || error.message || '未知错误'))
    }
  } finally {
    loadingSessions.value = false
  }
}

// 加载指定会话
const loadSession = async (sessionId) => {
  if (!selectedAgentId.value || sessionId === currentSessionId.value) return
  
  try {
    const sessionData = await getSession(sessionId)
    if (sessionData && sessionData.messages) {
      if (sessionData.agentId && sessionData.agentId !== selectedAgentId.value) {
        ElMessage.warning('该会话属于其他智能体，请先切换智能体')
        return
      }
      currentSessionId.value = sessionId
      // 转换消息格式
      messages.value = sessionData.messages.map(msg => ({
        role: msg.role,
        content: msg.content,
        steps: [] // 历史消息可能没有steps
      }))
      scrollToBottom()
    }
  } catch (error) {
    console.error('加载会话失败:', error)
    ElMessage.error('加载会话失败')
  }
}

// 格式化时间
const formatTime = (timestamp) => {
  const date = new Date(timestamp * 1000)
  const now = new Date()
  const diff = now - date
  
  if (diff < 60000) {
    return '刚刚'
  } else if (diff < 3600000) {
    return `${Math.floor(diff / 60000)} 分钟前`
  } else if (diff < 86400000) {
    return `${Math.floor(diff / 3600000)} 小时前`
  } else if (diff < 604800000) {
    return `${Math.floor(diff / 86400000)} 天前`
  } else {
    return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
  }
}

// 加载保存的侧边栏宽度
const loadSidebarWidth = () => {
  const saved = localStorage.getItem('chat-sidebar-width')
  if (saved) {
    const width = parseInt(saved, 10)
    if (width >= 200 && width <= 600) {
      sidebarWidth.value = width
    }
  }
}

// 保存侧边栏宽度
const saveSidebarWidth = () => {
  localStorage.setItem('chat-sidebar-width', sidebarWidth.value.toString())
}

// 开始调整大小
const startResize = (e) => {
  isResizing.value = true
  startX.value = e.clientX
  startWidth.value = sidebarWidth.value
  
  document.addEventListener('mousemove', handleResize)
  document.addEventListener('mouseup', stopResize)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  
  e.preventDefault()
}

// 调整大小
const handleResize = (e) => {
  if (!isResizing.value) return
  
  const diff = e.clientX - startX.value
  let newWidth = startWidth.value + diff
  
  // 限制最小和最大宽度
  if (newWidth < 200) newWidth = 200
  if (newWidth > 600) newWidth = 600
  
  sidebarWidth.value = newWidth
}

// 停止调整大小
const stopResize = () => {
  if (isResizing.value) {
    isResizing.value = false
    saveSidebarWidth()
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    document.removeEventListener('mousemove', handleResize)
    document.removeEventListener('mouseup', stopResize)
  }
}

onMounted(() => {
  loadCurrentLLM()
  loadAgents()
  loadSidebarWidth()
})

const formatArgs = (args) => {
  if (!args) return ''
  return Object.entries(args)
    .map(([k, v]) => `${k}=${v}`)
    .join(', ')
}

const formatResult = (result) => {
  if (typeof result === 'string') return result
  return JSON.stringify(result, null, 2)
}

const handleEnterKey = (e) => {
  if (e.ctrlKey || e.metaKey) {
    e.preventDefault()
    sendMessage()
  }
  // 否则允许换行
}

const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

// 停止请求
const stopRequest = () => {
  if (abortController.value) {
    abortController.value.abort()
    abortController.value = null
    loading.value = false
    ElMessage.info('已停止请求')
    
    // 移除"正在思考"的消息（如果存在）
    const thinkingIndex = messages.value.findIndex(msg => msg.role === 'assistant' && msg.content === '正在思考...')
    if (thinkingIndex >= 0) {
      messages.value.splice(thinkingIndex, 1)
    }
    
    // 添加停止提示
    messages.value.push({
      role: 'assistant',
      content: '请求已停止。'
    })
    scrollToBottom()
  }
}

const sendMessage = async () => {
  if (!inputMessage.value.trim() || loading.value) return

  if (!selectedAgentId.value) {
    ElMessage.warning('请先选择一个智能体')
    return
  }

  // 检查是否配置了 LLM
  if (!currentLLM.value) {
    ElMessage.warning('请先配置 LLM 才能进行对话')
    return
  }

  const userMessage = inputMessage.value.trim()
  // 先清空输入框，让用户可以继续输入
  inputMessage.value = ''

  // 添加用户消息
  messages.value.push({
    role: 'user',
    content: userMessage
  })
  scrollToBottom()

  loading.value = true
  
  // 创建 AbortController 用于取消请求
  abortController.value = new AbortController()

  try {
    const response = await sendChat(currentSessionId.value, userMessage, selectedAgentId.value, abortController.value.signal)
    
    if (response.agentId && !selectedAgentId.value) {
      selectedAgentId.value = response.agentId
    }

    // 添加助手回复
    messages.value.push({
      role: 'assistant',
      content: response.reply,
      steps: response.steps
    })
    
    // 更新会话ID（如果是新会话）
    if (response.sessionId && response.sessionId !== currentSessionId.value) {
      currentSessionId.value = response.sessionId
    }
    
    // 刷新会话列表
    loadSessions()
    
    scrollToBottom()
  } catch (error) {
    // 如果是用户主动取消，不显示错误消息
    if (error.name === 'CanceledError' || error.code === 'ERR_CANCELED' || abortController.value?.signal.aborted) {
      return
    }
    
    console.error('Chat error:', error)
    const errorData = error.response?.data || {}
    const errorMessage = errorData.error || errorData.message || error.message || '未知错误'
    const errorType = errorData.type || (error.code === 'ECONNABORTED' ? 'timeout' : 'unknown')
    
    ElMessage.error('发送失败: ' + errorMessage)
    
    // 移除"正在思考"的消息（如果存在）
    const thinkingIndex = messages.value.findIndex(msg => msg.role === 'assistant' && msg.content === '正在思考...')
    if (thinkingIndex >= 0) {
      messages.value.splice(thinkingIndex, 1)
    }
    
    // 根据错误类型显示不同的提示
    let userMessage = ''
    if (errorType === 'timeout' || error.code === 'ECONNABORTED') {
      userMessage = `抱歉，请求超时：${errorMessage}\n\n可能的原因：\n1. LLM 服务响应过慢\n2. MCP 工具调用（kubernetes-mcp-server）耗时过长\n3. 网络连接不稳定\n\n建议：简化问题或稍后重试。`
    } else if (errorType === 'llm') {
      userMessage = `抱歉，LLM 调用失败：${errorMessage}\n\n请检查：\n1. LLM 配置是否正确\n2. LLM API Key 是否有效\n3. LLM 服务是否可访问`
    } else if (errorType === 'tool') {
      userMessage = `抱歉，工具调用失败：${errorMessage}\n\n请检查：\n1. MCP 服务（kubernetes-mcp-server）是否正常运行\n2. MCP 服务连接是否正常\n3. 工具调用参数是否正确`
    } else {
      userMessage = `抱歉，处理请求时出错：${errorMessage}\n\n请检查：\n1. 网络连接是否正常\n2. 服务配置是否正确\n3. 查看后端日志获取详细信息`
    }
    
    // 添加错误消息
    messages.value.push({
      role: 'assistant',
      content: userMessage
    })
    scrollToBottom()
  } finally {
    loading.value = false
    abortController.value = null // 清除 AbortController
    // 确保输入框重新获得焦点
    nextTick(() => {
      scrollToBottom()
      // 尝试让输入框重新获得焦点（如果可能）
      const textarea = document.querySelector('.input-container textarea')
      if (textarea && !loading.value) {
        textarea.focus()
      }
    })
  }
}

watch(() => messages.value.length, () => {
  scrollToBottom()
})
</script>

<style scoped>
.chat-container {
  display: flex;
  height: calc(100vh - 100px);
  gap: 0;
  position: relative;
}

.resizer {
  width: 4px;
  background: #e0e0e0;
  cursor: col-resize;
  flex-shrink: 0;
  position: relative;
  transition: background-color 0.2s;
}

.resizer:hover {
  background: #409eff;
}

.resizer.resizing {
  background: #409eff;
}

.resizer::before {
  content: '';
  position: absolute;
  left: -2px;
  right: -2px;
  top: 0;
  bottom: 0;
  cursor: col-resize;
}

.history-sidebar {
  background: white;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  flex-shrink: 0;
  min-width: 200px;
  max-width: 600px;
}

.history-header {
  padding: 16px;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}

.history-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.empty-history {
  padding: 40px 20px;
  text-align: center;
}

.history-item {
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.history-item:hover {
  background-color: #f5f5f5;
}

.history-item.active {
  background-color: #e6f7ff;
  border-left: 3px solid #409eff;
}

.history-title {
  font-size: 14px;
  color: #333;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-meta {
  font-size: 12px;
  color: #999;
}

.chat-window {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  position: relative;
}

.chat-window :deep(.el-card__header) {
  position: sticky;
  top: 0;
  z-index: 100;
  background: white;
  border-bottom: 1px solid #eee;
}

.chat-window :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}

.header-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-selector {
  margin-left: auto;
  margin-right: 20px;
}

.messages-container {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fafafa;
  margin-bottom: 20px;
  border-radius: 4px;
}

.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
}

.message {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.message.user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.message.user .message-avatar {
  background: #409eff;
  color: white;
}

.message-content {
  max-width: 70%;
  background: white;
  padding: 12px 16px;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.message.user .message-content {
  background: #409eff;
  color: white;
}

.message-text {
  white-space: pre-wrap;
  word-break: break-word;
}

.message-steps {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 2px solid #e6f7ff;
  background: #fafafa;
  border-radius: 8px;
  padding: 12px;
}

.steps-header {
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.steps-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.step-item {
  padding: 10px;
  background: white;
  border-radius: 6px;
  border-left: 3px solid #409eff;
}

.step-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.step-tool {
  margin-left: 8px;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  color: #606266;
  font-weight: 500;
}

.step-text {
  margin-left: 8px;
  font-size: 12px;
  color: #909399;
  font-style: italic;
}

.step-result {
  margin-top: 10px;
  padding: 10px;
  background: #f0f9ff;
  border-radius: 4px;
  border: 1px solid #b3d8ff;
}

.step-result pre {
  margin: 8px 0 0 0;
  font-size: 12px;
  overflow-x: auto;
  color: #303133;
  line-height: 1.5;
}

.input-container {
  padding: 0 20px 20px;
}

.input-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 10px;
}
</style>
