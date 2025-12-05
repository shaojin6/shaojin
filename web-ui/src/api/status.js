import axios from 'axios'

const API_BASE = '/api'

// 获取系统状态
export async function getStatus() {
  const response = await axios.get(`${API_BASE}/status`)
  return response.data
}

