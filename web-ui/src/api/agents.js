import axios from 'axios'

const API_BASE = '/api'

export async function getAgents() {
  const response = await axios.get(`${API_BASE}/config/agents`)
  return response.data
}

export async function createAgent(payload) {
  const response = await axios.post(`${API_BASE}/config/agents`, payload)
  return response.data
}

export async function updateAgent(id, payload) {
  const response = await axios.put(`${API_BASE}/config/agents/${encodeURIComponent(id)}`, payload)
  return response.data
}

export async function deleteAgent(id) {
  await axios.delete(`${API_BASE}/config/agents/${encodeURIComponent(id)}`)
}

