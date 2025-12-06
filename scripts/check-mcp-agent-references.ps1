# Script to check if any Agents reference a specific MCP service

param(
    [Parameter(Mandatory=$true)]
    [string]$MCPServerId,
    
    [string]$ApiBase = "http://localhost:9090"
)

Write-Host "Checking for Agents that reference MCP service: $MCPServerId" -ForegroundColor Cyan
Write-Host ""

try {
    $agents = Invoke-RestMethod -Uri "$ApiBase/api/config/agents" -Method GET -TimeoutSec 5
    
    $referencingAgents = $agents | Where-Object { $_.mcpServerId -eq $MCPServerId }
    
    if ($referencingAgents) {
        Write-Host "⚠️  Found $($referencingAgents.Count) Agent(s) that reference '$MCPServerId':" -ForegroundColor Yellow
        Write-Host ""
        $referencingAgents | ForEach-Object {
            Write-Host "  Agent ID: $($_.id)" -ForegroundColor White
            Write-Host "  Agent Name: $($_.name)" -ForegroundColor White
            Write-Host "  MCP Server ID: $($_.mcpServerId)" -ForegroundColor White
            Write-Host "  Enabled: $($_.enabled)" -ForegroundColor White
            Write-Host "  Is Default: $($_.isDefault)" -ForegroundColor White
            Write-Host ""
        }
        Write-Host "⚠️  WARNING: You should delete or modify these Agents before deleting the MCP service!" -ForegroundColor Red
        Write-Host ""
        Write-Host "To delete an Agent:" -ForegroundColor Cyan
        Write-Host "  Invoke-RestMethod -Uri '$ApiBase/api/config/agents/{agent-id}' -Method DELETE" -ForegroundColor Gray
        Write-Host ""
        Write-Host "To modify an Agent to use a different MCP service:" -ForegroundColor Cyan
        Write-Host "  1. Get the Agent: `$agent = Invoke-RestMethod -Uri '$ApiBase/api/config/agents/{agent-id}' -Method GET" -ForegroundColor Gray
        Write-Host "  2. Modify mcpServerId: `$agent.mcpServerId = 'other-mcp-server'" -ForegroundColor Gray
        Write-Host "  3. Update: Invoke-RestMethod -Uri '$ApiBase/api/config/agents/{agent-id}' -Method PUT -Body (`$agent | ConvertTo-Json) -ContentType 'application/json'" -ForegroundColor Gray
        exit 1
    } else {
        Write-Host "✅ No Agents reference '$MCPServerId'" -ForegroundColor Green
        Write-Host "   You can safely delete the MCP service." -ForegroundColor Green
        exit 0
    }
} catch {
    Write-Host "❌ Error checking Agents: $_" -ForegroundColor Red
    exit 1
}

