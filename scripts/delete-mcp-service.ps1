# Script to safely delete an MCP service (checks for Agent references first)

param(
    [Parameter(Mandatory=$true)]
    [string]$MCPServerId,
    
    [string]$ApiBase = "http://localhost:9090",
    
    [switch]$Force
)

Write-Host "Deleting MCP service: $MCPServerId" -ForegroundColor Cyan
Write-Host ""

# Step 1: Check for Agent references
Write-Host "Step 1: Checking for Agent references..." -ForegroundColor Yellow
try {
    $agents = Invoke-RestMethod -Uri "$ApiBase/api/config/agents" -Method GET -TimeoutSec 5
    $referencingAgents = $agents | Where-Object { $_.mcpServerId -eq $MCPServerId }
    
    if ($referencingAgents -and -not $Force) {
        Write-Host "⚠️  Found $($referencingAgents.Count) Agent(s) that reference '$MCPServerId':" -ForegroundColor Red
        Write-Host ""
        $referencingAgents | ForEach-Object {
            Write-Host "  - Agent ID: $($_.id), Name: $($_.name)" -ForegroundColor White
        }
        Write-Host ""
        Write-Host "❌ Cannot delete MCP service while Agents reference it!" -ForegroundColor Red
        Write-Host ""
        Write-Host "Please:" -ForegroundColor Yellow
        Write-Host "  1. Delete or modify these Agents first" -ForegroundColor Yellow
        Write-Host "  2. Then delete the MCP service" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Or use -Force to skip this check (not recommended)" -ForegroundColor Yellow
        exit 1
    } elseif ($referencingAgents -and $Force) {
        Write-Host "⚠️  Found $($referencingAgents.Count) Agent(s) that reference '$MCPServerId'" -ForegroundColor Yellow
        Write-Host "   Proceeding with deletion due to -Force flag..." -ForegroundColor Yellow
    } else {
        Write-Host "✅ No Agents reference '$MCPServerId'" -ForegroundColor Green
    }
} catch {
    Write-Host "❌ Error checking Agents: $_" -ForegroundColor Red
    exit 1
}

# Step 2: Verify MCP service exists
Write-Host ""
Write-Host "Step 2: Verifying MCP service exists..." -ForegroundColor Yellow
try {
    $mcp = Invoke-RestMethod -Uri "$ApiBase/api/config/remote-mcp/$MCPServerId" -Method GET -TimeoutSec 5
    Write-Host "✅ Found MCP service: $($mcp.name) (ServerID: $($mcp.serverId))" -ForegroundColor Green
} catch {
    if ($_.Exception.Response.StatusCode -eq 404) {
        Write-Host "⚠️  MCP service '$MCPServerId' not found" -ForegroundColor Yellow
        Write-Host "   It may have already been deleted." -ForegroundColor Yellow
        exit 0
    } else {
        Write-Host "❌ Error checking MCP service: $_" -ForegroundColor Red
        exit 1
    }
}

# Step 3: Delete MCP service
Write-Host ""
Write-Host "Step 3: Deleting MCP service..." -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "$ApiBase/api/config/remote-mcp/$MCPServerId" -Method DELETE -TimeoutSec 5
    Write-Host "✅ Successfully deleted MCP service: $MCPServerId" -ForegroundColor Green
} catch {
    Write-Host "❌ Error deleting MCP service: $_" -ForegroundColor Red
    exit 1
}

# Step 4: Verify deletion
Write-Host ""
Write-Host "Step 4: Verifying deletion..." -ForegroundColor Yellow
Start-Sleep -Seconds 1
try {
    $mcp = Invoke-RestMethod -Uri "$ApiBase/api/config/remote-mcp/$MCPServerId" -Method GET -TimeoutSec 5
    Write-Host "⚠️  MCP service still exists!" -ForegroundColor Red
    exit 1
} catch {
    if ($_.Exception.Response.StatusCode -eq 404) {
        Write-Host "✅ MCP service has been successfully deleted!" -ForegroundColor Green
        Write-Host ""
        Write-Host "Next steps:" -ForegroundColor Cyan
        Write-Host "  1. Restart the service" -ForegroundColor White
        Write-Host "  2. Verify the MCP service does not reappear" -ForegroundColor White
        Write-Host "  3. If it reappears, check:" -ForegroundColor White
        Write-Host "     - File backup (.config/web-config.json)" -ForegroundColor Gray
        Write-Host "     - Service logs for restore messages" -ForegroundColor Gray
        exit 0
    } else {
        Write-Host "⚠️  Unexpected error during verification: $_" -ForegroundColor Yellow
        exit 0
    }
}

