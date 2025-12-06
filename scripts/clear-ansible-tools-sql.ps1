# Script to clear ansible-mcp-server tools via SQL (if MySQL client is available)

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

Write-Host "Clearing ansible-mcp-server tools from database..." -ForegroundColor Cyan
Write-Host ""

# Check if mysql client is available
$mysqlClientPath = (where.exe mysql 2>&1 | Select-Object -First 1)
if ($mysqlClientPath -like "*not found*" -or -not $mysqlClientPath) {
    Write-Host "MySQL client not found. Please run this SQL query manually:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "=" * 60
    Write-Host "SQL Query:"
    Write-Host "=" * 60
    Write-Host "UPDATE remote_mcp_configs" -ForegroundColor White
    Write-Host "SET tools = NULL, tools_last_update = NULL" -ForegroundColor White
    Write-Host "WHERE server_id = 'ansible-mcp-server';" -ForegroundColor White
    Write-Host ""
    Write-Host "Connection info:" -ForegroundColor Cyan
    Write-Host "  Host: $mysqlHost" -ForegroundColor White
    Write-Host "  Port: $mysqlPort" -ForegroundColor White
    Write-Host "  User: $mysqlUser" -ForegroundColor White
    Write-Host "  Database: $mysqlDB" -ForegroundColor White
    Write-Host ""
    Write-Host "After running the SQL:" -ForegroundColor Yellow
    Write-Host "  1. Go to Web UI -> Configuration -> MCP Config" -ForegroundColor White
    Write-Host "  2. Find ansible-mcp-server" -ForegroundColor White
    Write-Host "  3. Click 'Refresh Remote Tools'" -ForegroundColor White
    Write-Host "  4. System will fetch correct tools from remote MCP server" -ForegroundColor White
    exit 1
}

Write-Host "⚠️  WARNING: This will clear tools for ansible-mcp-server!" -ForegroundColor Red
$confirm = Read-Host "Do you want to continue? (yes/no)"
if ($confirm -ne "yes") {
    Write-Host "Cancelled." -ForegroundColor Yellow
    exit 0
}

Write-Host ""
Write-Host "Executing SQL update..." -ForegroundColor Green
$query = "UPDATE remote_mcp_configs SET tools = NULL, tools_last_update = NULL WHERE server_id = 'ansible-mcp-server';"
try {
    $result = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Successfully cleared ansible-mcp-server tools!" -ForegroundColor Green
        Write-Host ""
        Write-Host "Next steps:" -ForegroundColor Cyan
        Write-Host "  1. Go to Web UI -> Configuration -> MCP Config" -ForegroundColor White
        Write-Host "  2. Find ansible-mcp-server" -ForegroundColor White
        Write-Host "  3. Click 'Refresh Remote Tools' (刷新远程工具)" -ForegroundColor White
        Write-Host "  4. System will fetch correct 8 Ansible tools from remote server" -ForegroundColor White
        Write-Host ""
        Write-Host "The incorrect Kubernetes tools have been cleared." -ForegroundColor Green
    } else {
        Write-Host "❌ Error executing SQL: $result" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "❌ Error: $_" -ForegroundColor Red
    exit 1
}

