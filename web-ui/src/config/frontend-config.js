/**
 * 前端配置管理
 * 支持从环境变量、本地存储或默认值获取配置
 */

// 默认配置
const DEFAULT_CONFIG = {
  // 工具列表加载超时配置（毫秒）
  toolsRefreshTimeout: 300000,  // 5分钟（强制刷新时）
  toolsLoadTimeout: 10000,      // 10秒（普通加载，使用缓存）
}

// 从环境变量获取配置
function getEnvConfig() {
  return {
    toolsRefreshTimeout: import.meta.env.VITE_TOOLS_REFRESH_TIMEOUT 
      ? parseInt(import.meta.env.VITE_TOOLS_REFRESH_TIMEOUT, 10) 
      : null,
    toolsLoadTimeout: import.meta.env.VITE_TOOLS_LOAD_TIMEOUT 
      ? parseInt(import.meta.env.VITE_TOOLS_LOAD_TIMEOUT, 10) 
      : null,
  }
}

// 从本地存储获取配置
function getLocalStorageConfig() {
  try {
    const stored = localStorage.getItem('frontendConfig')
    if (stored) {
      return JSON.parse(stored)
    }
  } catch (e) {
    console.warn('Failed to load config from localStorage:', e)
  }
  return {}
}

// 保存配置到本地存储
export function saveConfig(config) {
  try {
    const current = getLocalStorageConfig()
    const merged = { ...current, ...config }
    localStorage.setItem('frontendConfig', JSON.stringify(merged))
    return true
  } catch (e) {
    console.error('Failed to save config to localStorage:', e)
    return false
  }
}

// 获取配置值（优先级：环境变量 > 本地存储 > 默认值）
export function getConfig(key) {
  const envConfig = getEnvConfig()
  const localConfig = getLocalStorageConfig()
  
  // 如果环境变量中有配置，优先使用
  if (envConfig[key] !== null && envConfig[key] !== undefined) {
    return envConfig[key]
  }
  
  // 如果本地存储中有配置，使用本地存储
  if (localConfig[key] !== null && localConfig[key] !== undefined) {
    return localConfig[key]
  }
  
  // 使用默认值
  return DEFAULT_CONFIG[key]
}

// 获取所有配置
export function getAllConfig() {
  const envConfig = getEnvConfig()
  const localConfig = getLocalStorageConfig()
  
  return {
    toolsRefreshTimeout: envConfig.toolsRefreshTimeout ?? localConfig.toolsRefreshTimeout ?? DEFAULT_CONFIG.toolsRefreshTimeout,
    toolsLoadTimeout: envConfig.toolsLoadTimeout ?? localConfig.toolsLoadTimeout ?? DEFAULT_CONFIG.toolsLoadTimeout,
  }
}

// 重置配置为默认值
export function resetConfig() {
  try {
    localStorage.removeItem('frontendConfig')
    return true
  } catch (e) {
    console.error('Failed to reset config:', e)
    return false
  }
}

// 导出默认配置供参考
export { DEFAULT_CONFIG }

