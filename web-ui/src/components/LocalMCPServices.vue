<template>
  <el-card class="config-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span>本地 MCP 服务</span>
      </div>
    </template>

    <el-empty description="本地 MCP 服务功能开发中..." />
  </el-card>
</template>

<script setup>
const emit = defineEmits(['config-updated'])
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

