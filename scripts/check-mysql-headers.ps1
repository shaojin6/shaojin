# 检查 MySQL 中的 headers 数据
# 需要先安装 MySQL 客户端或使用其他方式连接

$mysqlHost = "11.0.1.110"
$mysqlPort = "30306"
$mysqlUser = "root"
$mysqlPassword = "canxixi"
$mysqlDB = "mcp"

Write-Host "Checking MySQL headers data..."
Write-Host "Host: $mysqlHost"
Write-Host "Port: $mysqlPort"
Write-Host "Database: $mysqlDB"
Write-Host ""

# 检查是否有 mysql 命令行工具
$mysqlPath = Get-Command mysql -ErrorAction SilentlyContinue
if ($mysqlPath) {
    Write-Host "Found mysql client, querying database..."
    $query = @"
SELECT 
    server_id,
    name,
    base_url,
    CASE 
        WHEN headers IS NULL THEN 'NULL'
        WHEN headers = '' THEN 'EMPTY'
        WHEN headers = '{}' THEN 'EMPTY_JSON'
        ELSE 'HAS_DATA'
    END as headers_status,
    LENGTH(headers) as headers_length,
    LEFT(headers, 100) as headers_preview
FROM remote_mcp_configs
ORDER BY server_id;
"@
    
    $env:MYSQL_PWD = $mysqlPassword
    $query | mysql -h $mysqlHost -P $mysqlPort -u $mysqlUser $mysqlDB
} else {
    Write-Host "MySQL client not found. Please run the following SQL query manually:"
    Write-Host ""
    Write-Host "=" * 60
    Write-Host "SQL Query:"
    Write-Host "=" * 60
    Write-Host @"
SELECT 
    server_id,
    name,
    base_url,
    CASE 
        WHEN headers IS NULL THEN 'NULL'
        WHEN headers = '' THEN 'EMPTY'
        WHEN headers = '{}' THEN 'EMPTY_JSON'
        ELSE 'HAS_DATA'
    END as headers_status,
    LENGTH(headers) as headers_length,
    LEFT(headers, 100) as headers_preview,
    headers as full_headers
FROM remote_mcp_configs
ORDER BY server_id;
"@
    Write-Host ""
    Write-Host "Connection info:"
    Write-Host "  Host: $mysqlHost"
    Write-Host "  Port: $mysqlPort"
    Write-Host "  User: $mysqlUser"
    Write-Host "  Database: $mysqlDB"
}

