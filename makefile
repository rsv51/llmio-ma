# LLMIO Makefile - 简化开发和部署流程

.PHONY: help build run test clean docker-build docker-run dev frontend backend install

# 默认目标
.DEFAULT_GOAL := help

# 变量定义
APP_NAME := llmio
DOCKER_IMAGE := $(APP_NAME):latest
GO_FILES := $(shell find . -name '*.go' -type f)
FRONTEND_DIR := webui

# 帮助信息
help: ## 显示帮助信息
	@echo "LLMIO - 可用的 Make 命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

# 安装依赖
install: ## 安装所有依赖
	@echo "安装 Go 依赖..."
	go mod download
	go mod tidy
	@echo "安装前端依赖..."
	cd $(FRONTEND_DIR) && npm install -g pnpm && pnpm install

# 整理依赖
tidy: ## 整理 Go 依赖
	go mod tidy

# 格式化代码
fmt: ## 格式化 Go 代码
	go fmt ./...

# 构建
build: backend frontend ## 构建后端和前端

backend: fmt tidy ## 构建后端
	@echo "构建后端..."
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(APP_NAME) .

frontend: ## 构建前端
	@echo "构建前端..."
	cd $(FRONTEND_DIR) && pnpm run build

webui: frontend ## 构建前端（别名）

# 运行
run: fmt tidy ## 运行应用
	go run .

dev: fmt tidy ## 开发模式（热重载）
	@echo "启动开发模式..."
	go run main.go

dev-frontend: ## 前端开发模式
	@echo "启动前端开发服务器..."
	cd $(FRONTEND_DIR) && pnpm run dev

# 测试
test: ## 运行测试
	@echo "运行测试..."
	go test -v ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "运行测试覆盖率分析..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

benchmark: ## 运行基准测试
	@echo "运行基准测试..."
	go test -bench=. -benchmem ./test_performance/

performance: ## 运行性能测试脚本
	@echo "运行性能测试..."
	@chmod +x scripts/performance_test.sh
	@./scripts/performance_test.sh

# Docker
docker-build: ## 构建 Docker 镜像
	@echo "构建 Docker 镜像..."
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## 运行 Docker 容器
	@echo "运行 Docker 容器..."
	docker run -p 7070:7070 -e TOKEN=${TOKEN} -v ./db:/app/db $(DOCKER_IMAGE)

docker-compose-up: ## 使用 docker-compose 启动
	@echo "使用 docker-compose 启动..."
	docker-compose up -d

docker-compose-down: ## 停止 docker-compose
	@echo "停止 docker-compose..."
	docker-compose down

docker-logs: ## 查看 Docker 日志
	docker-compose logs -f

# Git 操作
add: fmt tidy ## Git add (格式化后)
	git add .

# 清理
clean: ## 清理构建文件
	@echo "清理构建文件..."
	@rm -f $(APP_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf $(FRONTEND_DIR)/dist
	@echo "清理完成"

clean-all: clean ## 清理所有（包括依赖）
	@rm -rf $(FRONTEND_DIR)/node_modules
	@echo "完全清理完成"

clean-docker: ## 清理 Docker 镜像和容器
	@echo "清理 Docker 资源..."
	docker-compose down -v
	docker rmi $(DOCKER_IMAGE) 2>/dev/null || true

# 代码质量
lint: ## 运行代码检查
	@echo "运行 Go 代码检查..."
	@which golangci-lint > /dev/null || (echo "请先安装 golangci-lint: make install-tools" && exit 1)
	golangci-lint run
	@echo "运行前端代码检查..."
	cd $(FRONTEND_DIR) && pnpm run lint

# 数据库
db-init: ## 初始化数据库
	@echo "初始化数据库..."
	@mkdir -p db
	@echo "数据库目录已创建"

db-backup: ## 备份数据库
	@echo "备份数据库..."
	@mkdir -p backups
	@cp db/llmio.db backups/llmio_$(shell date +%Y%m%d_%H%M%S).db 2>/dev/null || echo "数据库文件不存在"
	@echo "数据库已备份"

# 部署
deploy: clean build docker-build ## 完整部署流程
	@echo "部署完成"

# 开发工具
install-tools: ## 安装开发工具
	@echo "安装开发工具..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "开发工具安装完成"

# 版本信息
version: ## 显示版本信息
	@echo "LLMIO 版本信息:"
	@echo "Go: $$(go version)"
	@echo "Node: $$(node --version 2>/dev/null || echo 'not installed')"
	@echo "Docker: $$(docker --version 2>/dev/null || echo 'not installed')"