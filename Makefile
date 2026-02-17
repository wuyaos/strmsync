# STRMSync Makefile
#
# Author: STRMSync Team

.PHONY: help build run test clean docker-build docker-up docker-down docker-logs mod-tidy gen-proto

# 默认目标
help:
	@echo "STRMSync Makefile Commands:"
	@echo "  make build        - 编译Go程序"
	@echo "  make run          - 本地运行程序"
	@echo "  make test         - 运行所有测试"
	@echo "  make clean        - 清理构建产物"
	@echo "  make docker-build - 构建Docker镜像"
	@echo "  make docker-up    - 启动Docker容器"
	@echo "  make docker-down  - 停止Docker容器"
	@echo "  make docker-logs  - 查看Docker日志"
	@echo "  make mod-tidy     - 整理Go依赖"
	@echo "  make gen-proto    - 生成CloudDrive2 gRPC代码"

# 编译
build:
	@echo "Building STRMSync..."
	@mkdir -p tests
	@cd backend && CGO_ENABLED=1 go build -o ../tests/strmsync .
	@echo "Build complete: ./tests/strmsync"

# 运行
run:
	@echo "Running STRMSync..."
	@cd backend && go run .

# 测试
test:
	@echo "Running tests..."
	@cd backend && go test -v -cover ./...

# 清理
clean:
	@echo "Cleaning..."
	@rm -f strmsync
	@rm -rf data/*.db data/*.db-* logs/*.log
	@echo "Clean complete"

# 整理依赖
mod-tidy:
	@echo "Tidying Go modules..."
	@cd backend && go mod tidy
	@echo "Go modules tidied"

# 生成CloudDrive2 gRPC代码
gen-proto:
	@echo "Generating CloudDrive2 gRPC code..."
	@./scripts/gen_clouddrive2_proto.sh

# Docker操作
docker-build:
	@echo "Building Docker image..."
	@docker-compose build
	@echo "Docker image built"

docker-up:
	@echo "Starting Docker containers..."
	@docker-compose up -d
	@echo "Containers started"

docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down
	@echo "Containers stopped"

docker-logs:
	@docker-compose logs -f
