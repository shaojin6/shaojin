<template>
  <el-card class="config-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>LLM 配置</span>
        <el-button type="primary" size="small" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          添加 LLM
        </el-button>
      </div>
    </template>

    <el-table :data="llmConfigs" style="width: 100%">
      <el-table-column label="名称" width="200">
        <template #default="scope">
          <div style="display: flex; align-items: center; gap: 8px;">
            <el-tag v-if="scope.row.isDefault" type="success" size="small">默认</el-tag>
            <span style="font-weight: 500;">{{ scope.row.name || scope.row.provider }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="provider" label="提供商" width="120">
        <template #default="scope">
          <el-tag>{{ getProviderName(scope.row.provider) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="model" label="模型" width="150" />
      <el-table-column prop="baseUrl" label="API 地址" />
      <el-table-column prop="enabled" label="状态" width="80">
        <template #default="scope">
          <el-switch
            v-model="scope.row.enabled"
            @change="toggleLLM(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="280">
        <template #default="scope">
          <el-button size="small" @click="handleEdit(scope.row)">编辑</el-button>
          <el-button size="small" @click="testLLM(scope.row)">测试</el-button>
          <el-button size="small" @click="setDefault(scope.row)" v-if="!scope.row.isDefault">设为默认</el-button>
          <el-button size="small" type="danger" @click="handleDelete(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑对话框 -->
    <el-dialog
      v-model="showDialog"
      :title="editingItem ? '编辑 LLM 配置' : '添加 LLM 配置'"
      width="600px"
      :close-on-click-modal="false"
      @close="resetForm"
    >
      <el-form :model="form" label-width="120px" label-position="left">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如: 阿里云百炼" />
        </el-form-item>

        <el-form-item label="提供商" required>
          <el-select v-model="form.provider" placeholder="选择 LLM 提供商">
            <el-option label="百炼平台（推荐）" value="bailian" />
            <el-option label="DashScope（旧版）" value="dashscope" />
            <el-option label="OpenAI" value="openai" />
            <el-option label="Ollama" value="ollama" />
          </el-select>
        </el-form-item>

        <el-form-item label="API 地址" required>
          <el-input 
            v-model="form.baseUrl" 
            placeholder="https://dashscope.aliyuncs.com/api/v1"
          />
        </el-form-item>

        <el-form-item label="模型" required>
          <el-select v-model="form.model" placeholder="选择模型">
            <el-option label="qwen3-max（最新最强）" value="qwen3-max" />
            <el-option label="qwen-plus（推荐）" value="qwen-plus" />
            <el-option label="qwen-turbo（快速）" value="qwen-turbo" />
            <el-option label="qwen-max（旧版最强）" value="qwen-max" />
            <el-option label="qwen-max-longcontext（长上下文）" value="qwen-max-longcontext" />
            <el-option label="gpt-4" value="gpt-4" />
            <el-option label="gpt-3.5-turbo" value="gpt-3.5-turbo" />
          </el-select>
        </el-form-item>

        <el-form-item label="API Key" required>
          <el-input 
            v-model="form.apiKey" 
            type="password" 
            show-password
            placeholder="输入你的 API Key"
          />
        </el-form-item>

        <el-form-item>
          <el-checkbox v-model="form.isDefault">设为默认 LLM</el-checkbox>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveLLM" :loading="loading">
          {{ editingItem ? '保存' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getLLMConfigs, saveLLMConfig, deleteLLMConfig, testLLM as testLLMAPI } from '../api/config'

const emit = defineEmits(['config-updated'])

const llmConfigs = ref([])
const showDialog = ref(false)
const editingItem = ref(null)
const loading = ref(false)

const form = reactive({
  name: '',
  provider: 'bailian',
  baseUrl: 'https://dashscope.aliyuncs.com/api/v1',
  model: 'qwen-plus',
  apiKey: '',
  enabled: true,
  isDefault: false
})

const loadLLMConfigs = async () => {
  try {
    llmConfigs.value = await getLLMConfigs()
    // 如果没有配置，创建一个默认的
    if (llmConfigs.value.length === 0) {
      // 可以在这里创建一个默认配置
    }
  } catch (error) {
    console.error('Failed to load LLM configs:', error)
  }
}

const handleAdd = () => {
  editingItem.value = null
  resetForm()
  showDialog.value = true
}

const handleEdit = (llm) => {
  editingItem.value = llm
  Object.assign(form, {
    name: llm.name || '',
    provider: llm.provider || 'bailian',
    baseUrl: llm.baseUrl || 'https://dashscope.aliyuncs.com/api/v1',
    model: llm.model || 'qwen-plus',
    apiKey: '', // 不显示完整 API Key
    enabled: llm.enabled !== false,
    isDefault: llm.isDefault || false
  })
  showDialog.value = true
}

const resetForm = () => {
  editingItem.value = null
  Object.assign(form, {
    name: '',
    provider: 'bailian',
    baseUrl: 'https://dashscope.aliyuncs.com/api/v1',
    model: 'qwen-plus',
    apiKey: '',
    enabled: true,
    isDefault: false
  })
}

const saveLLM = async () => {
  if (!form.name) {
    ElMessage.warning('请填写名称')
    return
  }
  // 新增时必须填写 API Key，编辑时如果为空则保留原有值
  if (!editingItem.value && !form.apiKey) {
    ElMessage.warning('请填写 API Key')
    return
  }

  loading.value = true
  try {
    const config = {
      ...form,
      id: editingItem.value?.id
    }
    // 编辑时如果 API Key 为空，不发送该字段，让后端保留原有值
    if (editingItem.value && !form.apiKey) {
      delete config.apiKey
    }
    await saveLLMConfig(config)
    const llmName = form.name || form.provider || '未命名 LLM'
    ElMessage.success(editingItem.value ? `${llmName} 配置更新成功` : `${llmName} 配置添加成功`)
    showDialog.value = false
    resetForm()
    await loadLLMConfigs()
    emit('config-updated')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}

const handleDelete = async (llm) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除 LLM 配置 "${llm.name || llm.provider}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await deleteLLMConfig(llm.id)
    const llmName = llm.name || llm.provider || '未命名 LLM'
    ElMessage.success(`${llmName} 删除成功`)
    await loadLLMConfigs()
    emit('config-updated')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.response?.data?.error || error.message))
    }
  }
}

const testLLM = async (llm) => {
  try {
    const result = await testLLMAPI(llm.id)
    if (result.status === 'ok') {
      ElMessage.success(result.message || '连接测试成功')
    } else {
      ElMessage.error(result.message || '连接测试失败')
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.error || error.message))
  }
}

const setDefault = async (llm) => {
  try {
    const config = { ...llm, isDefault: true }
    await saveLLMConfig(config)
    const llmName = llm.name || llm.provider || '未命名 LLM'
    ElMessage.success(`${llmName} 已设为默认 LLM`)
    await loadLLMConfigs()
    emit('config-updated')
  } catch (error) {
    ElMessage.error('操作失败: ' + (error.response?.data?.error || error.message))
  }
}

const toggleLLM = async (llm) => {
  try {
    const config = { ...llm, enabled: llm.enabled }
    await saveLLMConfig(config)
    const llmName = llm.name || llm.provider || '未命名 LLM'
    ElMessage.success(llm.enabled ? `${llmName} 已启用` : `${llmName} 已禁用`)
    emit('config-updated')
  } catch (error) {
    ElMessage.error('操作失败: ' + (error.response?.data?.error || error.message))
    llm.enabled = !llm.enabled
  }
}

const getProviderName = (provider) => {
  const names = {
    'bailian': '百炼平台',
    'dashscope': 'DashScope',
    'openai': 'OpenAI',
    'ollama': 'Ollama'
  }
  return names[provider] || provider
}

onMounted(() => {
  loadLLMConfigs()
})
</script>

<style scoped>
.config-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 500;
}
</style>

