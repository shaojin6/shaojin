<template>
  <el-card class="config-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>K8s 配置</span>
        <el-button type="primary" size="small" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          添加 K8s 集群
        </el-button>
      </div>
    </template>

    <el-table :data="k8sConfigs" style="width: 100%">
      <el-table-column label="名称" width="200">
        <template #default="scope">
          <span style="font-weight: 500;">{{ scope.row.name || '未命名集群' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="mode" label="连接方式" width="120">
        <template #default="scope">
          <el-tag>{{ scope.row.mode === 'kubeconfig' ? 'Kubeconfig' : '手动配置' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="server" label="API Server" />
      <el-table-column prop="namespace" label="命名空间" width="120" />
      <el-table-column prop="enabled" label="状态" width="80">
        <template #default="scope">
          <el-switch
            v-model="scope.row.enabled"
            @change="toggleK8s(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="scope">
          <el-button size="small" @click="handleEdit(scope.row)">编辑</el-button>
          <el-button size="small" @click="testK8s(scope.row)">测试</el-button>
          <el-button size="small" type="danger" @click="handleDelete(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 添加/编辑对话框 -->
    <el-dialog
      v-model="showDialog"
      :title="editingItem ? '编辑 K8s 配置' : '添加 K8s 配置'"
      width="700px"
      :close-on-click-modal="false"
      @close="resetForm"
    >
      <el-form :model="form" label-width="120px" label-position="left">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如: 生产环境集群" />
        </el-form-item>

        <el-form-item label="连接方式" required>
          <el-radio-group v-model="form.mode">
            <el-radio label="kubeconfig">Kubeconfig 文件</el-radio>
            <el-radio label="manual">手动配置</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- Kubeconfig 模式 -->
        <template v-if="form.mode === 'kubeconfig'">
          <el-form-item label="Kubeconfig" required>
            <el-upload
              :file-list="kubeconfigFileList"
              :auto-upload="false"
              :on-change="handleKubeconfigChange"
              :limit="1"
            >
              <el-button type="primary">选择文件</el-button>
            </el-upload>
          </el-form-item>
        </template>

        <!-- 手动配置模式 -->
        <template v-else>
          <el-form-item label="API Server" required>
            <el-input v-model="form.server" placeholder="https://11.0.1.110:6443" />
          </el-form-item>

          <el-form-item label="默认命名空间">
            <el-input 
              v-model="form.namespace" 
              placeholder="留空表示不限制命名空间（查询整个集群）"
              autocomplete="off"
            />
            <el-text type="info" size="small" style="display: block; margin-top: 5px;">
              可选字段。如果留空，工具将查询所有命名空间；如果填写，将作为默认命名空间使用
            </el-text>
          </el-form-item>

          <el-form-item label="认证方式">
            <el-radio-group v-model="authType">
              <el-radio label="token">Token</el-radio>
              <el-radio label="username">用户名/密码</el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item v-if="authType === 'token'" label="Token" required>
            <el-input 
              v-model="form.token" 
              type="password" 
              show-password 
              placeholder="Bearer Token"
              autocomplete="new-password"
            />
          </el-form-item>

          <template v-else>
            <el-form-item label="用户名" required>
              <el-input 
                v-model="form.username" 
                placeholder="用户名"
                autocomplete="username"
              />
            </el-form-item>
            <el-form-item label="密码" required>
              <el-input 
                v-model="form.password" 
                type="password" 
                show-password 
                placeholder="密码"
                autocomplete="new-password"
              />
            </el-form-item>
          </template>

          <el-form-item>
            <el-checkbox v-model="form.insecure" :disabled="hasCACert">跳过 TLS 验证</el-checkbox>
            <el-text type="info" size="small" style="display: block; margin-top: 5px;">
              如果配置了 CA 证书，将使用证书验证，此选项将被禁用
            </el-text>
          </el-form-item>

          <el-form-item label="CA 证书（可选）">
            <el-radio-group v-model="caCertMode" style="margin-bottom: 10px;">
              <el-radio label="none">不使用 CA 证书</el-radio>
              <el-radio label="file">上传证书文件</el-radio>
              <el-radio label="text">输入证书内容</el-radio>
            </el-radio-group>

            <!-- 文件上传方式 -->
            <el-upload
              v-if="caCertMode === 'file'"
              :file-list="caCertFileList"
              :auto-upload="false"
              :on-change="handleCACertChange"
              :limit="1"
              accept=".crt,.pem,.cert"
            >
              <el-button type="primary" size="small">选择 CA 证书文件</el-button>
              <template #tip>
                <div class="el-upload__tip">支持 .crt, .pem, .cert 格式的证书文件</div>
              </template>
            </el-upload>

            <!-- 文本输入方式 -->
            <el-input
              v-if="caCertMode === 'text'"
              v-model="form.caData"
              type="textarea"
              :rows="6"
              placeholder="请输入 PEM 格式的 CA 证书内容（-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----）"
              style="margin-top: 10px;"
            />
            <el-text v-if="caCertMode === 'text'" type="info" size="small" style="display: block; margin-top: 5px;">
              证书内容将自动进行 base64 编码存储
            </el-text>
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveK8s" :loading="loading">
          {{ editingItem ? '保存' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getK8sConfigs, saveK8sConfig, deleteK8sConfig, testK8s as testK8sAPI } from '../api/config'

const emit = defineEmits(['config-updated'])

const k8sConfigs = ref([])
const showDialog = ref(false)
const editingItem = ref(null)
const loading = ref(false)
const authType = ref('token')
const kubeconfigFileList = ref([])
const caCertMode = ref('none')
const caCertFileList = ref([])

const form = reactive({
  name: '',
  mode: 'manual',
  server: '',
  namespace: 'default',
  token: '',
  username: '',
  password: '',
  insecure: false,
  content: '',
  caFile: '',
  caData: '',
  enabled: true
})

// 计算是否有 CA 证书
const hasCACert = computed(() => {
  return caCertMode.value !== 'none' && (form.caData || form.caFile)
})

const loadK8sConfigs = async () => {
  try {
    k8sConfigs.value = await getK8sConfigs()
  } catch (error) {
    console.error('Failed to load K8s configs:', error)
  }
}

const handleAdd = () => {
  editingItem.value = null
  resetForm()
  showDialog.value = true
}

const handleEdit = (k8s) => {
  editingItem.value = k8s
  Object.assign(form, {
    name: k8s.name || '',
    mode: k8s.mode || 'manual',
    server: k8s.server || '',
    namespace: k8s.namespace || '', // 如果为空，显示为空字符串，不显示 'default'
    token: '', // 不显示完整 token
    username: k8s.username || '',
    password: '', // 不显示密码
    insecure: k8s.insecure || false,
    content: k8s.content || '',
    caFile: k8s.caFile || '',
    caData: k8s.caData || '',
    enabled: k8s.enabled !== false
  })
  
  // 判断 CA 证书模式
  if (k8s.caFile) {
    caCertMode.value = 'file'
  } else if (k8s.caData) {
    caCertMode.value = 'text'
    // 如果 caData 是 base64 编码的，需要解码显示
    try {
      const decoded = atob(k8s.caData)
      // 检查是否是有效的 PEM 格式
      if (decoded.includes('BEGIN CERTIFICATE')) {
        form.caData = decoded
      } else {
        // 如果解码后不是 PEM 格式，可能是原始 PEM，直接使用
        form.caData = k8s.caData
      }
    } catch (e) {
      // 如果不是 base64，直接使用
      form.caData = k8s.caData
    }
  } else {
    caCertMode.value = 'none'
  }
  
  if (k8s.token) {
    authType.value = 'token'
  } else if (k8s.username) {
    authType.value = 'username'
  }
  caCertFileList.value = []
  showDialog.value = true
}

const resetForm = () => {
  editingItem.value = null
  authType.value = 'token'
  kubeconfigFileList.value = []
  caCertMode.value = 'none'
  caCertFileList.value = []
  Object.assign(form, {
    name: '',
    mode: 'manual',
    server: '',
    namespace: 'default',
    token: '',
    username: '',
    password: '',
    insecure: false,
    content: '',
    caFile: '',
    caData: '',
    enabled: true
  })
}

const handleKubeconfigChange = (file) => {
  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target.result
    form.content = btoa(content) // base64 编码
  }
  reader.readAsText(file.raw)
}

const handleCACertChange = (file) => {
  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target.result
    // 将证书内容进行 base64 编码存储
    form.caData = btoa(content)
    caCertFileList.value = [file]
  }
  reader.onloadend = () => {
    if (form.caData) {
      // 如果设置了 CA 证书，自动禁用 insecure
      form.insecure = false
    }
  }
  reader.readAsText(file.raw)
}

const saveK8s = async () => {
  if (!form.name) {
    ElMessage.warning('请填写名称')
    return
  }

  if (form.mode === 'kubeconfig' && !form.content) {
    ElMessage.warning('请上传 Kubeconfig 文件')
    return
  }

  if (form.mode === 'manual') {
    if (!form.server) {
      ElMessage.warning('请填写 API Server 地址')
      return
    }
    if (authType.value === 'token' && !form.token) {
      ElMessage.warning('请填写 Token')
      return
    }
    if (authType.value === 'username' && (!form.username || !form.password)) {
      ElMessage.warning('请填写用户名和密码')
      return
    }
  }

  loading.value = true
  try {
    const config = {
      ...form,
      id: editingItem.value?.id
    }
    // 清理不需要的字段
    if (form.mode === 'kubeconfig') {
      delete config.server
      delete config.token
      delete config.username
      delete config.password
    } else {
      delete config.content
      if (authType.value === 'token') {
        delete config.username
        delete config.password
      } else {
        delete config.token
      }
      
      // 处理 CA 证书
      if (caCertMode.value === 'none') {
        delete config.caFile
        delete config.caData
      } else if (caCertMode.value === 'file') {
        // 文件上传方式：caData 已经包含 base64 编码的内容
        delete config.caFile // 前端不支持文件路径
        // caData 已经在上传时进行了 base64 编码
      } else if (caCertMode.value === 'text') {
        // 文本输入方式：将 PEM 内容进行 base64 编码
        if (config.caData && config.caData.includes('BEGIN CERTIFICATE')) {
          // 如果是 PEM 格式，进行 base64 编码
          config.caData = btoa(config.caData)
        }
        // 如果已经是 base64，不需要再次编码
        delete config.caFile
      }
      
      // 如果配置了 CA 证书，确保 insecure 为 false
      if (config.caData || config.caFile) {
        config.insecure = false
      }
    }
    await saveK8sConfig(config)
    const clusterName = form.name || '未命名集群'
    ElMessage.success(editingItem.value ? `${clusterName} 配置更新成功` : `${clusterName} 配置添加成功`)
    showDialog.value = false
    resetForm()
    await loadK8sConfigs()
    emit('config-updated')
  } catch (error) {
    ElMessage.error('保存失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}

const handleDelete = async (k8s) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除 K8s 配置 "${k8s.name || '未命名集群'}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await deleteK8sConfig(k8s.id)
    const clusterName = k8s.name || '未命名集群'
    ElMessage.success(`${clusterName} 删除成功`)
    await loadK8sConfigs()
    emit('config-updated')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.response?.data?.error || error.message))
    }
  }
}

const testK8s = async (k8s) => {
  try {
    const result = await testK8sAPI(k8s.id)
    if (result.status === 'ok') {
      ElMessage.success('连接测试成功: ' + result.message)
    } else {
      ElMessage.error('连接失败: ' + result.message)
    }
  } catch (error) {
    ElMessage.error('测试失败: ' + (error.response?.data?.message || error.message))
  }
}

const toggleK8s = async (k8s) => {
  try {
    const config = { ...k8s, enabled: k8s.enabled }
    await saveK8sConfig(config)
    const clusterName = k8s.name || '未命名集群'
    ElMessage.success(k8s.enabled ? `${clusterName} 已启用` : `${clusterName} 已禁用`)
    emit('config-updated')
  } catch (error) {
    ElMessage.error('操作失败: ' + (error.response?.data?.error || error.message))
    k8s.enabled = !k8s.enabled
  }
}

onMounted(() => {
  loadK8sConfigs()
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

