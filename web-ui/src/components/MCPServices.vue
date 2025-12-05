<template>
  <el-card class="config-card" shadow="hover">
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
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const emit = defineEmits(['config-updated'])

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

const handleTest = (service) => {
  ElMessage.info(`正在测试 ${service.name}...`)
  // TODO: 实现服务测试逻辑
}
</script>

<style scoped>
.config-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  align-items: center;
  font-weight: 500;
}
</style>
