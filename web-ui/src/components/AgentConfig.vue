<template>
  <el-card class="config-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>智能Agent配置</span>
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          添加智能体
        </el-button>
      </div>
    </template>

    <el-table :data="agents" v-loading="loadingAgents">
      <el-table-column prop="name" label="名称" width="200">
        <template #default="scope">
          <div class="agent-name">
            <strong>{{ scope.row.name }}</strong>
            <el-tag v-if="scope.row.isDefault" type="success" size="small" style="margin-left: 6px">
              默认
            </el-tag>
          </div>
          <div class="agent-desc">{{ scope.row.description || '未填写描述' }}</div>
        </template>
      </el-table-column>

      <el-table-column label="关联 MCP" min-width="200">
        <template #default="scope">
          {{ getMCPName(scope.row.mcpServerId) }}
        </template>
      </el-table-column>

      <el-table-column label="状态" width="120">
        <template #default="scope">
          <el-switch
            v-model="scope.row.enabled"
            @change="toggleAgent(scope.row)"
            active-text="启用"
            inactive-text="停用"
          />
        </template>
      </el-table-column>

      <el-table-column prop="updatedAt" label="更新时间" width="180">
        <template #default="scope">
          {{ formatTime(scope.row.updatedAt) }}
        </template>
      </el-table-column>

      <el-table-column label="操作" width="200">
        <template #default="scope">
          <el-button size="small" @click="handleEdit(scope.row)">编辑</el-button>
          <el-button
            size="small"
            type="danger"
            @click="handleDelete(scope.row)"
          >删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-empty
      v-if="!loadingAgents && agents.length === 0"
      description="请先添加一个智能体并绑定 MCP 服务"
      style="margin-top: 20px"
    />

    <el-dialog
      v-model="showDialog"
      :title="editingAgent ? '编辑智能体' : '添加智能体'"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form :model="form" label-width="120px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" maxlength="50" show-word-limit />
        </el-form-item>

        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="关联 MCP 服务" required>
          <el-select v-model="form.mcpServerId" placeholder="选择一个 MCP 服务">
            <el-option
              v-for="mcp in remoteMCPs"
              :key="mcp.serverId || mcp.name"
              :label="`${mcp.name} (${mcp.serverId})`"
              :value="mcp.serverId || mcp.name"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="系统提示词（编排）">
          <el-collapse>
            <el-collapse-item name="prompt">
              <template #title>
                <span style="font-weight: 500;">自定义提示词（可选）</span>
                <el-text type="info" size="small" style="margin-left: 10px;">
                  {{ form.systemPrompt ? '已配置' : '使用默认' }}
                </el-text>
              </template>
              <div style="margin-top: 10px;">
                <el-alert
                  title="提示词编排说明"
                  type="info"
                  :closable="false"
                  style="margin-bottom: 10px;"
                >
                  <template #default>
                    <div style="font-size: 12px; line-height: 1.6;">
                      <p>• 使用 <code>{tools}</code> 占位符会自动替换为工具列表</p>
                      <p>• 如果不使用占位符，工具列表会自动追加到提示词末尾</p>
                      <p>• 留空则使用系统默认提示词模板</p>
                    </div>
                  </template>
                </el-alert>
                <el-input
                  v-model="form.systemPrompt"
                  type="textarea"
                  :rows="12"
                  placeholder="请输入自定义系统提示词...&#10;&#10;示例：&#10;你是智能体 &quot;{agentName}&quot;，{description}&#10;&#10;可用工具：&#10;{tools}&#10;&#10;【严格要求】&#10;1. 必须只使用工具返回的真实数据&#10;2. 对于 K8s 资源查询，必须先调用工具&#10;3. 如果工具返回空结果，明确告诉用户&quot;没有找到&quot;"
                  show-word-limit
                  :maxlength="5000"
                />
                <div style="margin-top: 10px; display: flex; gap: 10px;">
                  <el-button size="small" @click="loadDefaultPrompt">加载默认模板</el-button>
                  <el-button size="small" @click="form.systemPrompt = ''">清空</el-button>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>
        </el-form-item>

        <el-form-item label="启用状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>

        <el-form-item label="设置为默认">
          <el-switch v-model="form.isDefault" active-text="是" inactive-text="否" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveAgent">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getAgents, createAgent, updateAgent, deleteAgent } from '../api/agents'
import { getRemoteMCPs } from '../api/config'

const agents = ref([])
const remoteMCPs = ref([])
const loadingAgents = ref(false)
const showDialog = ref(false)
const saving = ref(false)
const editingAgent = ref(null)

const form = reactive({
  name: '',
  description: '',
  mcpServerId: '',
  systemPrompt: '',
  enabled: true,
  isDefault: false
})

const loadAgents = async () => {
  loadingAgents.value = true
  try {
    agents.value = await getAgents()
  } catch (error) {
    console.error('加载智能体失败', error)
    ElMessage.error('加载智能体列表失败')
  } finally {
    loadingAgents.value = false
  }
}

const loadRemote = async () => {
  try {
    const data = await getRemoteMCPs()
    remoteMCPs.value = Array.isArray(data) ? data.filter(item => item.enabled) : []
  } catch (error) {
    console.error('加载 MCP 服务失败', error)
    remoteMCPs.value = []
  }
}

const resetForm = () => {
  Object.assign(form, {
    name: '',
    description: '',
    mcpServerId: '',
    systemPrompt: '',
    enabled: true,
    isDefault: agents.value.length === 0
  })
}

const handleAdd = () => {
  editingAgent.value = null
  resetForm()
  showDialog.value = true
}

const handleEdit = (agent) => {
  editingAgent.value = agent
  Object.assign(form, {
    name: agent.name,
    description: agent.description || '',
    mcpServerId: agent.mcpServerId,
    systemPrompt: agent.systemPrompt || '',
    enabled: agent.enabled,
    isDefault: agent.isDefault
  })
  showDialog.value = true
}

const loadDefaultPrompt = () => {
  form.systemPrompt = `你是智能体 "{agentName}"，{description}

可用工具：
{tools}

工作流程：
1. 理解用户的问题
2. **对于任何 Kubernetes 资源查询（如 Pod、Deployment、Namespace 等），必须调用相应的工具获取实时数据**
3. **绝对不要基于训练数据或历史知识直接回答 K8s 相关问题，必须先调用工具**
4. 基于工具返回的结果，用自然语言回答用户的问题

【重要规则】：
- **必须只使用工具返回的真实数据，绝对不要使用训练数据或假设**
- **对于 K8s 资源查询，必须先调用工具，不能直接回答**
- 如果工具返回空结果（如 [] 或 {"pods": []}），必须明确告诉用户"没有找到"或"不存在"
- 不要编造任何工具返回数据中不存在的信息
- 如果工具返回的数据中没有某个字段，就说"数据中未包含此信息"

响应格式：
- 如果需要调用工具，请以 JSON 格式回复：
  {
    "action": "call_tool",
    "tool": "工具名称",
    "arguments": {"参数名": "参数值"},
    "thought": "你的思考过程",
    "reply": "我将调用工具来查询信息，请稍候..."
  }
- 如果可以直接回答，请以 JSON 格式回复：
  {
    "action": "respond",
    "reply": "你的回答"
  }

重要：无论何时，都必须包含 "reply" 字段，用自然语言向用户说明当前的操作或回答。请用中文回答用户的问题。`
  ElMessage.success('已加载默认提示词模板')
}

const saveAgent = async () => {
  if (!form.name || !form.mcpServerId) {
    ElMessage.warning('请填写名称并选择 MCP 服务')
    return
  }

  // 检查名称唯一性
  const trimmedName = form.name.trim()
  const existingAgent = agents.value.find(
    agent => agent.name === trimmedName && 
    (!editingAgent.value || agent.id !== editingAgent.value.id)
  )
  if (existingAgent) {
    ElMessage.warning(`智能体名称 "${trimmedName}" 已存在，请使用其他名称`)
    return
  }

  saving.value = true
  try {
    const payload = {
      name: trimmedName,
      description: form.description?.trim() || '',
      mcpServerId: form.mcpServerId,
      systemPrompt: form.systemPrompt?.trim() || '',
      enabled: form.enabled,
      isDefault: form.isDefault
    }
    if (editingAgent.value) {
      await updateAgent(editingAgent.value.id, payload)
      ElMessage.success('智能体更新成功')
    } else {
      await createAgent(payload)
      ElMessage.success('智能体创建成功')
    }
    showDialog.value = false
    loadAgents()
  } catch (error) {
    console.error('保存智能体失败', error)
    const errorMsg = error.response?.data?.error || error.message || '保存失败'
    ElMessage.error(errorMsg)
  } finally {
    saving.value = false
  }
}

const toggleAgent = async (agent) => {
  try {
    await updateAgent(agent.id, {
      name: agent.name,
      description: agent.description || '',
      mcpServerId: agent.mcpServerId,
      systemPrompt: agent.systemPrompt || '',
      enabled: agent.enabled,
      isDefault: agent.isDefault
    })
    ElMessage.success(`${agent.enabled ? '已启用' : '已停用'} ${agent.name}`)
  } catch (error) {
    agent.enabled = !agent.enabled
    console.error('更新状态失败', error)
    ElMessage.error('更新智能体状态失败')
  }
}

const handleDelete = async (agent) => {
  try {
    await ElMessageBox.confirm(
      `确定删除智能体 "${agent.name}" 吗？删除后无法恢复。`,
      '确认删除',
      {
        type: 'warning',
        confirmButtonText: '确定删除',
        cancelButtonText: '取消'
      }
    )
    
    try {
      await deleteAgent(agent.id)
      ElMessage.success('删除成功')
      // 确保刷新列表
      await loadAgents()
    } catch (error) {
      console.error('删除智能体失败', error)
      const errorMsg = error.response?.data?.error || error.message || '删除失败'
      ElMessage.error(`删除失败: ${errorMsg}`)
    }
  } catch (error) {
    // 用户取消删除，不做任何操作
    if (error !== 'cancel') {
      console.error('删除确认失败', error)
    }
  }
}

const getMCPName = (serverId) => {
  const item = remoteMCPs.value.find(m => (m.serverId || m.name) === serverId)
  return item ? `${item.name} (${item.serverId})` : serverId || '未知'
}

const formatTime = (timestamp) => {
  if (!timestamp) return '-'
  const date = new Date(timestamp * 1000)
  return date.toLocaleString()
}

onMounted(() => {
  loadAgents()
  loadRemote()
})
</script>

<style scoped>
.config-card {
  min-height: 400px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.agent-name {
  display: flex;
  align-items: center;
}

.agent-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>


