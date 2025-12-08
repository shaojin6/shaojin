package k8s

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client Kubernetes 客户端封装
type Client struct {
	Clientset *kubernetes.Clientset
	Config    *rest.Config
}

// NewClient 创建新的 Kubernetes 客户端
// 优先使用集群内配置（InCluster），如果不可用则使用 kubeconfig
// 支持通过环境变量 K8S_API_SERVER 等手动指定连接信息
func NewClient() (*Client, error) {
	var config *rest.Config
	var err error

	// 检查环境变量，支持手动指定 API Server
	if apiServer := os.Getenv("K8S_API_SERVER"); apiServer != "" {
		config = &rest.Config{
			Host: apiServer,
		}

		// 设置认证信息
		if token := os.Getenv("K8S_API_TOKEN"); token != "" {
			config.BearerToken = token
			// 尝试从 Token 中提取 ServiceAccount 信息（用于日志）
			// Token 是 JWT，可以解析但不显示完整内容
			log.Printf("[K8sClient] Using BearerToken authentication (Token length: %d)", len(token))
		} else if username := os.Getenv("K8S_API_USERNAME"); username != "" {
			config.Username = username
			config.Password = os.Getenv("K8S_API_PASSWORD")
			log.Printf("[K8sClient] Using Username/Password authentication (Username: %s)", username)
		} else {
			// 如果没有提供认证信息，返回错误
			return nil, fmt.Errorf("K8S_API_SERVER is set but no authentication provided. Please set K8S_API_TOKEN or K8S_API_USERNAME/K8S_API_PASSWORD")
		}

		// 如果使用 HTTP 但端口是 6443，自动转换为 HTTPS
		if strings.HasPrefix(apiServer, "http://") && strings.Contains(apiServer, ":6443") {
			apiServer = strings.Replace(apiServer, "http://", "https://", 1)
			config.Host = apiServer
			log.Printf("[K8sClient] Auto-converted HTTP to HTTPS for port 6443")
		}

		log.Printf("[K8sClient] Connecting to API server: %s", apiServer)

		// 设置 CA
		if caFile := os.Getenv("K8S_API_CA_FILE"); caFile != "" {
			config.CAFile = caFile
		} else if caData := os.Getenv("K8S_API_CA_DATA"); caData != "" {
			config.CAData = []byte(caData)
		}

		// 如果设置了 INSECURE，跳过 TLS 验证
		if os.Getenv("K8S_API_INSECURE") == "true" {
			config.Insecure = true
			config.TLSClientConfig = rest.TLSClientConfig{
				Insecure: true,
			}
		}

		// 设置超时时间
		config.Timeout = 30 * time.Second

		// 设置 User-Agent，避免某些 API 服务器拒绝连接
		config.UserAgent = "k8s-mcp-agent/1.0"

		// 如果 API Server 使用 HTTP 但实际需要 HTTPS，尝试自动转换
		// Kubernetes API 通常使用 HTTPS，如果配置为 HTTP 可能会失败
		if strings.HasPrefix(apiServer, "http://") && !strings.HasPrefix(apiServer, "https://") {
			// 检查是否是标准 Kubernetes 端口，如果是则尝试 HTTPS
			if strings.Contains(apiServer, ":6443") {
				log.Printf("[K8sClient] Warning: API server uses HTTP but port 6443 typically requires HTTPS. Consider using https://")
			}
		}
	} else {
		// 首先尝试使用集群内配置（当运行在 Pod 中时）
		config, err = rest.InClusterConfig()
		if err != nil {
			// 如果不在集群内，尝试使用 kubeconfig 文件
			var kubeconfig string
			if kubeconfig = os.Getenv("KUBECONFIG"); kubeconfig == "" {
				// 使用默认的 kubeconfig 路径
				home := homeDir()
				kubeconfig = filepath.Join(home, ".kube", "config")
			}

			// 检查文件是否存在
			if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
				return nil, fmt.Errorf("kubeconfig file not found at %s. Please set K8S_API_SERVER environment variable or create kubeconfig file", kubeconfig)
			}

			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
			}
		}

		// 设置超时时间
		config.Timeout = 30 * time.Second
	}

	// 设置 QPS 和 Burst，避免请求过快导致连接关闭
	if config.QPS == 0 {
		config.QPS = 10
	}
	if config.Burst == 0 {
		config.Burst = 20
	}

	// 创建客户端集
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset: clientset,
		Config:    config,
	}, nil
}

// NewClientFromConfig 根据 K8sConfig 创建 Kubernetes 客户端
// 支持 kubeconfig 和 manual 两种模式
func NewClientFromConfig(k8sConfig *types.K8sConfig) (*Client, error) {
	if k8sConfig == nil {
		return nil, fmt.Errorf("k8sConfig cannot be nil")
	}

	var config *rest.Config
	var err error

	if k8sConfig.Mode == "kubeconfig" {
		// kubeconfig 模式：从 content 字段读取 kubeconfig 内容（base64 编码）
		if k8sConfig.Content == "" {
			return nil, fmt.Errorf("kubeconfig content is empty")
		}

		// 解码 base64 内容
		kubeconfigData, err := base64.StdEncoding.DecodeString(k8sConfig.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode kubeconfig content: %w", err)
		}

		// 将 kubeconfig 内容写入临时文件
		tempFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := tempFile.Write(kubeconfigData); err != nil {
			return nil, fmt.Errorf("failed to write kubeconfig to temp file: %w", err)
		}
		tempFile.Close()

		// 从临时文件构建配置
		config, err = clientcmd.BuildConfigFromFlags("", tempFile.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	} else if k8sConfig.Mode == "manual" {
		// 手动配置模式
		if k8sConfig.Server == "" {
			return nil, fmt.Errorf("API server is required for manual mode")
		}

		config = &rest.Config{
			Host: k8sConfig.Server,
		}

		// 设置认证信息
		if k8sConfig.Token != "" {
			config.BearerToken = k8sConfig.Token
			log.Printf("[K8sClient] Using BearerToken authentication (Token length: %d)", len(k8sConfig.Token))
		} else if k8sConfig.Username != "" {
			config.Username = k8sConfig.Username
			config.Password = k8sConfig.Password
			log.Printf("[K8sClient] Using Username/Password authentication (Username: %s)", k8sConfig.Username)
		} else {
			return nil, fmt.Errorf("authentication is required: please provide Token or Username/Password")
		}

		// 如果使用 HTTP 但端口是 6443，自动转换为 HTTPS
		if strings.HasPrefix(k8sConfig.Server, "http://") && strings.Contains(k8sConfig.Server, ":6443") {
			server := strings.Replace(k8sConfig.Server, "http://", "https://", 1)
			config.Host = server
			log.Printf("[K8sClient] Auto-converted HTTP to HTTPS for port 6443")
		}

		log.Printf("[K8sClient] Connecting to API server: %s", config.Host)

		// 设置 CA 证书（优先级：CAFile > CAData > Insecure）
		if k8sConfig.CAFile != "" {
			config.CAFile = k8sConfig.CAFile
			log.Printf("[K8sClient] Using CA certificate file: %s", k8sConfig.CAFile)
		} else if k8sConfig.CAData != "" {
			// CAData 可能是 base64 编码的 PEM 内容，需要解码
			caDataBytes, err := base64.StdEncoding.DecodeString(k8sConfig.CAData)
			if err != nil {
				// 如果解码失败，可能是原始 PEM 内容，直接使用
				config.CAData = []byte(k8sConfig.CAData)
				log.Printf("[K8sClient] Using CA certificate data (raw PEM, length: %d)", len(k8sConfig.CAData))
			} else {
				config.CAData = caDataBytes
				log.Printf("[K8sClient] Using CA certificate data (base64 decoded, length: %d)", len(caDataBytes))
			}
		}

		// 如果设置了 Insecure，跳过 TLS 验证（但如果有 CA 证书，优先使用 CA 证书）
		if k8sConfig.Insecure && k8sConfig.CAFile == "" && k8sConfig.CAData == "" {
			config.Insecure = true
			config.TLSClientConfig = rest.TLSClientConfig{
				Insecure: true,
			}
			log.Printf("[K8sClient] TLS verification disabled (Insecure mode)")
		} else if k8sConfig.CAFile != "" || k8sConfig.CAData != "" {
			// 如果配置了 CA 证书，确保不使用 Insecure 模式
			config.Insecure = false
			log.Printf("[K8sClient] Using CA certificate for TLS verification")
		}

		// 设置超时时间
		config.Timeout = 30 * time.Second
	} else {
		return nil, fmt.Errorf("unsupported mode: %s (supported: kubeconfig, manual)", k8sConfig.Mode)
	}

	// 设置 QPS 和 Burst
	if config.QPS == 0 {
		config.QPS = 10
	}
	if config.Burst == 0 {
		config.Burst = 20
	}

	// 设置 User-Agent
	config.UserAgent = "k8s-mcp-agent/1.0"

	// 创建客户端集
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Config:    config,
	}, nil
}

// homeDir 获取用户主目录
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // Windows
}
