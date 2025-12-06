# 验证 MCP 配置更新是否成功保存到数据库
# 用法: .\scripts\verify-mcp-update.ps1 -ServerID "ansible-mcp-server"

param(
    [Parameter(Mandatory=$false)]
    [string]$ServerID = "ansible-mcp-server"
)

Write-Host "`n=== 验证 MCP 配置更新 ===" -ForegroundColor Cyan
Write-Host ""

# 1. 从 API 获取配置
Write-Host "1. 从 API 获取配置..." -ForegroundColor Yellow
try {
    $apiConfig = Invoke-RestMethod -Uri "http://localhost:9090/api/config/remote-mcp/$ServerID" -Method GET -TimeoutSec 5
    Write-Host "   ✅ API 配置获取成功" -ForegroundColor Green
    Write-Host "   - 名称: $($apiConfig.name)" -ForegroundColor Gray
    Write-Host "   - BaseURL: $($apiConfig.baseUrl)" -ForegroundColor Gray
    Write-Host "   - Timeout: $($apiConfig.timeout)" -ForegroundColor Gray
    Write-Host "   - SSE Read Timeout: $($apiConfig.sseReadTimeout)" -ForegroundColor Gray
    Write-Host "   - Headers 数量: $($apiConfig.headers.Count)" -ForegroundColor Gray
    if ($apiConfig.headers) {
        Write-Host "   - Headers 内容:" -ForegroundColor Gray
        foreach ($key in $apiConfig.headers.PSObject.Properties.Name) {
            Write-Host "     $key = $($apiConfig.headers.$key)" -ForegroundColor DarkGray
        }
    }
} catch {
    Write-Host "   ❌ API 配置获取失败: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""

# 2. 从 MySQL 查询配置
Write-Host "2. 从 MySQL 查询配置..." -ForegroundColor Yellow

# MySQL 连接信息（从环境变量或默认值）
$mysqlHost = if ($env:MYSQL_HOST) { $env:MYSQL_HOST } else { "11.0.1.110" }
$mysqlPort = if ($env:MYSQL_PORT) { $env:MYSQL_PORT } else { "30306" }
$mysqlUser = if ($env:MYSQL_USER) { $env:MYSQL_USER } else { "root" }
$mysqlPassword = if ($env:MYSQL_PASSWORD) { $env:MYSQL_PASSWORD } else { "canxixi" }
$mysqlDB = if ($env:MYSQL_DB) { $env:MYSQL_DB } else { "mcp" }

$mysqlPath = "mysql"
if (Get-Command mysql -ErrorAction SilentlyContinue) {
    $mysqlPath = "mysql"
} else {
    Write-Host "   ⚠️  MySQL 客户端未找到，请手动执行以下 SQL 查询:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "   SELECT server_id, name, base_url, timeout, sse_read_timeout, headers, last_update" -ForegroundColor Cyan
    Write-Host "   FROM remote_mcp_configs" -ForegroundColor Cyan
    Write-Host "   WHERE server_id = '$ServerID';" -ForegroundColor Cyan
    Write-Host ""
    exit 0
}

# 构建 MySQL 命令
$mysqlCmd = "SELECT server_id, name, base_url, timeout, sse_read_timeout, headers, last_update FROM remote_mcp_configs WHERE server_id = '$ServerID';"

try {
    $env:MYSQL_PWD = $mysqlPassword
    $dbResult = & $mysqlPath -h $mysqlHost -P $mysqlPort -u $mysqlUser $mysqlDB -e $mysqlCmd 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "   ✅ MySQL 查询成功" -ForegroundColor Green
        Write-Host ""
        Write-Host "   MySQL 查询结果:" -ForegroundColor Gray
        Write-Host $dbResult -ForegroundColor DarkGray
    } else {
        Write-Host "   ❌ MySQL 查询失败: $dbResult" -ForegroundColor Red
    }
} catch {
    Write-Host "   ❌ MySQL 查询异常: $_" -ForegroundColor Red
} finally {
    Remove-Item Env:\MYSQL_PWD -ErrorAction SilentlyContinue
}

Write-Host ""

# 3. 对比结果
Write-Host "3. 验证结果:" -ForegroundColor Yellow
Write-Host "   - 如果 API 和 MySQL 中的配置一致，说明更新成功 ✅" -ForegroundColor Green
Write-Host "   - 如果配置不一致，请检查日志或重新保存配置" -ForegroundColor Yellow
Write-Host ""

