# 重启所有服务脚本
# 停止并重新启动 k8s-mcp-web 和 ansible-mcp-server 服务

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   重启所有服务" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 定义要检查的端口
$ports = @(8080, 9090)

Write-Host "步骤 1: 停止现有服务..." -ForegroundColor Yellow
Write-Host ""

$stoppedProcesses = @()

foreach ($port in $ports) {
    $connections = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
    if ($connections) {
        $processes = $connections | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($processId in $processes) {
            try {
                $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
                if ($process) {
                    Write-Host "  停止端口 $port 上的进程: $($process.ProcessName) (PID: $processId)" -ForegroundColor White
                    Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
                    $stoppedProcesses += $processId
                }
            } catch {
                Write-Host "  警告: 无法停止进程 $processId : $_" -ForegroundColor Yellow
            }
        }
    } else {
        Write-Host "  端口 $port 未被占用" -ForegroundColor Gray
    }
}

if ($stoppedProcesses.Count -gt 0) {
    Write-Host ""
    Write-Host "已停止 $($stoppedProcesses.Count) 个进程，等待 3 秒..." -ForegroundColor Green
    Start-Sleep -Seconds 3
} else {
    Write-Host "没有发现运行中的服务" -ForegroundColor Gray
}

Write-Host ""
Write-Host "步骤 2: 检查服务文件..." -ForegroundColor Yellow
Write-Host ""

# 设置工作目录
$originalLocation = Get-Location
Set-Location $PSScriptRoot\..

# 检查 k8s-mcp-web.exe
$webExe = "bin\k8s-mcp-web.exe"
if (-not (Test-Path $webExe)) {
    Write-Host "警告: 未找到 $webExe" -ForegroundColor Yellow
    Write-Host "正在构建..." -ForegroundColor Yellow
    go build -o $webExe .\cmd\web\main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "错误: 构建失败" -ForegroundColor Red
        Set-Location $originalLocation
        exit 1
    }
    Write-Host "构建成功" -ForegroundColor Green
} else {
    Write-Host "  ✓ 找到 $webExe" -ForegroundColor Green
}

# 检查 ansible-mcp-main 目录
$ansibleDir = "ansible-mcp-main"
$skipAnsible = $false
if (-not (Test-Path $ansibleDir)) {
    Write-Host "警告: 未找到 $ansibleDir 目录，将跳过 Ansible MCP Server 的启动" -ForegroundColor Yellow
    $skipAnsible = $true
} else {
    Write-Host "  ✓ 找到 $ansibleDir 目录" -ForegroundColor Green
}

Write-Host ""
Write-Host "步骤 3: 启动服务..." -ForegroundColor Yellow
Write-Host ""

# 启动 k8s-mcp-web 服务（使用现有脚本）
Write-Host "启动 k8s-mcp-web 服务..." -ForegroundColor Cyan

$webScript = Join-Path $PSScriptRoot "start-with-console.bat"
if (Test-Path $webScript) {
    Write-Host "  使用 start-with-console.bat 启动服务..." -ForegroundColor White
    $webProcess = Start-Process cmd -ArgumentList @("/c", "start", "`"k8s-mcp-web`"", $webScript) -PassThru
    Write-Host "  ✓ k8s-mcp-web 服务已启动" -ForegroundColor Green
    Write-Host "  访问地址: http://localhost:9090" -ForegroundColor Cyan
} else {
    Write-Host "  警告: 未找到启动脚本，尝试直接启动..." -ForegroundColor Yellow
    # 设置环境变量
    $env:REDIS_ADDR = "11.0.1.110:31202"
    $env:REDIS_PASSWORD = "difyai123456"
    $env:REDIS_DB = "1"
    $env:MYSQL_HOST = "11.0.1.110"
    $env:MYSQL_PORT = "30306"
    $env:MYSQL_USER = "root"
    $env:MYSQL_PASSWORD = "canxixi"
    $env:MYSQL_DB = "mcp"
    $env:MONGODB_HOST = "11.0.1.110"
    $env:MONGODB_PORT = "30792"
    $env:MONGODB_DB = "mcp"
    $env:MCP_WEB_ADDR = ":9090"
    
    $webProcess = Start-Process cmd -ArgumentList @("/c", "start", "`"k8s-mcp-web`"", "powershell", "-NoExit", "-Command", "cd '$((Get-Location).Path)'; `$env:REDIS_ADDR='$env:REDIS_ADDR'; `$env:REDIS_PASSWORD='$env:REDIS_PASSWORD'; `$env:REDIS_DB='$env:REDIS_DB'; `$env:MYSQL_HOST='$env:MYSQL_HOST'; `$env:MYSQL_PORT='$env:MYSQL_PORT'; `$env:MYSQL_USER='$env:MYSQL_USER'; `$env:MYSQL_PASSWORD='$env:MYSQL_PASSWORD'; `$env:MYSQL_DB='$env:MYSQL_DB'; `$env:MONGODB_HOST='$env:MONGODB_HOST'; `$env:MONGODB_PORT='$env:MONGODB_PORT'; `$env:MONGODB_DB='$env:MONGODB_DB'; `$env:MCP_WEB_ADDR='$env:MCP_WEB_ADDR'; .\bin\k8s-mcp-web.exe") -PassThru
    Write-Host "  ✓ k8s-mcp-web 服务已启动" -ForegroundColor Green
}

# 启动 ansible-mcp-server（如果存在）
if (-not $skipAnsible) {
    Write-Host ""
    Write-Host "启动 ansible-mcp-server 服务..." -ForegroundColor Cyan
    
    # 检查 Python
    $pythonVersion = python --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  在新窗口中启动 ansible-mcp-server..." -ForegroundColor White
        
        $ansibleScript = Join-Path $PSScriptRoot "start-ansible-mcp.ps1"
        if (Test-Path $ansibleScript) {
            $ansibleProcess = Start-Process powershell -ArgumentList @("-NoExit", "-ExecutionPolicy", "Bypass", "-File", $ansibleScript) -PassThru
            Write-Host "  ✓ ansible-mcp-server 服务已启动" -ForegroundColor Green
            Write-Host "  访问地址: http://localhost:8080/docs" -ForegroundColor Cyan
        } else {
            Write-Host "  警告: 未找到启动脚本" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  警告: 未找到 Python，跳过 ansible-mcp-server 的启动" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "步骤 4: 等待服务启动..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

Write-Host ""
Write-Host "步骤 5: 检查服务状态..." -ForegroundColor Yellow
Write-Host ""

# 检查端口 9090
$port9090 = Get-NetTCPConnection -LocalPort 9090 -ErrorAction SilentlyContinue
if ($port9090) {
    Write-Host "  ✓ k8s-mcp-web 服务运行正常 (端口 9090)" -ForegroundColor Green
} else {
    Write-Host "  ✗ k8s-mcp-web 服务未启动 (端口 9090)" -ForegroundColor Red
}

# 检查端口 8080
$port8080 = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($port8080) {
    Write-Host "  ✓ ansible-mcp-server 服务运行正常 (端口 8080)" -ForegroundColor Green
} else {
    Write-Host "  - ansible-mcp-server 服务未启动 (端口 8080)" -ForegroundColor Gray
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   重启完成" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "服务已在独立窗口中运行" -ForegroundColor Green
Write-Host ""
Write-Host "停止服务:" -ForegroundColor Yellow
Write-Host "  关闭对应的窗口，或使用以下命令查看进程:" -ForegroundColor White
Write-Host "  Get-NetTCPConnection -LocalPort 9090 | Select-Object OwningProcess" -ForegroundColor Gray
Write-Host "  Get-NetTCPConnection -LocalPort 8080 | Select-Object OwningProcess" -ForegroundColor Gray
Write-Host ""

Set-Location $originalLocation
