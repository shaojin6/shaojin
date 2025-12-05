@echo off
REM 创建 .env 配置文件

if exist .env (
    echo .env file already exists!
    echo Please edit .env manually or delete it first.
    pause
    exit /b 1
)

echo Creating .env file from template...
copy config.env.example .env >nul

echo.
echo .env file created successfully!
echo Please edit .env file and set your Kubernetes API configuration:
echo   - K8S_API_SERVER: Your Kubernetes API server address
echo   - K8S_API_TOKEN: Your authentication token
echo   - K8S_API_INSECURE: Set to true if using self-signed certificates
echo.
pause

