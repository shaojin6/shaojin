#!/bin/bash
# 获取 k8s-mcp-server-sa ServiceAccount 的 Token
# 在 dify 命名空间下

echo "获取 k8s-mcp-server-sa ServiceAccount Token..."
echo "命名空间: dify"
echo ""

# 检查 kubectl 是否可用
if ! command -v kubectl &> /dev/null; then
    echo "错误: 未找到 kubectl 命令"
    echo "请确保已安装 kubectl 并配置好 kubeconfig"
    exit 1
fi

# 获取 ServiceAccount 的 Secret 名称
SECRET_NAME=$(kubectl get sa k8s-mcp-server-sa -n dify -o jsonpath='{.secrets[0].name}' 2>/dev/null)

if [ -z "$SECRET_NAME" ]; then
    echo "错误: 未找到 k8s-mcp-server-sa ServiceAccount 的 Secret"
    echo "请确保 ServiceAccount 已创建并且有关联的 Secret"
    exit 1
fi

echo "找到 Secret: $SECRET_NAME"
echo ""

# 获取 Token
TOKEN=$(kubectl get secret $SECRET_NAME -n dify -o jsonpath='{.data.token}' | base64 -d)

if [ -z "$TOKEN" ]; then
    echo "错误: 无法从 Secret 中提取 Token"
    exit 1
fi

echo "Token 获取成功!"
echo ""
echo "请将以下内容添加到 .env 文件中:"
echo "----------------------------------------"
echo "K8S_API_SERVER=http://11.0.1.110:6443"
echo "K8S_API_TOKEN=$TOKEN"
echo "K8S_API_INSECURE=true"
echo "----------------------------------------"
echo ""

