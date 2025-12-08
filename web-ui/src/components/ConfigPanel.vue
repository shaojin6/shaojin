<template>
  <el-card class="config-panel" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>⚙️ 配置管理</span>
      </div>
    </template>

    <el-tabs v-model="activeTab">
      <!-- 远程 MCP 服务 -->
      <el-tab-pane label="远程 MCP 服务" name="remote-mcp">
        <div class="remote-mcp-config">
          <el-button type="primary" @click="showAddRemoteMCPDialog = true" style="margin-bottom: 20px">
            <el-icon><Plus /></el-icon>
            添加远程 MCP 服务
          </el-button>

          <el-table :data="remoteMCPs" style="width: 100%">
            <el-table-column label="名称" width="200">
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
                  <div>
                    <div style="font-weight: 500;">{{ scope.row.name }}</div>
                    <div style="font-size: 12px; color: #909399;">{{ scope.row.serverId }}</div>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="type" label="类型" width="100">
              <template #default="scope">
                <el-tag :type="scope.row.type === 'http' ? 'success' : 'info'">
                  {{ scope.row.type.toUpperCase() }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="baseUrl" label="地址" />
            <el-table-column prop="enabled" label="状态" width="80">
              <template #default="scope">
                <el-switch
                  v-model="scope.row.enabled"
                  @change="toggleRemoteMCP(scope.row)"
                />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="220">
              <template #default="scope">
                <el-button size="small" @click="editRemoteMCP(scope.row)">编辑</el-button>
                <el-button size="small" @click="testRemoteMCP(scope.row)">测试</el-button>
                <el-button size="small" type="danger" @click="deleteRemoteMCP(scope.row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 添加/编辑对话框 -->
        <el-dialog
          v-model="showAddRemoteMCPDialog"
          :title="editingRemoteMCP ? `编辑 MCP 服务 (${remoteMCPForm.type.toUpperCase()})` : `添加 MCP 服务 (${remoteMCPForm.type.toUpperCase()})`"
          width="700px"
          :close-on-click-modal="false"
        >
          <el-form :model="remoteMCPForm" label-width="140px" label-position="left">
            <!-- 服务终点 URL -->
            <el-form-item label="服务终点 URL" required>
              <el-input
                v-model="remoteMCPForm.baseUrl"
                placeholder="服务终点的 URL"
                @blur="trimBaseUrl"
              />
              <el-text type="info" size="small" style="display: block; margin-top: 5px;">
                例如: http://k8s-mcp-service.dify.svc.cluster.local:8080
              </el-text>
              <el-text type="warning" size="small" style="display: block; margin-top: 5px;">
                注意：URL 中不要包含空格，会自动清理
              </el-text>
            </el-form-item>

            <!-- 名称和图标 -->
            <el-form-item label="名称和图标">
              <div style="display: flex; gap: 10px; align-items: center;">
                <el-input
                  v-model="remoteMCPForm.name"
                  placeholder="命名你的 MCP 服务"
                  style="flex: 1"
                />
                <el-button
                  :style="{ backgroundColor: remoteMCPForm.iconColor || '#6366f1', color: 'white', border: 'none', width: '40px', height: '40px' }"
                  @click="showIconPicker = true"
                >
                  {{ remoteMCPForm.icon || 'M' }}
                </el-button>
              </div>
            </el-form-item>

            <!-- 服务器标识符 -->
            <el-form-item label="服务器标识符" required>
              <el-input
                v-model="remoteMCPForm.serverId"
                placeholder="服务器唯一标识,例如 my-mcp-server"
                maxlength="24"
                show-word-limit
              />
              <el-text type="info" size="small" style="display: block; margin-top: 5px;">
                工作空间内服务器的唯一标识。支持小写字母、数字、下划线和连字符，最多 24 个字符。
              </el-text>
            </el-form-item>

            <!-- 协议类型 -->
            <el-form-item label="协议类型" required>
              <el-select v-model="remoteMCPForm.type" placeholder="选择协议">
                <el-option label="HTTP/REST API" value="http" />
                <el-option label="WebSocket" value="websocket" />
              </el-select>
            </el-form-item>

            <!-- 超时时间 -->
            <el-form-item label="超时时间">
              <el-input-number
                v-model="remoteMCPForm.timeout"
                :min="1"
                :max="300"
                placeholder="30"
                style="width: 100%"
              />
              <el-text type="info" size="small" style="display: block; margin-top: 5px;">
                请求超时时间（秒）
              </el-text>
            </el-form-item>

            <!-- SSE 读取超时时间 -->
            <el-form-item v-if="remoteMCPForm.type === 'http'" label="SSE 读取超时时间">
              <el-input-number
                v-model="remoteMCPForm.sseReadTimeout"
                :min="1"
                :max="600"
                placeholder="300"
                style="width: 100%"
              />
              <el-text type="info" size="small" style="display: block; margin-top: 5px;">
                SSE 读取超时时间（秒）
              </el-text>
            </el-form-item>

            <!-- 请求头 -->
            <el-form-item label="请求头">
              <el-text type="info" size="small" style="display: block; margin-bottom: 10px;">
                发送到 MCP 服务器的额外 HTTP 请求头
              </el-text>
              <div style="border: 1px solid #dcdfe6; border-radius: 4px; padding: 10px;">
                <el-table :data="remoteMCPForm.headers" style="width: 100%" border>
                  <el-table-column label="请求头名称" width="200">
                    <template #default="scope">
                      <el-input
                        v-model="scope.row.name"
                        placeholder="例如: Authorization"
                        size="small"
                      />
                    </template>
                  </el-table-column>
                  <el-table-column label="请求头值">
                    <template #default="scope">
                      <el-input
                        v-model="scope.row.value"
                        type="password"
                        show-password
                        placeholder="例如: Bearer token123"
                        size="small"
                      />
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="80">
                    <template #default="scope">
                      <el-button
                        type="danger"
                        size="small"
                        text
                        @click="removeHeader(scope.$index)"
                      >
                        删除
                      </el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <el-button
                  type="primary"
                  text
                  @click="addHeader"
                  style="margin-top: 10px"
                >
                  <el-icon><Plus /></el-icon>
                  添加请求头
                </el-button>
              </div>
            </el-form-item>

            <!-- 工具端点（高级选项） -->
            <el-collapse>
              <el-collapse-item name="advanced" title="高级选项">
                <el-form-item label="工具端点">
                  <el-input
                    v-model="remoteMCPForm.toolsEndpoint"
                    placeholder="/api/tools (默认，会自动尝试多个常见路径)"
                  />
                  <el-text type="info" size="small" style="display: block; margin-top: 5px;">
                    工具列表 API 端点。留空会自动尝试: /api/tools, /tools, /mcp/tools, /v1/tools
                  </el-text>
                </el-form-item>
              </el-collapse-item>
            </el-collapse>
          </el-form>

          <template #footer>
            <el-button @click="showAddRemoteMCPDialog = false">取消</el-button>
            <el-button type="primary" @click="saveRemoteMCP" :loading="remoteMCPLoading">
              {{ editingRemoteMCP ? '保存' : '添加并授权' }}
            </el-button>
          </template>
        </el-dialog>

        <!-- 图标选择器（简化版） -->
        <el-dialog
          v-model="showIconPicker"
          title="选择图标"
          width="400px"
        >
          <div style="display: grid; grid-template-columns: repeat(8, 1fr); gap: 10px;">
            <el-button
              v-for="icon in ['M', 'K', 'S', 'A', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'L', 'N', 'O', 'P']"
              :key="icon"
              :style="{ backgroundColor: remoteMCPForm.iconColor || '#6366f1', color: 'white' }"
              @click="selectIcon(icon)"
            >
              {{ icon }}
            </el-button>
          </div>
          <div style="margin-top: 20px;">
            <el-text>选择颜色：</el-text>
            <el-color-picker v-model="remoteMCPForm.iconColor" />
          </div>
        </el-dialog>
      </el-tab-pane>

      <!-- LLM 配置 -->
      <el-tab-pane label="LLM 配置" name="llm">
        <el-form :model="llmConfig" label-width="120px" label-position="left">
          <el-form-item label="提供商">
            <el-select v-model="llmConfig.provider" placeholder="选择 LLM 提供商">
              <el-option label="百炼平台（推荐）" value="bailian" />
              <el-option label="DashScope（旧版）" value="dashscope" />
              <el-option label="OpenAI" value="openai" />
              <el-option label="Ollama" value="ollama" />
            </el-select>
          </el-form-item>

          <el-form-item label="API 地址">
            <el-input 
              v-model="llmConfig.baseUrl" 
              placeholder="https://dashscope.aliyuncs.com/api/v1"
            />
            <el-text type="info" size="small" style="display: block; margin-top: 5px;">
              百炼平台和 DashScope 使用相同的 API 地址
            </el-text>
          </el-form-item>

          <el-form-item label="模型">
            <el-select v-model="llmConfig.model" placeholder="选择模型">
              <el-option label="qwen-plus（推荐）" value="qwen-plus" />
              <el-option label="qwen-turbo（快速）" value="qwen-turbo" />
              <el-option label="qwen-max（最强）" value="qwen-max" />
            </el-select>
          </el-form-item>

          <el-form-item label="API Key">
            <el-input 
              v-model="llmConfig.apiKey" 
              type="password" 
              show-password
              placeholder="输入你的百炼平台 API Key"
            />
            <el-text type="info" size="small" style="display: block; margin-top: 5px;">
              <el-link href="https://modelstudio.aliyun.com/" target="_blank" type="primary">
                前往百炼平台获取 API Key
              </el-link>
            </el-text>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" @click="saveLLMConfig" :loading="llmLoading">
              保存配置
            </el-button>
            <el-button @click="testLLMConnection" :loading="llmTesting">
              测试连接
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- K8s 配置 -->
      <el-tab-pane label="K8s 配置" name="k8s">
        <el-form :model="k8sConfig" label-width="120px" label-position="left">
          <el-form-item label="配置模式">
            <el-radio-group v-model="k8sConfig.mode">
              <el-radio label="kubeconfig">Kubeconfig 文件</el-radio>
              <el-radio label="manual">手动配置</el-radio>
            </el-radio-group>
          </el-form-item>

          <template v-if="k8sConfig.mode === 'kubeconfig'">
            <el-form-item label="Kubeconfig">
              <el-upload
                :auto-upload="false"
                :on-change="handleKubeconfigUpload"
                :file-list="kubeconfigFileList"
                :limit="1"
                accept=".yaml,.yml"
              >
                <el-button type="primary">选择文件</el-button>
                <template #tip>
                  <div class="el-upload__tip">
                    上传 kubeconfig 文件（YAML 格式）
                  </div>
                </template>
              </el-upload>
            </el-form-item>
          </template>

          <template v-if="k8sConfig.mode === 'manual'">
            <el-form-item label="API Server" required>
              <el-input
                v-model="k8sConfig.server"
                placeholder="https://11.0.1.110:6443"
              />
            </el-form-item>

            <el-form-item label="认证方式">
              <el-radio-group v-model="k8sAuthType">
                <el-radio label="token">Bearer Token</el-radio>
                <el-radio label="username">用户名/密码</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item v-if="k8sAuthType === 'token'" label="Token" required>
              <el-input
                v-model="k8sConfig.token"
                type="password"
                show-password
                placeholder="输入 Bearer Token"
              />
            </el-form-item>

            <template v-if="k8sAuthType === 'username'">
              <el-form-item label="用户名" required>
                <el-input v-model="k8sConfig.username" placeholder="输入用户名" />
              </el-form-item>
              <el-form-item label="密码" required>
                <el-input
                  v-model="k8sConfig.password"
                  type="password"
                  show-password
                  placeholder="输入密码"
                />
              </el-form-item>
            </template>

            <el-form-item label="跳过 TLS 验证">
              <el-switch v-model="k8sInsecure" />
            </el-form-item>

            <el-form-item label="CA 证书（可选）">
              <el-input
                v-model="k8sConfig.caData"
                type="textarea"
                :rows="3"
                placeholder="PEM 格式的 CA 证书内容"
              />
            </el-form-item>
          </template>

          <el-form-item label="默认命名空间">
            <el-input
              v-model="k8sConfig.namespace"
              placeholder="default"
            />
          </el-form-item>

          <el-form-item>
            <el-button type="primary" @click="saveK8sConfig" :loading="k8sLoading">
              保存配置
            </el-button>
            <el-button @click="testK8sConnection" :loading="k8sTesting">
              测试连接
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </el-card>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getConfig, saveLLMConfig as saveLLM, testLLM, testK8s, saveK8sConfig as saveK8s, getRemoteMCPs, addRemoteMCP, updateRemoteMCP, deleteRemoteMCP as deleteRemoteMCPAPI, testRemoteMCP as testRemoteMCPAPI } from '../api/config'

const emit = defineEmits(['config-updated'])

const activeTab = ref('llm')
const llmLoading = ref(false)
const llmTesting = ref(false)
const k8sTesting = ref(false)

// 远程 MCP 配置
const remoteMCPs = ref([])
const showAddRemoteMCPDialog = ref(false)
const editingRemoteMCP = ref(null)
const remoteMCPLoading = ref(false)
const remoteMCPForm = reactive({
  name: '',
  serverId: '',
  type: 'http',
  baseUrl: '',
  icon: 'M',
  iconColor: '#6366f1',
  timeout: 30,
  sseReadTimeout: 300,
  headers: [],
  toolsEndpoint: ''
})
const showIconPicker = ref(false)

const llmConfig = reactive({
  provider: 'bailian',
  baseUrl: 'https://dashscope.aliyuncs.com/api/v1',
  model: 'qwen-plus',
  apiKey: ''
})

const k8sConfig = reactive({
  id: '',
  mode: 'manual',
  server: '',
  namespace: 'default',
  token: '',
  username: '',
  password: '',
  caData: ''
})
const k8sAuthType = ref('token')
const k8sInsecure = ref(false)
const k8sLoading = ref(false)
const kubeconfigFileList = ref([])

const loadConfig = async () => {
  try {
    const config = await getConfig()
    if (config.llm) {
      Object.assign(llmConfig, config.llm)
      // 不显示完整的 API Key，只显示前几位
      if (llmConfig.apiKey && llmConfig.apiKey.length > 8) {
        llmConfig.apiKey = llmConfig.apiKey.substring(0, 8) + '...'
      }
    }
    if (config.k8s) {
      Object.assign(k8sConfig, config.k8s)
      // 根据配置判断认证方式
      if (config.k8s.token) {
        k8sAuthType.value = 'token'
      } else if (config.k8s.username) {
        k8sAuthType.value = 'username'
      }
      // 设置 insecure 标志
      if (config.k8s.insecure !== undefined) {
        k8sInsecure.value = config.k8s.insecure
      }
      // 不显示敏感信息
      if (k8sConfig.token && k8sConfig.token.length > 8) {
        k8sConfig.token = k8sConfig.token.substring(0, 8) + '...'
      }
      if (k8sConfig.password && k8sConfig.password.length > 0) {
        k8sConfig.password = '***'
      }
    }
    // 加载远程 MCP 服务
    await loadRemoteMCPs()
  } catch (error) {
    console.error('Failed to load config:', error)
  }
}

const loadRemoteMCPs = async () => {
  try {
    remoteMCPs.value = await getRemoteMCPs()
  } catch (error) {
    console.error('Failed to load remote MCPs:', error)
  }
}

const saveRemoteMCP = async () => {
  // 清理 baseURL
  trimBaseUrl()

  // 验证必填字段
  if (!remoteMCPForm.name || !remoteMCPForm.baseUrl || !remoteMCPForm.serverId) {
    ElMessage.warning('请填写服务名称、访问地址和服务器标识符')
    return
  }

  // 验证 URL 格式
  try {
    new URL(remoteMCPForm.baseUrl)
  } catch (e) {
    ElMessage.warning('请输入有效的 URL 地址')
    return
  }

  // 验证服务器标识符格式（小写字母、数字、下划线、连字符，最多24字符）
  const serverIdPattern = /^[a-z0-9_-]{1,24}$/
  if (!serverIdPattern.test(remoteMCPForm.serverId)) {
    ElMessage.warning('服务器标识符只能包含小写字母、数字、下划线和连字符，最多24个字符')
    return
  }

  remoteMCPLoading.value = true
  try {
    // 将 headers 数组转换为对象
    const headersObj = {}
    remoteMCPForm.headers.forEach(header => {
      if (header.name && header.value) {
        headersObj[header.name] = header.value
      }
    })

    const config = {
      name: remoteMCPForm.name,
      serverId: remoteMCPForm.serverId,
      type: remoteMCPForm.type,
      baseUrl: remoteMCPForm.baseUrl,
      icon: remoteMCPForm.icon,
      timeout: remoteMCPForm.timeout || 30,
      sseReadTimeout: remoteMCPForm.sseReadTimeout || 300,
      headers: headersObj,
      toolsEndpoint: remoteMCPForm.toolsEndpoint || '/api/tools',
      enabled: true
    }

    if (editingRemoteMCP.value) {
      await updateRemoteMCP(editingRemoteMCP.value.serverId, config)
      ElMessage.success('远程 MCP 服务更新成功')
    } else {
      await addRemoteMCP(config)
      ElMessage.success('远程 MCP 服务添加成功')
    }

    showAddRemoteMCPDialog.value = false
    resetRemoteMCPForm()
    await loadRemoteMCPs()
    emit('config-updated')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    remoteMCPLoading.value = false
  }
}

const resetRemoteMCPForm = () => {
  editingRemoteMCP.value = null
  Object.assign(remoteMCPForm, {
    name: '',
    serverId: '',
    type: 'http',
    baseUrl: '',
    icon: 'M',
    iconColor: '#6366f1',
    timeout: 30,
    sseReadTimeout: 300,
    headers: [],
    toolsEndpoint: ''
  })
}

const addHeader = () => {
  remoteMCPForm.headers.push({ name: '', value: '' })
}

const removeHeader = (index) => {
  remoteMCPForm.headers.splice(index, 1)
}

const selectIcon = (icon) => {
  remoteMCPForm.icon = icon
  showIconPicker.value = false
}

const trimBaseUrl = () => {
  // 自动清理 URL 中的空格
  if (remoteMCPForm.baseUrl) {
    remoteMCPForm.baseUrl = remoteMCPForm.baseUrl.replace(/\s+/g, '').trim()
  }
}

const editRemoteMCP = (mcp) => {
  editingRemoteMCP.value = mcp
  Object.assign(remoteMCPForm, {
    name: mcp.name || '',
    serverId: mcp.serverId || '',
    type: mcp.type || 'http',
    baseUrl: mcp.baseUrl || '',
    icon: mcp.icon || 'M',
    iconColor: mcp.iconColor || '#6366f1',
    timeout: mcp.timeout || 30,
    sseReadTimeout: mcp.sseReadTimeout || 300,
    headers: mcp.headers ? Object.entries(mcp.headers).map(([name, value]) => ({ name, value })) : [],
    toolsEndpoint: mcp.toolsEndpoint || ''
  })
  showAddRemoteMCPDialog.value = true
}

const toggleRemoteMCP = async (mcp) => {
  try {
    await updateRemoteMCP(mcp.serverId || mcp.name, { ...mcp, enabled: mcp.enabled })
    ElMessage.success(mcp.enabled ? '服务已启用' : '服务已禁用')
    emit('config-updated')
  } catch (error) {
    ElMessage.error('操作失败: ' + (error.response?.data?.error || error.message))
    mcp.enabled = !mcp.enabled // 回滚
  }
}

const testRemoteMCP = async (mcp) => {
  try {
    const identifier = mcp.serverId || mcp.name
    const result = await testRemoteMCPAPI(identifier)
    if (result.status === 'ok') {
      ElMessage.success('连接测试成功')
    } else {
      ElMessage.error('连接失败: ' + result.message)
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.error || error.message))
  }
}

const deleteRemoteMCP = async (mcp) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除远程 MCP 服务 "${mcp.name}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    const identifier = mcp.serverId || mcp.name
    await deleteRemoteMCPAPI(identifier)
    ElMessage.success('删除成功')
    await loadRemoteMCPs()
    emit('config-updated')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.response?.data?.error || error.message))
    }
  }
}

const saveLLMConfig = async () => {
  if (!llmConfig.apiKey) {
    ElMessage.warning('请输入 API Key')
    return
  }

  llmLoading.value = true
  try {
    await saveLLM(llmConfig)
    ElMessage.success('LLM 配置保存成功')
    emit('config-updated')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    llmLoading.value = false
  }
}

const testLLMConnection = async () => {
  if (!llmConfig.apiKey) {
    ElMessage.warning('请先配置 API Key')
    return
  }

  llmTesting.value = true
  try {
    // 先保存配置
    await saveLLM(llmConfig)
    
    // 然后测试
    const result = await testLLM()
    if (result.status === 'ok') {
      ElMessage.success('LLM 连接测试成功')
    } else {
      ElMessage.error('连接失败: ' + result.message)
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.message || error.message))
  } finally {
    llmTesting.value = false
  }
}

const handleKubeconfigUpload = (file) => {
  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target.result
    // 转换为 base64
    k8sConfig.content = btoa(content)
    kubeconfigFileList.value = [file]
  }
  reader.readAsText(file.raw)
}

const saveK8sConfig = async () => {
  if (k8sConfig.mode === 'manual') {
    if (!k8sConfig.server) {
      ElMessage.warning('请输入 API Server 地址')
      return
    }
    if (k8sAuthType.value === 'token' && !k8sConfig.token) {
      ElMessage.warning('请输入 Token')
      return
    }
    if (k8sAuthType.value === 'username' && (!k8sConfig.username || !k8sConfig.password)) {
      ElMessage.warning('请输入用户名和密码')
      return
    }
  } else {
    if (!k8sConfig.content) {
      ElMessage.warning('请上传 kubeconfig 文件')
      return
    }
  }

  k8sLoading.value = true
  try {
    const configToSave = {
      ...k8sConfig,
      mode: k8sConfig.mode
    }

    if (k8sConfig.mode === 'manual') {
      // 清理不需要的字段
      if (k8sAuthType.value === 'token') {
        delete configToSave.username
        delete configToSave.password
      } else {
        delete configToSave.token
      }
      // 设置 insecure（明确设置，确保 false 值也会保存）
      configToSave.insecure = k8sInsecure.value || false
    } else {
      // kubeconfig 模式，清理手动配置字段
      delete configToSave.server
      delete configToSave.token
      delete configToSave.username
      delete configToSave.password
      delete configToSave.caData
    }

    await saveK8s(configToSave)
    ElMessage.success('K8s 配置保存成功')
    emit('config-updated')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    k8sLoading.value = false
  }
}

const testK8sConnection = async () => {
  k8sTesting.value = true
  try {
    // 如果配置有 ID，传递 ID；否则不传递（使用默认配置）
    const result = await testK8s(k8sConfig.id)
    if (result.status === 'ok') {
      ElMessage.success('K8s 连接测试成功: ' + result.message)
    } else {
      ElMessage.error('连接失败: ' + result.message)
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.message || error.message))
  } finally {
    k8sTesting.value = false
  }
}

// 监听对话框关闭，重置表单
watch(showAddRemoteMCPDialog, (val) => {
  if (!val) {
    resetRemoteMCPForm()
  }
})

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.config-panel {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}
</style>

