.PHONY: build build-cli build-web test clean docker-build deploy

# 构建 CLI 二进制文件
build-cli:
	go build -o bin/k8s-mcp-agent ./cmd/main.go

# 构建 Web 二进制文件
build-web:
	go build -o bin/k8s-mcp-web ./cmd/web/main.go

# 构建所有
build: build-cli build-web

# 运行测试
test:
	go test ./...

# 清理构建产物
clean:
	rm -rf bin/

# 构建 Docker 镜像
docker-build:
	docker build -t k8s-mcp-agent:latest .

# 部署到 Kubernetes
deploy:
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/deployment.yaml

# 本地运行 CLI
run-cli:
	go run ./cmd/main.go

# 本地运行 Web
run-web:
	go run ./cmd/web/main.go

# 格式化代码
fmt:
	go fmt ./...

# 安装依赖
deps:
	go mod download
	go mod tidy

