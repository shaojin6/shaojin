@echo off
echo Starting Kubernetes MCP Web Service...
echo.

REM 在当前会话中设置环境变量（临时，仅本次会话有效）
if "%K8S_API_SERVER%"=="" (
    echo Setting environment variables for this session...
    set K8S_API_SERVER=http://11.0.1.110:6443
    echo K8S_API_SERVER=%K8S_API_SERVER%
)

if "%K8S_API_TOKEN%"=="" (
    echo Warning: K8S_API_TOKEN not set. Please set it:
    echo   set K8S_API_TOKEN=your-token-here
    echo.
    echo Or set it permanently:
    echo   setx K8S_API_TOKEN your-token-here
    echo.
)

if "%K8S_API_INSECURE%"=="" (
    set K8S_API_INSECURE=true
    echo K8S_API_INSECURE=%K8S_API_INSECURE%
)

if "%MCP_WEB_ADDR%"=="" (
    set MCP_WEB_ADDR=:8080
)

echo.
echo Configuration:
echo   K8S_API_SERVER=%K8S_API_SERVER%
echo   K8S_API_TOKEN=%K8S_API_TOKEN% (hidden)
echo   K8S_API_INSECURE=%K8S_API_INSECURE%
echo   MCP_WEB_ADDR=%MCP_WEB_ADDR%
echo.
echo Starting server...
echo Press Ctrl+C to stop
echo.

bin\k8s-mcp-web.exe

