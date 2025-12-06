# Script to fix ansible-mcp-server tools by removing Kubernetes tools

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

Write-Host "Fixing ansible-mcp-server tools..." -ForegroundColor Cyan
Write-Host ""

# Expected Ansible tools
$ansibleTools = @(
    "list_inventory",
    "list_hosts",
    "validate_playbook",
    "ping_hosts",
    "run_ad_hoc",
    "get_ansible_version",
    "generate_playbook",
    "run_playbook"
)

# Kubernetes tools that should NOT be in ansible-mcp-server
$kubernetesTools = @(
    "configuration_view",
    "events_list",
    "helm_install",
    "helm_list",
    "helm_uninstall",
    "namespaces_list",
    "nodes_log",
    "nodes_stats_summary",
    "nodes_top",
    "pods_delete",
    "pods_exec",
    "pods_get",
    "pods_list",
    "pods_list_in_namespace",
    "pods_log",
    "pods_run",
    "pods_top",
    "resources_create_or_update",
    "resources_delete",
    "resources_get",
    "resources_list"
)

Write-Host "This script will:" -ForegroundColor Yellow
Write-Host "  1. Read the current tools from MySQL for ansible-mcp-server" -ForegroundColor White
Write-Host "  2. Filter out Kubernetes tools" -ForegroundColor White
Write-Host "  3. Keep only Ansible tools" -ForegroundColor White
Write-Host "  4. Update the database" -ForegroundColor White
Write-Host ""
Write-Host "⚠️  WARNING: This will modify the database!" -ForegroundColor Red
$confirm = Read-Host "Do you want to continue? (yes/no)"
if ($confirm -ne "yes") {
    Write-Host "Cancelled." -ForegroundColor Yellow
    exit 0
}

# Check if mysql client is available
$mysqlClientPath = (where.exe mysql 2>&1 | Select-Object -First 1)
if ($mysqlClientPath -like "*not found*" -or -not $mysqlClientPath) {
    Write-Host "MySQL client not found. Please run the SQL update manually:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Step 1: Get current tools JSON" -ForegroundColor Cyan
    Write-Host "SELECT tools FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';" -ForegroundColor White
    Write-Host ""
    Write-Host "Step 2: Parse JSON and filter tools" -ForegroundColor Cyan
    Write-Host "Keep only tools with names in: $($ansibleTools -join ', ')" -ForegroundColor White
    Write-Host ""
    Write-Host "Step 3: Update database" -ForegroundColor Cyan
    Write-Host "UPDATE remote_mcp_configs SET tools = '<filtered_json>' WHERE server_id = 'ansible-mcp-server';" -ForegroundColor White
    Write-Host ""
    Write-Host "Or use the API to clear tools:" -ForegroundColor Cyan
    Write-Host "1. Get current config: GET /api/config/remote-mcp/ansible-mcp-server" -ForegroundColor White
    Write-Host "2. Remove tools array or set to empty array" -ForegroundColor White
    Write-Host "3. Update config: PUT /api/config/remote-mcp/ansible-mcp-server" -ForegroundColor White
    exit 1
}

Write-Host "Fetching current tools from database..." -ForegroundColor Green
$query = "SELECT tools FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';"
try {
    $result = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$query" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error fetching tools: $result" -ForegroundColor Red
        exit 1
    }
    
    # Parse the result (skip header line)
    $lines = $result | Where-Object { $_ -notmatch "tools" -and $_ -notmatch "^-" -and $_.Trim() -ne "" }
    if ($lines.Count -eq 0) {
        Write-Host "No tools found in database for ansible-mcp-server" -ForegroundColor Yellow
        Write-Host "This might be correct if tools haven't been cached yet." -ForegroundColor Yellow
        exit 0
    }
    
    $toolsJson = $lines -join "`n"
    Write-Host "Current tools JSON length: $($toolsJson.Length) characters" -ForegroundColor Cyan
    
    # Try to parse JSON using PowerShell
    try {
        $toolsArray = $toolsJson | ConvertFrom-Json
        Write-Host "Found $($toolsArray.Count) tools in database" -ForegroundColor Cyan
        
        # Filter tools
        $filteredTools = $toolsArray | Where-Object { 
            $toolName = $_.name
            $ansibleTools -contains $toolName
        }
        
        Write-Host "After filtering: $($filteredTools.Count) Ansible tools" -ForegroundColor Green
        Write-Host "Removed $($toolsArray.Count - $filteredTools.Count) Kubernetes tools" -ForegroundColor Yellow
        
        if ($filteredTools.Count -eq 0) {
            Write-Host "⚠️  No Ansible tools found. Setting tools to empty array." -ForegroundColor Yellow
            $newToolsJson = "[]"
        } else {
            $newToolsJson = ($filteredTools | ConvertTo-Json -Depth 10 -Compress)
        }
        
        # Escape single quotes for SQL
        $escapedJson = $newToolsJson -replace "'", "''"
        
        # Update database
        Write-Host ""
        Write-Host "Updating database..." -ForegroundColor Green
        $updateQuery = "UPDATE remote_mcp_configs SET tools = '$escapedJson' WHERE server_id = 'ansible-mcp-server';"
        $updateResult = & $mysqlClientPath -h $mysqlHost -P $mysqlPort -u $mysqlUser "-p$mysqlPassword" $mysqlDB -e "$updateQuery" 2>&1
        
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✅ Successfully updated ansible-mcp-server tools in database!" -ForegroundColor Green
            Write-Host ""
            Write-Host "Next steps:" -ForegroundColor Cyan
            Write-Host "  1. Restart the service" -ForegroundColor White
            Write-Host "  2. Refresh tools in Web UI" -ForegroundColor White
            Write-Host "  3. Verify only Ansible tools are shown" -ForegroundColor White
        } else {
            Write-Host "❌ Error updating database: $updateResult" -ForegroundColor Red
            exit 1
        }
        
    } catch {
        Write-Host "❌ Error parsing JSON: $_" -ForegroundColor Red
        Write-Host "Tools JSON:" -ForegroundColor Yellow
        Write-Host $toolsJson -ForegroundColor Gray
        exit 1
    }
    
} catch {
    Write-Host "❌ Error: $_" -ForegroundColor Red
    exit 1
}

