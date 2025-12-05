import axios from 'axios'

const API_BASE = '/api'

// 登录
export async function login(username, password) {
  const response = await axios.post(`${API_BASE}/auth/login`, {
    username,
    password
  })
  return response.data
}

// 登出
export async function logout() {
  await axios.post(`${API_BASE}/auth/logout`)
}

// 检查登录状态
export async function checkAuth() {
  try {
    const response = await axios.get(`${API_BASE}/auth/check`)
    return response.data
  } catch (error) {
    return { authenticated: false }
  }
}

