@echo off
chcp 65001 >nul
echo ========================================
echo    重启所有服务
echo ========================================
echo.

echo 步骤 1: 停止现有服务...
echo.

REM 停止端口 9090 上的进程
for /f "tokens=5" %%a in ('netstat -aon ^| findstr :9090 ^| findstr LISTENING') do (
    echo 停止端口 9090 上的进程 (PID: %%a)
    taskkill /F /PID %%a >nul 2>&1
)

REM 停止端口 8080 上的进程
for /f "tokens=5" %%a in ('netstat -aon ^| findstr :8080 ^| findstr LISTENING') do (
    echo 停止端口 8080 上的进程 (PID: %%a)
    taskkill /F /PID %%a >nul 2>&1
)

echo.
echo 等待 3 秒...
timeout /t 3 /nobreak >nul
echo.

echo 步骤 2: 检查服务文件...
echo.

cd /d "%~dp0\.."

REM 检查 k8s-mcp-web.exe
if not exist "bin\k8s-mcp-web.exe" (
    echo 警告: 未找到 bin\k8s-mcp-web.exe
    echo 正在构建...
    go build -o bin\k8s-mcp-web.exe .\cmd\web\main.go
    if errorlevel 1 (
        echo 错误: 构建失败
        pause
        exit /b 1
    )
    echo 构建成功
) else (
    echo   [√] 找到 bin\k8s-mcp-web.exe
)

REM 检查 ansible-mcp-main 目录
if not exist "ansible-mcp-main" (
    echo 警告: 未找到 ansible-mcp-main 目录，将跳过 Ansible MCP Server 的启动
    set SKIP_ANSIBLE=1
) else (
    echo   [√] 找到 ansible-mcp-main 目录
    set SKIP_ANSIBLE=0
)

echo.
echo 步骤 3: 启动服务...
echo.

REM 启动 k8s-mcp-web 服务
echo 启动 k8s-mcp-web 服务...
if exist "scripts\start-with-console.bat" (
    start "k8s-mcp-web" "scripts\start-with-console.bat"
    echo   [√] k8s-mcp-web 服务已启动
    echo   访问地址: http://localhost:9090
) else (
    echo   [×] 未找到启动脚本
)

REM 启动 ansible-mcp-server（如果存在）
if "%SKIP_ANSIBLE%"=="0" (
    echo.
    echo 启动 ansible-mcp-server 服务...
    python --version >nul 2>&1
    if not errorlevel 1 (
        if exist "scripts\start-ansible-mcp.ps1" (
            start "ansible-mcp-server" powershell -NoExit -ExecutionPolicy Bypass -File "scripts\start-ansible-mcp.ps1"
            echo   [√] ansible-mcp-server 服务已启动
            echo   访问地址: http://localhost:8080/docs
        ) else (
            echo   [×] 未找到启动脚本
        )
    ) else (
        echo   警告: 未找到 Python，跳过 ansible-mcp-server 的启动
    )
)

echo.
echo 步骤 4: 等待服务启动...
timeout /t 5 /nobreak >nul
echo.

echo 步骤 5: 检查服务状态...
echo.

REM 检查端口 9090
netstat -an 2>nul | findstr :9090 | findstr LISTENING >nul
if not errorlevel 1 (
    echo   [√] k8s-mcp-web 服务运行正常 (端口 9090)
) else (
    echo   [×] k8s-mcp-web 服务未启动 (端口 9090)
)

REM 检查端口 8080
netstat -an 2>nul | findstr :8080 | findstr LISTENING >nul
if not errorlevel 1 (
    echo   [√] ansible-mcp-server 服务运行正常 (端口 8080)
) else (
    echo   [-] ansible-mcp-server 服务未启动 (端口 8080)
)

echo.
echo ========================================
echo    重启完成
echo ========================================
echo.
echo 服务已在独立窗口中运行
echo.
echo 停止服务: 关闭对应的窗口
echo.

pause

