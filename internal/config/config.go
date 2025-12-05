package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFile 从 .env 文件加载环境变量到当前进程
func LoadEnvFile(filePath string) error {
	// 如果文件不存在，返回 nil（不报错，允许使用环境变量）
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // 跳过格式不正确的行
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 移除引号（如果存在）
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// 只设置未存在的环境变量（环境变量优先级更高）
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file at line %d: %w", lineNum, err)
	}

	return nil
}

// FindConfigFile 查找配置文件，按优先级：
// 1. 当前工作目录的 .env
// 2. 可执行文件所在目录及父目录的 .env（向上查找最多3层）
// 3. 用户主目录的 .k8s-mcp.env
func FindConfigFile() string {
	// 1. 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		envFile := filepath.Join(cwd, ".env")
		if _, err := os.Stat(envFile); err == nil {
			return envFile
		}
	}

	// 2. 可执行文件所在目录及父目录（向上查找项目根目录）
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		// 尝试向上查找项目根目录（最多3层）
		for i := 0; i < 3; i++ {
			envFile := filepath.Join(execDir, ".env")
			if _, err := os.Stat(envFile); err == nil {
				return envFile
			}
			parentDir := filepath.Dir(execDir)
			if parentDir == execDir {
				break // 已到达根目录
			}
			execDir = parentDir
		}
	}

	// 3. 用户主目录
	if home := os.Getenv("HOME"); home != "" {
		envFile := filepath.Join(home, ".k8s-mcp.env")
		if _, err := os.Stat(envFile); err == nil {
			return envFile
		}
	} else if home := os.Getenv("USERPROFILE"); home != "" {
		envFile := filepath.Join(home, ".k8s-mcp.env")
		if _, err := os.Stat(envFile); err == nil {
			return envFile
		}
	}

	return ""
}

// LoadConfig 加载配置文件（自动查找）
// 如果找不到配置文件，会尝试加载当前目录的 .env 文件（如果存在）
func LoadConfig() error {
	configFile := FindConfigFile()
	if configFile == "" {
		// 没有找到配置文件，尝试使用当前目录的 .env
		if cwd, err := os.Getwd(); err == nil {
			configFile = filepath.Join(cwd, ".env")
		} else {
			configFile = ".env"
		}
	}

	return LoadEnvFile(configFile)
}

