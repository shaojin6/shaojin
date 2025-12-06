import axios from 'axios'

const API_BASE = '/api'

// 获取配置
export async function getConfig() {
  const response = await axios.get(`${API_BASE}/config`)
  return response.data
}

// K8s 配置管理（支持多个）
export async function getK8sConfigs() {
  const response = await axios.get(`${API_BASE}/config/k8s`)
  return response.data
}

export async function getK8sConfig(id) {
  const response = await axios.get(`${API_BASE}/config/k8s/${id}`)
  return response.data
}

export async function saveK8sConfig(config) {
  if (config.id) {
    await axios.put(`${API_BASE}/config/k8s/${config.id}`, config)
  } else {
    await axios.post(`${API_BASE}/config/k8s`, config)
  }
}

export async function deleteK8sConfig(id) {
  await axios.delete(`${API_BASE}/config/k8s/${id}`)
}

// 别名
export const saveK8s = saveK8sConfig

// LLM 配置管理（支持多个）
export async function getLLMConfigs() {
  const response = await axios.get(`${API_BASE}/config/llm`)
  return response.data
}

export async function getLLMConfig(id) {
  const response = await axios.get(`${API_BASE}/config/llm/${id}`)
  return response.data
}

export async function saveLLMConfig(config) {
  if (config.id) {
    await axios.put(`${API_BASE}/config/llm/${config.id}`, config)
  } else {
    await axios.post(`${API_BASE}/config/llm`, config)
  }
}

export async function deleteLLMConfig(id) {
  await axios.delete(`${API_BASE}/config/llm/${id}`)
}

// 测试 K8s 连接
export async function testK8s() {
  const response = await axios.post(`${API_BASE}/test-k8s`)
  return response.data
}

// 测试 LLM 连接（支持指定 ID）
export async function testLLM(id) {
  const response = await axios.post(`${API_BASE}/test-llm`, id ? { id } : {})
  return response.data
}

export async function getCurrentLLM() {
  const response = await axios.get(`${API_BASE}/current-llm`)
  return response.data
}

// 远程 MCP 服务相关 API
export async function getRemoteMCPs() {
  const response = await axios.get(`${API_BASE}/config/remote-mcp`)
  return response.data
}

export async function addRemoteMCP(config) {
  await axios.post(`${API_BASE}/config/remote-mcp`, config, {
    timeout: 10000 // 10秒超时，避免工具刷新过程阻塞太久
  })
}

export async function updateRemoteMCP(identifier, config) {
  await axios.put(`${API_BASE}/config/remote-mcp/${encodeURIComponent(identifier)}`, config, {
    timeout: 10000 // 10秒超时，避免工具刷新过程阻塞太久
  })
}

export async function deleteRemoteMCP(identifier) {
  await axios.delete(`${API_BASE}/config/remote-mcp/${encodeURIComponent(identifier)}`)
}

export async function testRemoteMCP(identifier) {
  const response = await axios.post(`${API_BASE}/config/remote-mcp/${encodeURIComponent(identifier)}/test`)
  return response.data
}

export async function getRemoteMCPTools(identifier, refresh = false, timeout = null) {
  const url = `${API_BASE}/config/remote-mcp/${encodeURIComponent(identifier)}/tools`
  
  // 如果指定了超时时间，使用指定的；否则使用默认值
  // 刷新时使用更长的超时时间（因为可能需要从远程获取）
  const defaultTimeout = refresh ? 300000 : 10000 // 刷新：5分钟，普通加载：10秒
  
  const response = await axios.get(url, {
    params: refresh ? { refresh: 'true' } : {},
    timeout: timeout || defaultTimeout
  })
  return response.data
}

// 测试端点路径（不保存配置）
export async function testEndpoint(baseUrl, toolsEndpoint, headers) {
  const response = await axios.post(`${API_BASE}/config/remote-mcp/test-endpoint`, {
    baseUrl,
    toolsEndpoint,
    headers
  })
  return response.data
}
