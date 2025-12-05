@echo off
echo Starting Kubernetes MCP Web Service...
echo.

REM 检查环境变量
if "%K8S_API_SERVER%"=="" (
    echo Warning: K8S_API_SERVER not set, will use kubeconfig or in-cluster config
) else (
    echo Using K8S_API_SERVER: %K8S_API_SERVER%
)

if "%MCP_WEB_ADDR%"=="" (
    set MCP_WEB_ADDR=:8080
    echo Using default address: %MCP_WEB_ADDR%
) else (
    echo Using address: %MCP_WEB_ADDR%
)

echo.
echo Starting server...
echo Press Ctrl+C to stop
echo.

bin\k8s-mcp-web.exe

