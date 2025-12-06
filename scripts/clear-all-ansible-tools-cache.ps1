# Script to clear all ansible-mcp-server tools cache (Redis, MySQL, and database)

$mysqlHost = $env:MYSQL_HOST
if (-not $mysqlHost) { $mysqlHost = "11.0.1.110" }
$mysqlPort = $env:MYSQL_PORT
if (-not $mysqlPort) { $mysqlPort = "30306" }
$mysqlUser = $env:MYSQL_USER
if (-not $mysqlUser) { $mysqlUser = "root" }
$mysqlPassword = $env:MYSQL_PASSWORD
if (-not $mysqlPassword) { $mysqlPassword = "canxixi" }
$mysqlDB = $env:MYSQL_DB
if (-not $mysqlDB) { $mysqlDB = "mcp" }

$redisHost = $env:REDIS_ADDR
if (-not $redisHost) { $redisHost = "11.0.1.110:31202" }
$redisPassword = $env:REDIS_PASSWORD
if (-not $redisPassword) { $redisPassword = "difyai123456" }
$redisDB = $env:REDIS_DB
if (-not $redisDB) { $redisDB = "1" }

Write-Host "Clearing all ansible-mcp-server tools cache..." -ForegroundColor Cyan
Write-Host ""

Write-Host "⚠️  WARNING: This will clear:" -ForegroundColor Red
Write-Host "  1. remote_mcp_configs.tools (database)" -ForegroundColor Yellow
Write-Host "  2. mcp_tools_cache (MySQL cache table)" -ForegroundColor Yellow
Write-Host "  3. Redis cache (mcp:tools:ansible-mcp-server)" -ForegroundColor Yellow
Write-Host ""
$confirm = Read-Host "Do you want to continue? (yes/no)"
if ($confirm -ne "yes") {
    Write-Host "Cancelled." -ForegroundColor Yellow
    exit 0
}

Write-Host ""
Write-Host "Step 1: Clearing remote_mcp_configs.tools..." -ForegroundColor Green
$mysqlClientPath = (where.exe mysql 2>&1 | Select-Object -First 1)
if ($mysqlClientPath -like "*not found*" -or -not $mysqlClientPath) {
    Write-Host "MySQL client not found. Please run this SQL manually:" -ForegroundColor Yellow
    Write-Host "UPDATE remote_mcp_configs SET tools = NULL, tools_last_update = NULL WHERE server_id = 'ansible-mcp-server';" -ForegroundColor White
} else {
    $query1 = "UPDATE remote_mcp_configs SET tools = NULL, tools_last_update = NULL WHERE server_id = 'ansible-mcp-server';"
    try {
        $result1 = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query1" 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✅ Cleared remote_mcp_configs.tools" -ForegroundColor Green
        } else {
            Write-Host "❌ Error: $result1" -ForegroundColor Red
        }
    } catch {
        Write-Host "❌ Error: $_" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Step 2: Clearing mcp_tools_cache..." -ForegroundColor Green
if ($mysqlClientPath -like "*not found*" -or -not $mysqlClientPath) {
    Write-Host "MySQL client not found. Please run this SQL manually:" -ForegroundColor Yellow
    Write-Host "DELETE FROM mcp_tools_cache WHERE identifier = 'ansible-mcp-server';" -ForegroundColor White
} else {
    $query2 = "DELETE FROM mcp_tools_cache WHERE identifier = 'ansible-mcp-server';"
    try {
        $result2 = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query2" 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✅ Cleared mcp_tools_cache" -ForegroundColor Green
        } else {
            Write-Host "❌ Error: $result2" -ForegroundColor Red
        }
    } catch {
        Write-Host "❌ Error: $_" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Step 3: Clearing Redis cache..." -ForegroundColor Green
Write-Host "Please use Redis CLI or your Redis client to delete the key:" -ForegroundColor Yellow
Write-Host "  DEL mcp:tools:ansible-mcp-server" -ForegroundColor White
Write-Host ""
Write-Host "Or use Redis command:" -ForegroundColor Yellow
Write-Host "  redis-cli -h 11.0.1.110 -p 31202 -a difyai123456 -n 1 DEL mcp:tools:ansible-mcp-server" -ForegroundColor White

Write-Host ""
Write-Host "✅ Cache clearing instructions provided!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. Restart the service" -ForegroundColor White
Write-Host "  2. Go to Web UI -> 配置管理 -> MCP 配置" -ForegroundColor White
Write-Host "  3. Find 'ansible-mcp-server'" -ForegroundColor White
Write-Host "  4. Click '刷新远程工具' button" -ForegroundColor White
Write-Host "  5. System will fetch correct 8 Ansible tools from remote server" -ForegroundColor White

