<template>
  <el-dialog
    v-model="visible"
    title="登录"
    width="400px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
  >
    <el-form :model="loginForm" label-width="80px">
      <el-form-item label="用户名" required>
        <el-input
          v-model="loginForm.username"
          placeholder="请输入用户名"
          @keyup.enter="handleLogin"
        />
      </el-form-item>
      <el-form-item label="密码" required>
        <el-input
          v-model="loginForm.password"
          type="password"
          show-password
          placeholder="请输入密码"
          @keyup.enter="handleLogin"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="handleLogin" type="primary" :loading="loading">
        登录
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { login } from '../api/auth'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'login-success'])

const visible = ref(props.modelValue)
const loading = ref(false)
const loginForm = reactive({
  username: '',
  password: ''
})

watch(() => props.modelValue, (val) => {
  visible.value = val
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

const handleLogin = async () => {
  if (!loginForm.username || !loginForm.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }

  loading.value = true
  try {
    const result = await login(loginForm.username, loginForm.password)
    if (result.success) {
      ElMessage.success('登录成功')
      visible.value = false
      emit('login-success')
    } else {
      ElMessage.error(result.message || '登录失败')
    }
  } catch (error) {
    ElMessage.error('登录失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}
</script>

