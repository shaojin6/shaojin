# Script to check MCP configs in MySQL database

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

Write-Host "Checking MySQL database for remote_mcp_configs..."
Write-Host "Host: $mysqlHost"
Write-Host "Port: $mysqlPort"
Write-Host "Database: $mysqlDB"
Write-Host ""

# Check if mysql client is available
$mysqlClientPath = (where.exe mysql 2>&1 | Select-Object -First 1)
if ($mysqlClientPath -like "*not found*" -or -not $mysqlClientPath) {
    Write-Host "MySQL client not found. Please run the following SQL query manually:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "=" * 60
    Write-Host "SQL Query:"
    Write-Host "=" * 60
    Write-Host "SELECT server_id, name, base_url, timeout, sse_read_timeout, enabled FROM remote_mcp_configs ORDER BY server_id;"
    Write-Host ""
    Write-Host "Connection info:"
    Write-Host "  Host: $mysqlHost"
    Write-Host "  Port: $mysqlPort"
    Write-Host "  User: $mysqlUser"
    Write-Host "  Database: $mysqlDB"
    Write-Host ""
    Write-Host "To check specifically for ansible-mcp-server:"
    Write-Host "SELECT server_id, name, base_url, timeout, sse_read_timeout, enabled FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';"
} else {
    Write-Host "Executing MySQL query..." -ForegroundColor Green
    Write-Host ""
    
    # Query all MCP configs
    $query = "SELECT server_id, name, base_url, timeout, sse_read_timeout, enabled FROM remote_mcp_configs ORDER BY server_id;"
    try {
        $result = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query" 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "All MCP Configs:" -ForegroundColor Cyan
            $result | ForEach-Object { Write-Host $_ }
            Write-Host ""
            
            # Check specifically for ansible-mcp-server
            $checkQuery = "SELECT server_id, name, base_url, timeout, sse_read_timeout, enabled FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';"
            $checkResult = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$checkQuery" 2>&1
            if ($LASTEXITCODE -eq 0) {
                Write-Host "Checking for ansible-mcp-server:" -ForegroundColor Cyan
                if ($checkResult -match "ansible-mcp-server") {
                    Write-Host "⚠️  ansible-mcp-server still exists in database!" -ForegroundColor Red
                    $checkResult | ForEach-Object { Write-Host $_ }
                } else {
                    Write-Host "✅ ansible-mcp-server has been deleted from database" -ForegroundColor Green
                }
            }
        } else {
            Write-Host "Error executing query: $result" -ForegroundColor Red
        }
    } catch {
        Write-Host "Error executing MySQL query: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Query completed!"

