# STRMSync Makefile
# 前后端合并部署构建系统

.PHONY: help dev build clean frontend backend install-deps test release run prepare-dist

# 默认目标
.DEFAULT_GOAL := help

# ==================== 变量定义 ====================

APP_NAME := strmsync
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 目录定义
FRONTEND_DIR := frontend
BACKEND_DIR := backend
DIST_DIR := dist
WEB_STATICS_DIR := $(DIST_DIR)/web_statics
ENV_FILE := .env
ENV_TEMPLATE_FILE := .env.example
START_SCRIPT := scripts/prod-start.sh
DIST_ENV_FILE := $(DIST_DIR)/.env
DIST_ENV_TEMPLATE_FILE := $(DIST_DIR)/.env.example
DIST_START_SCRIPT := $(DIST_DIR)/prod-start.sh

# Go 构建参数
GO_BUILD_FLAGS := -ldflags "-s -w -X 'main.appVersion=$(VERSION)' -X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)'"
GO_BUILD_OUTPUT := $(DIST_DIR)/$(APP_NAME)
RUN_DIR := $(dir $(GO_BUILD_OUTPUT))
RUN_BIN := $(notdir $(GO_BUILD_OUTPUT))

# 平台和架构
PLATFORMS := linux windows darwin
ARCHS := amd64 arm64

# 端口配置
BACKEND_PORT := 6754
FRONTEND_PORT := 5676

# ==================== 帮助信息 ====================

## help: 显示帮助信息
help:
	@echo "STRMSync 构建系统"
	@echo ""
	@echo "使用方法: make [target]"
	@echo ""
	@echo "开发目标:"
	@echo "  dev              启动开发环境（前后端同时运行）"
	@echo "  dev-frontend     仅启动前端开发服务器"
	@echo "  dev-backend      仅启动后端开发服务器"
	@echo ""
	@echo "构建目标:"
	@echo "  build            完整构建（前端+后端）到 dist/"
	@echo "  frontend         仅构建前端到 dist/web_statics/"
	@echo "  backend          仅构建后端到 dist/"
	@echo "  install-deps     安装所有依赖"
	@echo ""
	@echo "发布目标:"
	@echo "  release          构建多平台发布包到 dist/"
	@echo ""
	@echo "维护目标:"
	@echo "  clean            清理构建产物"
	@echo "  test             运行测试"
	@echo "  run              运行构建后的程序"
	@echo ""
	@echo "当前版本: $(VERSION)"

# ==================== 依赖安装 ====================

## install-deps: 安装前后端依赖
install-deps:
	@echo "==> 安装前端依赖..."
	cd $(FRONTEND_DIR) && npm install
	@echo "==> 安装后端依赖..."
	cd $(BACKEND_DIR) && go mod download
	@echo "✓ 依赖安装完成"

# ==================== 前端构建 ====================

## frontend: 构建前端静态文件到 dist/web_statics/
frontend:
	@echo "==> 构建前端 ($(VERSION))..."
	@if [ ! -d "$(FRONTEND_DIR)/node_modules" ]; then \
		echo "前端依赖未安装，正在安装..."; \
		cd $(FRONTEND_DIR) && npm install; \
	fi
	@rm -rf $(WEB_STATICS_DIR)
	cd $(FRONTEND_DIR) && npm run build
	@echo "✓ 前端构建完成: $(WEB_STATICS_DIR)/"

# ==================== 后端构建 ====================

## backend: 构建后端可执行文件到 dist/
backend:
	@echo "==> 构建后端 ($(VERSION))..."
	@mkdir -p $(DIST_DIR)
	cd $(BACKEND_DIR) && go build $(GO_BUILD_FLAGS) -o ../$(GO_BUILD_OUTPUT) ./cmd/server
	@echo "✓ 后端构建完成: $(GO_BUILD_OUTPUT)"

# ==================== 完整构建 ====================

## build: 完整构建（前端+后端）到 dist/
build: frontend backend prepare-dist
	@echo ""
	@echo "=========================================="
	@echo "✓ 构建完成！"
	@echo "=========================================="
	@echo "版本: $(VERSION)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo ""
	@echo "产物:"
	@echo "  - 后端可执行文件: $(GO_BUILD_OUTPUT)"
	@echo "  - 前端静态文件: $(WEB_STATICS_DIR)/"
	@echo ""
	@echo "运行方式:"
	@echo "  ./$(GO_BUILD_OUTPUT)"
	@echo "  或: make run"

# ==================== 部署辅助 ====================

## prepare-dist: 复制启动脚本与环境变量文件到 dist/
prepare-dist:
	@mkdir -p "$(DIST_DIR)"
	@if [ -f "$(ENV_FILE)" ]; then \
		cp "$(ENV_FILE)" "$(DIST_ENV_FILE)"; \
	elif [ ! -f "$(DIST_ENV_FILE)" ] && [ -f "$(ENV_TEMPLATE_FILE)" ]; then \
		cp "$(ENV_TEMPLATE_FILE)" "$(DIST_ENV_FILE)"; \
	fi
	@if [ -f "$(ENV_TEMPLATE_FILE)" ]; then \
		cp "$(ENV_TEMPLATE_FILE)" "$(DIST_ENV_TEMPLATE_FILE)"; \
	fi
	@if [ -f "$(START_SCRIPT)" ]; then \
		cp "$(START_SCRIPT)" "$(DIST_START_SCRIPT)"; \
		chmod +x "$(DIST_START_SCRIPT)"; \
	fi

# ==================== 开发环境 ====================

## dev-frontend: 启动前端开发服务器
dev-frontend:
	@echo "==> 启动前端开发服务器 (http://localhost:$(FRONTEND_PORT))..."
	cd $(FRONTEND_DIR) && VITE_BACKEND_PORT=$(BACKEND_PORT) npm run dev

## dev-backend: 启动后端开发服务器
dev-backend:
	@echo "==> 启动后端开发服务器 (http://localhost:$(BACKEND_PORT))..."
	@if command -v air > /dev/null; then \
		air -c .air.toml; \
	else \
		echo "Air 未安装，使用 go run..."; \
		cd $(BACKEND_DIR) && go run ./cmd/server; \
	fi

## dev: 启动完整开发环境
dev:
	@echo "==> 启动开发环境（前后端并行）"
	@echo "提示: 使用 Ctrl+C 停止两个进程"
	@$(MAKE) -j 2 dev-backend dev-frontend

# ==================== 测试 ====================

## test: 运行测试
test:
	@echo "==> 运行后端测试..."
	cd $(BACKEND_DIR) && go test -v -race -coverprofile=coverage.out ./...
	@echo "==> 运行前端测试..."
	@if [ -f "$(FRONTEND_DIR)/package.json" ] && grep -q '"test"' $(FRONTEND_DIR)/package.json; then \
		cd $(FRONTEND_DIR) && npm run test; \
	else \
		echo "前端测试未配置，跳过"; \
	fi

# ==================== 清理 ====================

## clean: 清理构建产物
clean:
	@echo "==> 清理构建产物..."
	rm -rf $(DIST_DIR)
	rm -rf $(WEB_STATICS_DIR)
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(BACKEND_DIR)/coverage.out
	@echo "✓ 清理完成"

# ==================== 发布构建 ====================

## release: 构建多平台发布包
release: clean
	@echo "==> 构建多平台发布包 ($(VERSION))..."
	@mkdir -p $(DIST_DIR)

	# 构建前端（所有平台共用）
	@echo ""
	@echo "[1/3] 构建前端..."
	@$(MAKE) frontend

	# 构建各平台后端
	@echo ""
	@echo "[2/3] 构建多平台后端..."
	@for platform in $(PLATFORMS); do \
		for arch in $(ARCHS); do \
			echo ""; \
			echo "==> 构建 $$platform/$$arch..."; \
			output_name="$(APP_NAME)"; \
			if [ "$$platform" = "windows" ]; then \
				output_name="$(APP_NAME).exe"; \
			fi; \
			\
			release_dir="$(DIST_DIR)/$(APP_NAME)-$(VERSION)-$${platform}-$${arch}"; \
			mkdir -p "$$release_dir"; \
			\
			GOOS=$$platform GOARCH=$$arch CGO_ENABLED=0 \
				cd $(BACKEND_DIR) && go build $(GO_BUILD_FLAGS) \
				-o "../$$release_dir/$${output_name}" \
				./cmd/server; \
			\
			cp -r $(WEB_STATICS_DIR) "$$release_dir/"; \
			cp VERSION "$$release_dir/" 2>/dev/null || true; \
			cp README.md "$$release_dir/" 2>/dev/null || true; \
			\
			echo "✓ $$platform/$$arch 构建完成"; \
		done; \
	done

	# 打包
	@echo ""
	@echo "[3/3] 打包发布文件..."
	@cd $(DIST_DIR) && \
	for dir in $(APP_NAME)-$(VERSION)-*; do \
		if [ -d "$$dir" ]; then \
			platform=$$(echo $$dir | grep -o 'windows\|linux\|darwin'); \
			if [ "$$platform" = "windows" ]; then \
				zip -r "$${dir}.zip" "$$dir" > /dev/null; \
				echo "  ✓ $${dir}.zip"; \
			else \
				tar -czf "$${dir}.tar.gz" "$$dir"; \
				echo "  ✓ $${dir}.tar.gz"; \
			fi; \
		fi; \
	done

	@echo ""
	@echo "=========================================="
	@echo "✓ 发布包构建完成！"
	@echo "=========================================="
	@echo "版本: $(VERSION)"
	@echo "产物目录: $(DIST_DIR)/"
	@echo ""
	@ls -lh $(DIST_DIR)/*.{tar.gz,zip} 2>/dev/null || true

# ==================== 运行 ====================

## run: 运行构建后的程序
run:
	@if [ ! -f "$(GO_BUILD_OUTPUT)" ]; then \
		echo "可执行文件不存在，正在构建..."; \
		$(MAKE) build; \
	fi
	@if [ ! -d "$(WEB_STATICS_DIR)" ]; then \
		echo "前端静态文件不存在，正在构建..."; \
		$(MAKE) frontend; \
	fi
	@$(MAKE) prepare-dist
	@echo "==> 运行 $(APP_NAME) (工作目录: $(RUN_DIR))..."
	@cd "$(RUN_DIR)" && "./$(RUN_BIN)"
