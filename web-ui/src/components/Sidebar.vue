<template>
  <el-aside width="250px" class="sidebar">
    <el-menu
      :default-active="activeMenu"
      class="sidebar-menu"
      @select="handleMenuSelect"
    >
      <!-- 智能问答 -->
      <el-menu-item index="chat">
        <el-icon><ChatDotRound /></el-icon>
        <span>智能问答</span>
      </el-menu-item>
      
      <!-- 配置管理 -->
      <el-sub-menu index="config">
        <template #title>
          <el-icon><Setting /></el-icon>
          <span>配置管理</span>
        </template>
        <el-menu-item index="agents">
          <el-icon><User /></el-icon>
          <span>智能Agent配置</span>
        </el-menu-item>
        <el-menu-item index="mcp-config">
          <el-icon><Connection /></el-icon>
          <span>MCP 配置</span>
        </el-menu-item>
        <el-menu-item index="llm">
          <el-icon><ChatDotRound /></el-icon>
          <span>LLM 配置</span>
        </el-menu-item>
        <el-menu-item index="k8s">
          <el-icon><Operation /></el-icon>
          <span>K8s 配置</span>
        </el-menu-item>
      </el-sub-menu>
      
      <!-- 系统状态 -->
      <el-menu-item index="status">
        <el-icon><DataAnalysis /></el-icon>
        <span>系统状态</span>
      </el-menu-item>
      
      <!-- 系统日志 -->
      <el-menu-item index="logs">
        <el-icon><Document /></el-icon>
        <span>系统日志</span>
      </el-menu-item>
    </el-menu>
  </el-aside>
</template>

<script setup>
import { ref } from 'vue'
import { Connection, ChatDotRound, Setting, DataAnalysis, Operation, User, Document } from '@element-plus/icons-vue'

const emit = defineEmits(['menu-select'])

const activeMenu = ref('chat')

const handleMenuSelect = (index) => {
  activeMenu.value = index
  emit('menu-select', index)
}
</script>

<style scoped>
.sidebar {
  background: #fff;
  border-right: none;
  height: calc(100vh - 60px);
  overflow-y: auto;
}

.sidebar-menu {
  border-right: none;
  height: 100%;
}
</style>

