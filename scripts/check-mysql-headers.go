package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// MySQL 连接信息
	dsn := "root:canxixi@tcp(11.0.1.110:30306)/mcp?parseTime=true&charset=utf8mb4&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open MySQL: %v", err)
	}
	defer db.Close()

	// 测试连接
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping MySQL: %v", err)
	}

	fmt.Println("Connected to MySQL successfully!")
	fmt.Println("")

	// 查询 remote_mcp_configs 表
	rows, err := db.Query(`
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
			LEFT(headers, 200) as headers_preview
		FROM remote_mcp_configs
		ORDER BY server_id
	`)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	fmt.Println("Remote MCP Configs in MySQL:")
	fmt.Println("=" + string(make([]byte, 100)))
	fmt.Printf("%-25s %-25s %-40s %-15s %-15s %s\n", 
		"Server ID", "Name", "Base URL", "Headers Status", "Headers Length", "Headers Preview")
	fmt.Println("=" + string(make([]byte, 100)))

	for rows.Next() {
		var serverID, name, baseURL, headersStatus, headersPreview sql.NullString
		var headersLength sql.NullInt64

		err := rows.Scan(&serverID, &name, &baseURL, &headersStatus, &headersLength, &headersPreview)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		fmt.Printf("%-25s %-25s %-40s %-15s %-15d %s\n",
			serverID.String, name.String, baseURL.String, 
			headersStatus.String, headersLength.Int64, headersPreview.String)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating rows: %v", err)
	}

	fmt.Println("")
	fmt.Println("Query completed!")
}

