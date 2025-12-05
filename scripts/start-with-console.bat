@echo off
chcp 65001 >nul
title MCP Web Service - Console Logs
color 0A

echo ========================================
echo   MCP Web Service - Console Logs
echo ========================================
echo.
echo Logs will be displayed in this window
echo Logs are also saved to: service.log
echo Press Ctrl+C to stop the service
echo.
echo ========================================
echo.

cd /d "%~dp0\.."

REM 设置环境变量
set REDIS_ADDR=11.0.1.110:31202
set REDIS_PASSWORD=difyai123456
set REDIS_DB=1
set MYSQL_HOST=11.0.1.110
set MYSQL_PORT=30306
set MYSQL_USER=root
set MYSQL_PASSWORD=canxixi
set MYSQL_DB=mcp
set MONGODB_HOST=11.0.1.110
set MONGODB_PORT=30792
set MONGODB_DB=mcp
set MCP_WEB_ADDR=:9090

REM 检查可执行文件是否存在
if not exist "bin\k8s-mcp-web.exe" (
    echo ERROR: k8s-mcp-web.exe not found in bin directory
    echo Please build the project first: go build -o bin\k8s-mcp-web.exe cmd\web\main.go
    pause
    exit /b 1
)

echo Starting service...
echo.

REM 启动服务（会同时输出到控制台和日志文件）
bin\k8s-mcp-web.exe

pause


