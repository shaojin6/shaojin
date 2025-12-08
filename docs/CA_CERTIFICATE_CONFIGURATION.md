# CA 证书配置指南

## 概述

系统支持为 Kubernetes 集群配置 CA 证书，用于生产环境的安全 TLS 连接验证。相比"跳过 TLS 验证"，使用 CA 证书是更安全的方式。

## 功能特性

### 支持的配置方式

1. **文件上传方式**：上传 `.crt`、`.pem` 或 `.cert` 格式的 CA 证书文件
2. **文本输入方式**：直接输入 PEM 格式的 CA 证书内容

### 证书处理逻辑

- **存储格式**：证书内容以 base64 编码存储在数据库中（`caData` 字段）
- **自动编码**：前端会自动将 PEM 格式的证书内容进行 base64 编码
- **自动解码**：后端在使用证书时会自动解码 base64 内容

### 优先级规则

CA 证书验证的优先级顺序：

1. **CAFile**（文件路径）- 如果配置了文件路径，优先使用
2. **CAData**（证书内容）- 如果配置了证书内容，使用证书内容
3. **Insecure**（跳过验证）- 只有在没有配置 CA 证书时才生效

**重要**：如果配置了 CA 证书（CAFile 或 CAData），系统会自动禁用 `Insecure` 模式，确保使用证书验证。

## 使用方法

### 方式一：文件上传

1. 进入 **K8s 配置** 页面
2. 点击 **编辑** 或 **添加 K8s 集群**
3. 在 **CA 证书（可选）** 部分，选择 **上传证书文件**
4. 点击 **选择 CA 证书文件**，选择您的证书文件（`.crt`、`.pem` 或 `.cert`）
5. 系统会自动将证书内容进行 base64 编码并存储
6. 保存配置

### 方式二：文本输入

1. 进入 **K8s 配置** 页面
2. 点击 **编辑** 或 **添加 K8s 集群**
3. 在 **CA 证书（可选）** 部分，选择 **输入证书内容**
4. 在文本框中粘贴 PEM 格式的证书内容，例如：
   ```
   -----BEGIN CERTIFICATE-----
   MIIDXTCCAkWgAwIBAgIJAKL7...
   ...
   -----END CERTIFICATE-----
   ```
5. 系统会自动将证书内容进行 base64 编码并存储
6. 保存配置

## 证书格式要求

### PEM 格式

证书必须是标准的 PEM 格式，包含以下内容：

```
-----BEGIN CERTIFICATE-----
[Base64 编码的证书内容]
-----END CERTIFICATE-----
```

### 支持的证书类型

- **CA 根证书**：用于验证服务器证书的根 CA 证书
- **中间证书**：证书链中的中间 CA 证书
- **服务器证书**：Kubernetes API Server 的证书（通常不需要，除非是自签名）

## 配置示例

### 示例 1：使用文件上传

```json
{
  "id": "prod-cluster-1",
  "name": "生产环境集群",
  "mode": "manual",
  "server": "https://k8s-api.example.com:6443",
  "token": "eyJhbGci...",
  "caData": "LS0tLS1CRUdJTi...",  // base64 编码的证书
  "insecure": false
}
```

### 示例 2：使用文本输入

用户在前端输入：
```
-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKL7...
-----END CERTIFICATE-----
```

系统自动转换为 base64 并存储：
```json
{
  "caData": "LS0tLS1CRUdJTi..."
}
```

## 后端处理逻辑

### 证书解码

后端在创建 K8s 客户端时会自动处理证书：

```go
// 如果 caData 是 base64 编码的，自动解码
caDataBytes, err := base64.StdEncoding.DecodeString(k8sConfig.CAData)
if err != nil {
    // 如果解码失败，可能是原始 PEM 内容，直接使用
    config.CAData = []byte(k8sConfig.CAData)
} else {
    config.CAData = caDataBytes
}
```

### 验证优先级

```go
// 优先级：CAFile > CAData > Insecure
if k8sConfig.CAFile != "" {
    config.CAFile = k8sConfig.CAFile
} else if k8sConfig.CAData != "" {
    config.CAData = []byte(decodedCAData)
}

// 如果配置了 CA 证书，确保不使用 Insecure 模式
if k8sConfig.CAFile != "" || k8sConfig.CAData != "" {
    config.Insecure = false
}
```

## 安全建议

### 生产环境

✅ **推荐**：使用 CA 证书进行 TLS 验证
- 提供完整的证书链验证
- 防止中间人攻击
- 符合安全最佳实践

❌ **不推荐**：使用 `Insecure: true` 跳过验证
- 仅适用于开发/测试环境
- 存在安全风险

### 开发/测试环境

- 可以使用 `Insecure: true` 快速测试
- 但建议仍然配置 CA 证书以保持一致性

## 常见问题

### Q: 如何获取 Kubernetes 集群的 CA 证书？

**A:** 有几种方式：

1. **从 kubeconfig 文件获取**：
   ```bash
   kubectl config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' | base64 -d
   ```

2. **从集群证书文件获取**：
   ```bash
   cat /etc/kubernetes/pki/ca.crt
   ```

3. **从 API Server 获取**（需要先跳过验证）：
   ```bash
   openssl s_client -connect k8s-api.example.com:6443 -showcerts
   ```

### Q: 证书格式不正确怎么办？

**A:** 确保证书是 PEM 格式：
- 以 `-----BEGIN CERTIFICATE-----` 开头
- 以 `-----END CERTIFICATE-----` 结尾
- 中间是 base64 编码的内容

### Q: 配置了 CA 证书后仍然连接失败？

**A:** 检查以下几点：
1. 证书内容是否正确（包含完整的证书链）
2. API Server 地址是否正确
3. 证书是否与 API Server 匹配
4. 查看后端日志中的错误信息

### Q: 可以同时配置 CAFile 和 CAData 吗？

**A:** 可以，但系统会优先使用 `CAFile`。如果 `CAFile` 存在，`CAData` 将被忽略。

## 技术实现

### 数据库存储

CA 证书存储在 `k8s_configs` 表中：

```sql
CREATE TABLE k8s_configs (
    ...
    ca_file VARCHAR(500),    -- CA 证书文件路径
    ca_data TEXT,            -- CA 证书内容（base64 编码）
    ...
)
```

### 前端处理

- 文件上传：使用 `FileReader` API 读取文件内容，然后进行 base64 编码
- 文本输入：直接获取文本内容，保存时进行 base64 编码
- 编辑时：自动解码 base64 内容显示为原始 PEM 格式

### 后端处理

- 自动检测 base64 编码并解码
- 支持原始 PEM 格式（向后兼容）
- 优先级处理确保正确的验证方式

## 更新日志

- **v0.1.5+**: 添加完整的 CA 证书配置支持
  - 支持文件上传和文本输入两种方式
  - 自动 base64 编码/解码
  - 优先级处理逻辑
  - 前端 UI 优化

