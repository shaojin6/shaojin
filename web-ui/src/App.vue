<template>
  <el-container class="app-container" v-if="authenticated">
    <el-header class="app-header">
      <h1>🚀 MCP 智能体</h1>
      <el-button text @click="handleLogout" style="color: white; margin-left: auto;">
        <el-icon><SwitchButton /></el-icon>
        退出登录
      </el-button>
    </el-header>
    
    <el-container class="main-container">
      <!-- 左侧导航 -->
      <el-aside width="250px" class="sidebar-container">
        <Sidebar @menu-select="handleMenuSelect" />
      </el-aside>
      
      <!-- 右侧内容区 -->
      <el-main class="content-main">
        <!-- 智能问答 -->
        <ChatWindow v-if="activeView === 'chat'" :session-id="sessionId" />
        
        <!-- MCP 配置（原远程 MCP 服务） -->
        <RemoteMCPConfig v-if="activeView === 'mcp-config'" @config-updated="handleConfigUpdated" />
        
        <!-- 智能Agent配置 -->
        <AgentConfig v-if="activeView === 'agents'" @config-updated="handleConfigUpdated" />
        
        <!-- LLM 配置 -->
        <LLMConfig v-if="activeView === 'llm'" @config-updated="handleConfigUpdated" />
        
        <!-- K8s 配置 -->
        <K8sConfig v-if="activeView === 'k8s'" @config-updated="handleConfigUpdated" />
        
        <!-- 系统状态 -->
        <StatusCard v-if="activeView === 'status'" :status="status" @refresh="loadStatus" />
        
        <!-- 系统日志 -->
        <LogViewer v-if="activeView === 'logs'" />
      </el-main>
    </el-container>
  </el-container>

  <LoginDialog v-model="showLogin" @login-success="handleLoginSuccess" v-else />
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { SwitchButton } from '@element-plus/icons-vue'
import Sidebar from './components/Sidebar.vue'
import ChatWindow from './components/ChatWindow.vue'
import LogViewer from './components/LogViewer.vue'
import RemoteMCPConfig from './components/RemoteMCPConfig.vue'
import AgentConfig from './components/AgentConfig.vue'
import LLMConfig from './components/LLMConfig.vue'
import K8sConfig from './components/K8sConfig.vue'
import StatusCard from './components/StatusCard.vue'
import LoginDialog from './components/LoginDialog.vue'
import { getStatus } from './api/status'
import { checkAuth, logout } from './api/auth'
import { ElMessage } from 'element-plus'

const activeView = ref('chat')
const status = ref(null)
const authenticated = ref(false)
const showLogin = ref(true)
const sessionId = ref('default-session-' + Date.now())

const handleMenuSelect = (menu) => {
  activeView.value = menu
}

const loadStatus = async () => {
  try {
    status.value = await getStatus()
  } catch (error) {
    console.error('Failed to load status:', error)
  }
}

const handleConfigUpdated = () => {
  loadStatus()
}

const handleLoginSuccess = () => {
  authenticated.value = true
  showLogin.value = false
  loadStatus()
}

const handleLogout = async () => {
  try {
    await logout()
    authenticated.value = false
    showLogin.value = true
    ElMessage.success('已退出登录')
  } catch (error) {
    console.error('Logout error:', error)
    authenticated.value = false
    showLogin.value = true
  }
}

const checkAuthentication = async () => {
  try {
    const result = await checkAuth()
    if (result.authenticated) {
      authenticated.value = true
      showLogin.value = false
    } else {
      authenticated.value = false
      showLogin.value = true
    }
  } catch (error) {
    authenticated.value = false
    showLogin.value = true
  }
}

onMounted(async () => {
  await checkAuthentication()
  if (authenticated.value) {
    loadStatus()
    // 每30秒刷新一次状态
    setInterval(loadStatus, 30000)
  }
})
</script>

<style scoped>
.app-container {
  height: 100vh;
}

.app-header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  display: flex;
  align-items: center;
  padding: 0 20px;
  height: 60px;
}

.app-header h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

.main-container {
  height: calc(100vh - 60px);
}

.sidebar-container {
  background: #fff;
  border-right: 1px solid #e4e7ed;
}

.content-main {
  padding: 20px;
  background: #f5f7fa;
  overflow-y: auto;
}
</style>
