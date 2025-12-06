# Script to clear incorrect tools from ansible-mcp-server via API

$ApiBase = "http://localhost:9090"

# Expected Ansible tools (should be kept)
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

# Kubernetes tools (should be removed from ansible-mcp-server)
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

Write-Host "Clearing incorrect tools from ansible-mcp-server..." -ForegroundColor Cyan
Write-Host ""

# Step 1: Get current config
Write-Host "Step 1: Fetching current config..." -ForegroundColor Yellow
try {
    $allConfigs = Invoke-RestMethod -Uri "$ApiBase/api/config/remote-mcp" -Method GET -TimeoutSec 5
    $config = $allConfigs | Where-Object { $_.serverId -eq "ansible-mcp-server" }
    if (-not $config) {
        Write-Host "❌ ansible-mcp-server not found in config list" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ Found config: $($config.name)" -ForegroundColor Green
} catch {
    Write-Host "❌ Error fetching config: $_" -ForegroundColor Red
    exit 1
}

# Step 2: Check current tools
Write-Host ""
Write-Host "Step 2: Checking current tools..." -ForegroundColor Yellow
if ($config.tools -and $config.tools.Count -gt 0) {
    Write-Host "Found $($config.tools.Count) tools in current config" -ForegroundColor Cyan
    
    # Count Kubernetes tools
    $k8sToolsCount = ($config.tools | Where-Object { $kubernetesTools -contains $_.name }).Count
    Write-Host "  - Kubernetes tools (to remove): $k8sToolsCount" -ForegroundColor Yellow
    
    # Count Ansible tools
    $ansibleToolsCount = ($config.tools | Where-Object { $ansibleTools -contains $_.name }).Count
    Write-Host "  - Ansible tools (to keep): $ansibleToolsCount" -ForegroundColor Green
    
    # Filter tools - keep only Ansible tools
    $filteredTools = $config.tools | Where-Object { $ansibleTools -contains $_.name }
    
    Write-Host ""
    Write-Host "After filtering:" -ForegroundColor Cyan
    Write-Host "  - Tools to keep: $($filteredTools.Count)" -ForegroundColor Green
    Write-Host "  - Tools to remove: $($config.tools.Count - $filteredTools.Count)" -ForegroundColor Yellow
    
    if ($filteredTools.Count -eq 0) {
        Write-Host ""
        Write-Host "⚠️  No Ansible tools found. Clearing all tools." -ForegroundColor Yellow
        $config.tools = @()
    } else {
        $config.tools = $filteredTools
    }
    
    # Step 3: Update config
    Write-Host ""
    Write-Host "Step 3: Updating config..." -ForegroundColor Yellow
    Write-Host "⚠️  WARNING: This will update the database!" -ForegroundColor Red
    $confirm = Read-Host "Do you want to continue? (yes/no)"
    if ($confirm -ne "yes") {
        Write-Host "Cancelled." -ForegroundColor Yellow
        exit 0
    }
    
    try {
        # Remove tools_last_update to force refresh
        $config.PSObject.Properties.Remove('toolsLastUpdate')
        
        # Update config
        $body = $config | ConvertTo-Json -Depth 10
        Invoke-RestMethod -Uri "$ApiBase/api/config/remote-mcp/ansible-mcp-server" -Method PUT -Body $body -ContentType "application/json" -TimeoutSec 10
        
        Write-Host "✅ Successfully updated ansible-mcp-server config!" -ForegroundColor Green
        Write-Host ""
        Write-Host "Next steps:" -ForegroundColor Cyan
        Write-Host "  1. Refresh tools in Web UI" -ForegroundColor White
        Write-Host "  2. Verify only Ansible tools are shown (should be 8 tools)" -ForegroundColor White
        Write-Host "  3. Verify kubernetes-mcp-server still has its tools" -ForegroundColor White
    } catch {
        Write-Host "❌ Error updating config: $_" -ForegroundColor Red
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "Response: $responseBody" -ForegroundColor Yellow
        }
        exit 1
    }
} else {
    Write-Host "✅ No tools found in config (this is correct if tools haven't been cached yet)" -ForegroundColor Green
    Write-Host ""
    Write-Host "The tools will be fetched correctly when you refresh in the Web UI." -ForegroundColor Cyan
}

