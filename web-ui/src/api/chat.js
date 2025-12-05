import axios from 'axios'

const API_BASE = '/api'

// 发送对话消息（支持取消）
export async function sendChat(sessionId, message, agentId, signal = null) {
  const payload = {
    sessionId,
    message
  }
  if (agentId) {
    payload.agentId = agentId
  }
  // 设置10分钟超时时间（因为 qwen3-max 等大模型可能需要更长时间）
  const config = {
    timeout: 600000, // 10分钟超时（600000毫秒）
  }
  if (signal) {
    config.signal = signal // 支持 AbortController
  }
  const response = await axios.post(`${API_BASE}/chat`, payload, config)
  return response.data
}

// 获取所有会话列表
export async function getSessions(agentId, limit = 50, skip = 0) {
  const response = await axios.get(`${API_BASE}/sessions`, {
    params: {
      ...(agentId ? { agentId } : {}),
      limit,
      skip
    },
    timeout: 60000 // 60秒超时
  })
  return response.data
}

// 获取会话详情
export async function getSession(sessionId) {
  const response = await axios.get(`${API_BASE}/sessions/${sessionId}`)
  return response.data
}

