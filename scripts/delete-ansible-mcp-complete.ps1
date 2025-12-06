# 完全删除 ansible-mcp-server 的脚本
# 包括：1. 从 API 删除 2. 从 MySQL 删除 3. 从文件备份删除

param(
    [string]$ServerID = "ansible-mcp-server"
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  完全删除 MCP 服务: $ServerID" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. 从 API 删除
Write-Host "1. 从 API 删除..." -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "http://localhost:9090/api/config/remote-mcp/$ServerID" -Method DELETE -TimeoutSec 5 | Out-Null
    Write-Host "   ✅ API 删除成功" -ForegroundColor Green
} catch {
    if ($_.Exception.Response.StatusCode -eq 404) {
        Write-Host "   ⚠️  服务不存在（可能已删除）" -ForegroundColor Yellow
    } else {
        Write-Host "   ❌ API 删除失败: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""

# 2. 从 MySQL 删除（如果 MySQL 客户端可用）
Write-Host "2. 从 MySQL 删除..." -ForegroundColor Yellow
$mysqlHost = "11.0.1.110"
$mysqlPort = "30306"
$mysqlUser = "root"
$mysqlPassword = "canxixi"
$mysqlDB = "mcp"

if (Get-Command mysql -ErrorAction SilentlyContinue) {
    $env:MYSQL_PWD = $mysqlPassword
    $query = "DELETE FROM remote_mcp_configs WHERE server_id = '$ServerID';"
    $result = & mysql -h $mysqlHost -P $mysqlPort -u $mysqlUser $mysqlDB -e $query 2>&1
    Remove-Item Env:\MYSQL_PWD -ErrorAction SilentlyContinue
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "   ✅ MySQL 删除成功" -ForegroundColor Green
    } else {
        Write-Host "   ⚠️  MySQL 删除可能失败: $result" -ForegroundColor Yellow
    }
} else {
    Write-Host "   ⚠️  MySQL 客户端未找到，请手动执行以下 SQL:" -ForegroundColor Yellow
    Write-Host "   DELETE FROM remote_mcp_configs WHERE server_id = '$ServerID';" -ForegroundColor Cyan
}

Write-Host ""

# 3. 从文件备份删除
Write-Host "3. 从文件备份删除..." -ForegroundColor Yellow
$configFile = ".config\web-config.json"
if (Test-Path $configFile) {
    try {
        $content = Get-Content $configFile -Raw | ConvertFrom-Json
        if ($content.remoteMcps -and $content.remoteMcps.PSObject.Properties.Name -contains $ServerID) {
            $content.remoteMcps.PSObject.Properties.Remove($ServerID)
            $content | ConvertTo-Json -Depth 10 | Set-Content $configFile -Encoding UTF8
            Write-Host "   ✅ 文件备份删除成功" -ForegroundColor Green
        } else {
            Write-Host "   ℹ️  文件备份中不存在该服务" -ForegroundColor Gray
        }
    } catch {
        Write-Host "   ❌ 文件备份删除失败: $_" -ForegroundColor Red
    }
} else {
    Write-Host "   ℹ️  配置文件不存在" -ForegroundColor Gray
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  删除完成！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "建议：重启服务以确保完全清除" -ForegroundColor Yellow

