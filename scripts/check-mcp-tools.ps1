# Script to check tools stored in MySQL for MCP services

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

Write-Host "Checking tools stored in MySQL for MCP services..." -ForegroundColor Cyan
Write-Host ""

# Check if mysql client is available
$mysqlClientPath = (where.exe mysql 2>&1 | Select-Object -First 1)
if ($mysqlClientPath -like "*not found*" -or -not $mysqlClientPath) {
    Write-Host "MySQL client not found. Please run the following SQL queries manually:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "=" * 60
    Write-Host "SQL Query 1: Check ansible-mcp-server tools"
    Write-Host "=" * 60
    Write-Host "SELECT server_id, name, LENGTH(tools) as tools_length, LEFT(tools, 500) as tools_preview FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';"
    Write-Host ""
    Write-Host "SQL Query 2: Check kubernetes-mcp-server tools"
    Write-Host "=" * 60
    Write-Host "SELECT server_id, name, LENGTH(tools) as tools_length, LEFT(tools, 500) as tools_preview FROM remote_mcp_configs WHERE server_id = 'kubernetes-mcp-server';"
    Write-Host ""
    Write-Host "SQL Query 3: Get full tools JSON for ansible-mcp-server"
    Write-Host "=" * 60
    Write-Host "SELECT tools FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';"
    Write-Host ""
    Write-Host "Connection info:"
    Write-Host "  Host: $mysqlHost"
    Write-Host "  Port: $mysqlPort"
    Write-Host "  User: $mysqlUser"
    Write-Host "  Database: $mysqlDB"
} else {
    Write-Host "Checking ansible-mcp-server tools..." -ForegroundColor Green
    $query1 = "SELECT server_id, name, LENGTH(tools) as tools_length, LEFT(tools, 500) as tools_preview FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';"
    try {
        $result1 = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query1" 2>&1
        if ($LASTEXITCODE -eq 0) {
            $result1 | ForEach-Object { Write-Host $_ }
        }
    } catch {
        Write-Host "Error: $_" -ForegroundColor Red
    }
    
    Write-Host ""
    Write-Host "Checking kubernetes-mcp-server tools..." -ForegroundColor Green
    $query2 = "SELECT server_id, name, LENGTH(tools) as tools_length, LEFT(tools, 500) as tools_preview FROM remote_mcp_configs WHERE server_id = 'kubernetes-mcp-server';"
    try {
        $result2 = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query2" 2>&1
        if ($LASTEXITCODE -eq 0) {
            $result2 | ForEach-Object { Write-Host $_ }
        }
    } catch {
        Write-Host "Error: $_" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Expected tools for ansible-mcp-server:" -ForegroundColor Cyan
Write-Host "  - list_inventory" -ForegroundColor White
Write-Host "  - list_hosts" -ForegroundColor White
Write-Host "  - validate_playbook" -ForegroundColor White
Write-Host "  - ping_hosts" -ForegroundColor White
Write-Host "  - run_ad_hoc" -ForegroundColor White
Write-Host "  - get_ansible_version" -ForegroundColor White
Write-Host "  - generate_playbook" -ForegroundColor White
Write-Host "  - run_playbook" -ForegroundColor White
Write-Host ""
Write-Host "Kubernetes tools (should NOT be in ansible-mcp-server):" -ForegroundColor Yellow
Write-Host "  - configuration_view, events_list, helm_install, helm_list, helm_uninstall" -ForegroundColor White
Write-Host "  - namespaces_list, nodes_log, nodes_stats_summary, nodes_top" -ForegroundColor White
Write-Host "  - pods_delete, pods_exec, pods_get, pods_list, pods_list_in_namespace" -ForegroundColor White
Write-Host "  - pods_log, pods_run, pods_top" -ForegroundColor White
Write-Host "  - resources_create_or_update, resources_delete, resources_get, resources_list" -ForegroundColor White

