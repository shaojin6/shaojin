<template>
  <el-card class="config-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>MCP 配置</span>
      </div>
    </template>

    <el-button type="primary" @click="handleAdd" style="margin-bottom: 20px">
      <el-icon><Plus /></el-icon>
      添加 MCP 服务
    </el-button>

    <el-table
      :data="remoteMCPs"
      style="width: 100%"
      v-loading="loadingData"
      @expand-change="handleExpandChange"
      row-key="serverId"
    >
      <template #empty>
        <el-empty description="暂无 MCP 服务" />
      </template>
      <el-table-column type="expand">
        <template #default="scope">
          <div class="tools-expand">
            <div class="tools-expand-header">
              <el-text size="small" type="info">
                工具列表（{{ scope.row.toolsCount !== undefined && scope.row.toolsCount !== null ? scope.row.toolsCount : '未加载' }}）
              </el-text>
              <el-button
                size="small"
                text
                :loading="scope.row.refreshingTools"
                @click="refreshToolsFromCache(scope.row)"
              >
                <el-icon><Refresh /></el-icon>
                本地刷新
              </el-button>
            </div>
            <div v-if="scope.row.loadingTools" class="tools-expand-body">
              <el-skeleton :rows="3" animated />
            </div>
            <div v-else-if="scope.row.tools && scope.row.tools.length" class="tools-expand-body">
              <div
                v-for="tool in scope.row.tools"
                :key="tool.name"
                class="tool-chip"
              >
                <div class="tool-chip-title">{{ tool.annotations?.title || tool.name }}</div>
                <div class="tool-chip-desc">{{ tool.description }}</div>
              </div>
            </div>
            <div v-else class="tools-expand-body tools-expand-empty">
              暂无工具，请先点击“测试”同步工具列表。
            </div>
          </div>
        </template>
      </el-table-column>
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
      <el-table-column label="工具" width="140">
        <template #default="scope">
          <el-button 
            size="small" 
            text 
            @click="showTools(scope.row)"
            :disabled="!scope.row.enabled"
          >
            <el-icon><Box /></el-icon>
            {{ scope.row.toolsCount !== undefined ? `${scope.row.toolsCount} 个工具` : '查看工具' }}
          </el-button>
        </template>
      </el-table-column>
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
          <el-button size="small" @click="handleEdit(scope.row)">编辑</el-button>
          <el-button size="small" @click="testRemoteMCP(scope.row)">测试</el-button>
          <el-button size="small" type="danger" @click="handleDelete(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑对话框 -->
    <el-dialog
      v-model="showDialog"
      :title="editingItem ? `编辑 MCP 服务 (${form.type.toUpperCase()})` : `添加 MCP 服务 (${form.type.toUpperCase()})`"
      width="700px"
      :close-on-click-modal="false"
      @close="resetForm"
    >
      <el-form :model="form" label-width="140px" label-position="left">
        <!-- 服务终点 URL -->
        <el-form-item label="服务终点 URL" required>
          <el-input
            v-model="form.baseUrl"
            placeholder="服务终点的 URL"
            @blur="trimBaseUrl"
          />
          <el-text type="info" size="small" style="display: block; margin-top: 5px;">
            例如: http://11.0.1.110:30080/mcp
          </el-text>
        </el-form-item>

        <!-- 名称和图标 -->
        <el-form-item label="名称和图标">
          <div style="display: flex; gap: 10px; align-items: center;">
            <el-input
              v-model="form.name"
              placeholder="命名你的 MCP 服务"
              style="flex: 1"
            />
            <el-button
              :style="{ backgroundColor: form.iconColor || '#6366f1', color: 'white', border: 'none', width: '40px', height: '40px' }"
              @click="showIconPicker = true"
            >
              {{ form.icon || 'M' }}
            </el-button>
          </div>
        </el-form-item>

        <!-- 服务器标识符 -->
        <el-form-item label="服务器标识符" required>
          <el-input
            v-model="form.serverId"
            placeholder="服务器唯一标识,例如 my-mcp-server"
            maxlength="24"
            show-word-limit
            :disabled="!!editingItem"
          />
          <el-text type="info" size="small" style="display: block; margin-top: 5px;">
            工作空间内服务器的唯一标识。支持小写字母、数字、下划线和连字符，最多 24 个字符。
            <span v-if="editingItem" style="color: #f56c6c;">编辑时不可修改</span>
          </el-text>
        </el-form-item>

        <!-- 协议类型 -->
        <el-form-item label="协议类型" required>
          <el-select v-model="form.type" placeholder="选择协议" style="width: 100%">
            <el-option label="HTTP/REST API" value="http" />
            <el-option label="WebSocket" value="websocket" />
          </el-select>
        </el-form-item>

        <!-- 超时时间 -->
        <el-form-item label="超时时间">
          <el-input-number
            v-model="form.timeout"
            :min="1"
            :max="300"
            placeholder="30"
            style="width: 100%"
          />
        </el-form-item>

        <!-- SSE 读取超时时间 -->
        <el-form-item v-if="form.type === 'http'" label="SSE 读取超时时间">
          <el-input-number
            v-model="form.sseReadTimeout"
            :min="1"
            :max="600"
            placeholder="300"
            style="width: 100%"
          />
        </el-form-item>

        <!-- 请求头 -->
        <el-form-item label="请求头">
          <div class="headers-container">
            <el-text type="info" size="small" style="display: block; margin-bottom: 10px;">
              发送到 MCP 服务器的额外 HTTP 请求头
            </el-text>
            <div
              v-for="(header, index) in form.headers"
              :key="index"
              class="header-row"
            >
              <el-input
                v-model="header.name"
                class="header-input name"
                placeholder="例如：Authorization"
              />
              <el-input
                v-model="header.value"
                class="header-input value"
                placeholder="例如：Bearer token123 或 API Key"
                type="textarea"
                :autosize="{ minRows: 1, maxRows: 3 }"
              />
              <el-button
                text
                type="danger"
                class="header-remove-btn"
                @click="removeHeader(index)"
              >
                删除
              </el-button>
            </div>
            <div class="header-actions">
              <el-button type="primary" link @click="addHeader">
                <el-icon><Plus /></el-icon>
                添加请求头
              </el-button>
            </div>
          </div>
        </el-form-item>

        <!-- 工具端点（高级选项） -->
        <el-collapse>
          <el-collapse-item name="advanced" title="高级选项">
            <el-form-item label="工具端点">
              <div style="display: flex; gap: 10px; align-items: center;">
                <el-input
                  v-model="form.toolsEndpoint"
                  placeholder="/api/tools (默认，会自动尝试多个常见路径)"
                  style="flex: 1"
                />
                <el-button 
                  type="primary" 
                  @click="testEndpointPath"
                  :loading="testingEndpoint"
                  :disabled="!form.baseUrl"
                >
                  测试端点
                </el-button>
              </div>
            </el-form-item>
          </el-collapse-item>
        </el-collapse>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRemoteMCP" :loading="loading">
          {{ editingItem ? '保存' : '添加并授权' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 图标选择器 -->
    <el-dialog
      v-model="showIconPicker"
      title="选择图标"
      width="400px"
    >
      <div style="display: grid; grid-template-columns: repeat(8, 1fr); gap: 10px;">
        <el-button
          v-for="icon in ['M', 'K', 'S', 'A', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'L', 'N', 'O', 'P']"
          :key="icon"
          :style="{ backgroundColor: form.iconColor || '#6366f1', color: 'white' }"
          @click="selectIcon(icon)"
        >
          {{ icon }}
        </el-button>
      </div>
      <div style="margin-top: 20px;">
        <el-text>选择颜色：</el-text>
        <el-color-picker v-model="form.iconColor" />
      </div>
    </el-dialog>

    <!-- 工具列表对话框 -->
    <el-dialog
      v-model="showToolsDialog"
      :title="`${currentMCPName} - 工具列表`"
      width="800px"
    >
      <div v-loading="loadingTools">
        <div v-if="toolsList.length === 0" class="empty-tools">
          <el-empty description="暂无工具或服务未启用" />
        </div>
        <div v-else>
          <div style="margin-bottom: 16px;">
            <el-text type="info">共 {{ toolsList.length }} 个工具</el-text>
            <el-button 
              text 
              size="small" 
              @click="refreshTools"
              style="margin-left: 10px"
            >
              <el-icon><Refresh /></el-icon>
              Remote MCP刷新
            </el-button>
          </div>
          <div class="tools-list">
            <div
              v-for="(tool, index) in toolsList"
              :key="index"
              class="tool-item"
            >
              <div class="tool-header">
                <el-text strong>{{ tool.name }}</el-text>
              </div>
              <div class="tool-description">
                <el-text type="info" size="small">{{ tool.description || '无描述' }}</el-text>
              </div>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Box, Refresh } from '@element-plus/icons-vue'
import { getRemoteMCPs, addRemoteMCP, updateRemoteMCP, deleteRemoteMCP as deleteRemoteMCPAPI, testRemoteMCP as testRemoteMCPAPI, testEndpoint, getRemoteMCPTools } from '../api/config'

const emit = defineEmits(['config-updated'])

const remoteMCPs = ref([])
const showDialog = ref(false)
const showIconPicker = ref(false)
const editingItem = ref(null)
const loading = ref(false)
const loadingData = ref(false)
const testingEndpoint = ref(false)
const showToolsDialog = ref(false)
const currentMCPName = ref('')
const currentMCPId = ref('')
const toolsList = ref([])
const loadingTools = ref(false)

const form = reactive({
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

const loadRemoteMCPs = async () => {
  loadingData.value = true
  try {
    const data = await getRemoteMCPs()
    // 确保返回的是数组
    if (Array.isArray(data)) {
      remoteMCPs.value = data
      console.log('Loaded remote MCPs:', remoteMCPs.value.length, 'items')
      
      // 先初始化工具列表为空，避免显示"未加载"
      remoteMCPs.value.forEach(mcp => {
        if (mcp.tools && mcp.tools.length > 0) {
          // 如果配置中已有工具数据（从数据库加载），直接使用
          mcp.toolsCount = mcp.tools.length
        } else {
          // 否则初始化为空
          mcp.tools = []
          mcp.toolsCount = 0
        }
      })
      
      // 关闭加载状态，立即显示列表
      loadingData.value = false
      
      // 异步加载所有启用的 MCP 服务的工具列表（从缓存，不阻塞）
      // 使用 Promise.allSettled 并行加载，后台执行，不阻塞 UI
      const loadPromises = remoteMCPs.value
        .filter(mcp => mcp.enabled) // 只加载启用的服务
        .map(mcp => {
          const identifier = mcp.serverId || mcp.name
          // 计算超时时间：普通加载时使用超时时间（毫秒），默认 60 秒 = 60000 毫秒
          const timeout = (mcp.timeout || 60) * 1000
          // 静默加载，不显示错误（因为可能还没有工具）
          return getRemoteMCPTools(identifier, false, timeout)
            .then(result => {
              if (result && result.tools && result.tools.length > 0) {
                mcp.tools = result.tools
                mcp.toolsCount = result.tools.length
              } else {
                // 如果没有缓存，保持为空
                mcp.tools = []
                mcp.toolsCount = 0
              }
            })
            .catch(() => {
              // 加载失败时，保持为空（不显示错误）
              mcp.tools = []
              mcp.toolsCount = 0
            })
        })
      
      // 并行加载所有工具列表（后台执行，不等待完成）
      Promise.allSettled(loadPromises).then(() => {
        console.log('All tool lists loaded from cache')
      })
    } else {
      console.warn('Unexpected data format:', data)
      remoteMCPs.value = []
      loadingData.value = false
    }
  } catch (error) {
    console.error('Failed to load remote MCPs:', error)
    ElMessage.error('加载 MCP 服务失败: ' + (error.response?.data?.error || error.message))
    remoteMCPs.value = []
    loadingData.value = false
  }
}

const handleAdd = () => {
  editingItem.value = null
  resetForm()
  showDialog.value = true
}

const handleEdit = (mcp) => {
  editingItem.value = mcp
  // 使用深拷贝避免修改原始对象
  Object.assign(form, JSON.parse(JSON.stringify({
    name: mcp.name || '',
    serverId: mcp.serverId || '',
    type: mcp.type || 'http',
    baseUrl: mcp.baseUrl || '',
    icon: mcp.icon || 'M',
    iconColor: mcp.iconColor || '#6366f1',
    timeout: mcp.timeout || 0, // 从数据库读取，如果为0则前端表单会显示默认值
    sseReadTimeout: mcp.sseReadTimeout || 0, // 从数据库读取，如果为0则前端表单会显示默认值
    headers: mcp.headers
      ? Object.entries(mcp.headers).map(([name, value]) => ({ name, value }))
      : [],
    toolsEndpoint: mcp.toolsEndpoint || ''
  })))
  showDialog.value = true
}

const resetForm = () => {
  editingItem.value = null
  Object.assign(form, {
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

const saveRemoteMCP = async () => {
  trimBaseUrl()

  if (!form.name || !form.baseUrl) {
    ElMessage.warning('请填写服务名称和服务地址')
    return
  }

  try {
    new URL(form.baseUrl)
  } catch (e) {
    ElMessage.warning('请输入有效的 URL 地址')
    return
  }

  // 自动生成 serverId（如果未填写）
  if (!form.serverId) {
    // 从名称生成 serverId
    form.serverId = form.name
      .toLowerCase()
      .replace(/[^a-z0-9_-]/g, '-')
      .replace(/-+/g, '-')
      .substring(0, 24)
      .replace(/^-|-$/g, '')
    
    if (!form.serverId) {
      form.serverId = 'mcp-' + Date.now().toString().slice(-10)
    }
  }

  const serverIdPattern = /^[a-z0-9_-]{1,24}$/
  if (!serverIdPattern.test(form.serverId)) {
    ElMessage.warning('服务器标识符只能包含小写字母、数字、下划线和连字符，最多24个字符')
    return
  }

  // 检查名称或服务器标识符是否已存在（仅在添加时检查）
  if (!editingItem.value) {
    const existingByName = remoteMCPs.value.find(mcp => mcp.name === form.name)
    const existingById = remoteMCPs.value.find(mcp => mcp.serverId === form.serverId)
    
    if (existingByName || existingById) {
      let errorMsg = '名称或服务器标识符已存在'
      if (existingByName && existingById && existingByName.serverId === existingById.serverId) {
        errorMsg = `名称 "${form.name}" 和服务器标识符 "${form.serverId}" 已存在`
      } else if (existingByName) {
        errorMsg = `名称 "${form.name}" 已存在`
      } else if (existingById) {
        errorMsg = `服务器标识符 "${form.serverId}" 已存在`
      }
      ElMessage.warning(errorMsg)
      return
    }
  }

  loading.value = true
  try {
    const headersObj = {}

    // 添加自定义请求头
    form.headers.forEach(header => {
      if (header.name && header.value) {
        headersObj[header.name.trim()] = header.value.trim()
      }
    })

    const config = {
      name: form.name,
      serverId: form.serverId,
      type: form.type,
      baseUrl: form.baseUrl,
      icon: form.icon,
      iconColor: form.iconColor,
      timeout: form.timeout || 0, // 从前端传入，如果为0则使用数据库中的值
      sseReadTimeout: form.sseReadTimeout || 0, // 从前端传入，如果为0则使用数据库中的值
      headers: headersObj,
      toolsEndpoint: form.toolsEndpoint || '',
      enabled: true
    }

    const serviceName = form.name || form.serverId || '未命名服务'
    if (editingItem.value) {
      // 编辑时使用原始的 serverId（不可修改）
      await updateRemoteMCP(editingItem.value.serverId, config)
      ElMessage.success(`${serviceName} 配置更新成功`)
    } else {
      await addRemoteMCP(config)
      ElMessage.success(`${serviceName} 配置添加成功`)
    }

    showDialog.value = false
    resetForm()
    // 立即重新加载数据
    await loadRemoteMCPs()
    emit('config-updated')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
    console.error('Save error:', error)
  } finally {
    loading.value = false
  }
}

const handleDelete = async (mcp) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除 MCP 服务 "${mcp.name}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    const identifier = mcp.serverId || mcp.name
    await deleteRemoteMCPAPI(identifier)
    const serviceName = mcp.name || mcp.serverId || '未命名服务'
    ElMessage.success(`${serviceName} 删除成功`)
    await loadRemoteMCPs()
    emit('config-updated')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.response?.data?.error || error.message))
    }
  }
}

const toggleRemoteMCP = async (mcp) => {
  try {
    await updateRemoteMCP(mcp.serverId || mcp.name, { ...mcp, enabled: mcp.enabled })
    const serviceName = mcp.name || mcp.serverId || '未命名服务'
    ElMessage.success(mcp.enabled ? `${serviceName} 已启用` : `${serviceName} 已禁用`)
    emit('config-updated')
  } catch (error) {
    ElMessage.error('操作失败: ' + (error.response?.data?.error || error.message))
    mcp.enabled = !mcp.enabled
  }
}

// 测试端点路径
const testEndpointPath = async () => {
  if (!form.baseUrl) {
    ElMessage.warning('请先填写服务地址')
    return
  }

  testingEndpoint.value = true
  try {
    // 构建 headers
    const headersObj = {}
    form.headers.forEach(header => {
      if (header.name && header.value) {
        headersObj[header.name.trim()] = header.value.trim()
      }
    })

    const result = await testEndpoint(form.baseUrl, form.toolsEndpoint, headersObj)
    
    if (result.status === 'ok') {
      ElMessage.success(`端点测试成功！找到 ${result.tools} 个工具。建议使用端点: ${result.endpoint}`)
      // 自动填充找到的端点
      if (!form.toolsEndpoint) {
        form.toolsEndpoint = result.endpoint
      }
      // 如果找到工具列表，显示工具对话框
      if (result.toolsList && result.toolsList.length > 0) {
        toolsList.value = result.toolsList
        currentMCPName.value = form.name || '测试服务'
        showToolsDialog.value = true
      }
    } else {
      // 显示详细的测试结果
      let message = '端点测试失败。尝试的端点：\n'
      if (result.details && result.details.length > 0) {
        result.details.forEach((detail, index) => {
          message += `${index + 1}. ${detail.endpoint}: ${detail.message}\n`
        })
      }
      ElMessage.warning(message)
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.error || error.message))
  } finally {
    testingEndpoint.value = false
  }
}

const testRemoteMCP = async (mcp) => {
  try {
    const identifier = mcp.serverId || mcp.name
    const result = await testRemoteMCPAPI(identifier)
    if (result.status === 'ok') {
      // 更新工具数量
      mcp.toolsCount = result.count || 0
      ElMessage.success(`连接测试成功，找到 ${result.count || 0} 个工具`)
      // 如果有工具列表，直接显示
      if (result.tools && result.tools.length > 0) {
        toolsList.value = result.tools
        currentMCPName.value = mcp.name || mcp.serverId
        currentMCPId.value = identifier
        showToolsDialog.value = true
      } else {
        // 否则加载工具列表
        await showTools(mcp)
      }
    } else {
      ElMessage.error('连接失败: ' + result.message)
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.error || error.message))
  }
}

// 显示工具列表
const showTools = async (mcp) => {
  if (!mcp.enabled) {
    ElMessage.warning('请先启用该 MCP 服务')
    return
  }
  
  currentMCPName.value = mcp.name || mcp.serverId
  currentMCPId.value = mcp.serverId || mcp.name
  showToolsDialog.value = true
  await loadTools()
}

// 加载工具列表
const loadTools = async (forceRefresh = false) => {
  loadingTools.value = true
  try {
    // 获取当前 MCP 配置以获取超时时间
    const currentMCP = remoteMCPs.value.find(mcp => 
      (mcp.serverId || mcp.name) === currentMCPId.value
    )
    
    // 计算超时时间：如果是刷新，使用 SSE 读取超时时间；否则使用普通超时时间
    let timeout = null
    if (currentMCP) {
      if (forceRefresh) {
        // 刷新时使用 SSE 读取超时时间（毫秒），默认 300 秒 = 300000 毫秒
        timeout = (currentMCP.sseReadTimeout || 300) * 1000
      } else {
        // 普通加载时使用超时时间（毫秒），默认 60 秒 = 60000 毫秒
        timeout = (currentMCP.timeout || 60) * 1000
      }
    }
    
    const result = await getRemoteMCPTools(currentMCPId.value, forceRefresh, timeout)
    toolsList.value = result.tools || []
    if (result.cached) {
      // 如果是缓存的数据，可以显示提示（可选）
      // ElMessage.info('已加载缓存的工具列表')
    }
  } catch (error) {
    ElMessage.error('加载工具列表失败: ' + (error.response?.data?.error || error.message))
    toolsList.value = []
  } finally {
    loadingTools.value = false
  }
}

// Remote MCP刷新：强制从远程MCP服务获取工具列表（会更新Redis缓存）
const refreshTools = () => {
  loadTools(true)
}

const loadToolsForRow = async (row, force = false) => {
  if (!row) return
  const identifier = row.serverId || row.name

  // 如果已加载且不是强制刷新，直接返回
  if (!force && row.tools && row.tools.length > 0) {
    return
  }

  // 计算超时时间：如果是刷新，使用 SSE 读取超时时间；否则使用普通超时时间
  let timeout = null
  if (row) {
    if (force) {
      // 刷新时使用 SSE 读取超时时间（毫秒），默认 300 秒 = 300000 毫秒
      timeout = (row.sseReadTimeout || 300) * 1000
    } else {
      // 普通加载时使用超时时间（毫秒），默认 60 秒 = 60000 毫秒
      timeout = (row.timeout || 60) * 1000
    }
  }

  // 如果已有工具数量但工具列表为空，说明可能正在加载，先尝试从缓存加载
  if (
    !force &&
    row.toolsCount !== undefined &&
    row.toolsCount > 0 &&
    (!row.tools || row.tools.length === 0)
  ) {
    try {
      const result = await getRemoteMCPTools(identifier, false, timeout)
      if (result && result.tools && result.tools.length > 0) {
        row.tools = result.tools
        row.toolsCount = result.tools.length
        return
      }
    } catch (error) {
      // 缓存加载失败，继续下面的流程
    }
  }

  const showSkeleton = !row.tools || row.tools.length === 0
  if (force) {
    row.refreshingTools = true
  } else if (showSkeleton) {
    row.loadingTools = true
  } else {
    row.refreshingTools = true
  }

  try {
    const result = await getRemoteMCPTools(identifier, force, timeout)
    row.tools = result.tools || []
    row.toolsCount = row.tools.length
  } catch (error) {
    ElMessage.error('加载工具列表失败: ' + (error.response?.data?.error || error.message))
    if (!row.tools || row.tools.length === 0) {
      row.tools = []
      row.toolsCount = 0
    }
  } finally {
    if (force || (!showSkeleton && row.refreshingTools)) {
      row.refreshingTools = false
    }
    if (showSkeleton) {
      row.loadingTools = false
    }
  }
}

// 从远程MCP服务强制刷新工具列表（会更新Redis缓存）
const refreshToolsForRow = (row) => {
  loadToolsForRow(row, true)
}

// 从本地Redis缓存刷新工具列表（不调用远程服务）
const refreshToolsFromCache = async (row) => {
  if (!row) return
  const identifier = row.serverId || row.name
  
  // 计算超时时间：从缓存加载使用普通超时时间
  const timeout = (row.timeout || 60) * 1000
  
  row.refreshingTools = true
  try {
    // 从缓存加载，不强制刷新（force=false）
    const result = await getRemoteMCPTools(identifier, false, timeout)
    if (result && result.tools && result.tools.length > 0) {
      row.tools = result.tools
      row.toolsCount = result.tools.length
    } else {
      row.tools = []
      row.toolsCount = 0
      ElMessage.info('缓存中暂无工具列表，请使用 Remote MCP刷新 从远程获取')
    }
  } catch (error) {
    ElMessage.error('从缓存加载工具列表失败: ' + (error.response?.data?.error || error.message))
    if (!row.tools || row.tools.length === 0) {
      row.tools = []
      row.toolsCount = 0
    }
  } finally {
    row.refreshingTools = false
  }
}

const handleExpandChange = (row) => {
  loadToolsForRow(row)
}

const addHeader = () => {
  form.headers.push({ name: '', value: '' })
}

const removeHeader = (index) => {
  form.headers.splice(index, 1)
}

const selectIcon = (icon) => {
  form.icon = icon
  showIconPicker.value = false
}

const trimBaseUrl = () => {
  if (form.baseUrl) {
    form.baseUrl = form.baseUrl.replace(/\s+/g, '').trim()
  }
}

onMounted(() => {
  loadRemoteMCPs()
})
</script>

<style scoped>
.tools-list {
  max-height: 500px;
  overflow-y: auto;
}

.tool-item {
  padding: 12px;
  margin-bottom: 8px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  background: #fafafa;
  transition: background-color 0.2s;
}

.tool-item:hover {
  background: #f0f0f0;
}

.tool-header {
  margin-bottom: 8px;
}

.tool-description {
  color: #606266;
  line-height: 1.5;
}

.empty-tools {
  padding: 40px 0;
  text-align: center;
}

.headers-container {
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  padding: 16px;
  background: #fdfdff;
}

.header-row {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
  align-items: flex-start;
}

.header-input.name {
  flex: 0 0 200px;
}

.header-input.value {
  flex: 1;
}

.header-remove-btn {
  margin-top: 4px;
}

.header-actions {
  margin-top: 4px;
}

.tools-expand {
  padding: 12px 16px;
  background: #fafbff;
  border-radius: 6px;
  border: 1px solid #e4e7ed;
}

.tools-expand-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.tools-expand-body {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.tools-expand-empty {
  color: #909399;
}

.tool-chip {
  flex: 0 0 calc(50% - 12px);
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  padding: 10px 12px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.tool-chip-title {
  font-weight: 600;
  margin-bottom: 6px;
}

.tool-chip-desc {
  font-size: 12px;
  color: #606266;
  line-height: 1.4;
}

.config-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  align-items: center;
  font-weight: 500;
}
</style>

