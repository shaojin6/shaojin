@echo off
echo ========================================
echo Starting Kubernetes MCP Web Service
echo ========================================
echo.

REM 检查是否存在 .env 文件
if exist .env (
    echo Using configuration from .env file
    echo.
) else (
    echo WARNING: .env file not found!
    echo Please create .env file from config.env.example
    echo   Run: scripts\create-config.bat
    echo.
    echo Using environment variables (if set)...
    echo.
)

REM 环境变量（可选，.env 文件优先级更高）
REM 如果 .env 文件不存在，可以使用这些环境变量
if not exist .env (
    set K8S_API_SERVER=http://11.0.1.110:6443
    set K8S_API_TOKEN=eyJhbGciOiJSUzI1NiIsImtpZCI6Ikkyb3pWeTR1MzJpZWoySUNLbzd2SXJTUUFJVVZCRno3ODJ2U2pZVXF4R1EifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkaWZ5Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6Ims4cy1tY3Atc2VydmVyLXNhLXRva2VuLTJzc2d4Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6Ims4cy1tY3Atc2VydmVyLXNhIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiZjNkODEyNDgtYTgwNS00NTVlLWFkMjItNDZjMzZkNjhhZGFiIiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRpZnk6azhzLW1jcC1zZXJ2ZXItc2EifQ.pwNOBeeMqmugI3eCxC1kqBWLQmN6uR3tzIi79_a9wvYsLjaNBgk7tc1uNS4swLW842zEtgHXhtvI4cOgvxX_83_VmPlGfylOR7S5qAFRIaZVZsPKLuKDJEaEDpMhAkgxByBnVBU4C2cVEDCoXf16_8WuDdfnJeDYkh8RuWPgd4JzRO5g9Esh2R9Py9zvA65wGEv_cEVB5EaHRZbIvqaVpivOekXNhf0THPg5EdFBTmcnbLQNuRxRiQWwZ65FOSN-46D0LDOm3pGGZKnWSxeSITzIMdyyPDFjxJmLYdB_XKg-foIqVLimdy-VuMg2SrvLdIa3Mlpr9NaZWuzm9K063A
    set K8S_API_INSECURE=true
    set MCP_WEB_ADDR=:8080
)

echo Building...
go build -o bin\k8s-mcp-web.exe .\cmd\web\main.go
if errorlevel 1 (
    echo Build failed!
    pause
    exit /b 1
)

echo.
echo Build successful! Starting server...
echo Press Ctrl+C to stop
echo.
echo Server will be available at: http://localhost:8080
echo.

bin\k8s-mcp-web.exe

