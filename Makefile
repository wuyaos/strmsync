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
ROOT_DIR := $(shell pwd)
FRONTEND_DIR := frontend
BACKEND_DIR := backend
DIST_DIR := dist
WEB_STATICS_DIR := $(DIST_DIR)/web_statics
BUILD_DIR := build
GO_BUILD_DIR := $(BUILD_DIR)/go
GO_BIN_DIR := $(GO_BUILD_DIR)/bin
GO_CACHE_DIR := $(ROOT_DIR)/$(GO_BUILD_DIR)/cache
GO_MOD_CACHE_DIR := $(ROOT_DIR)/$(GO_BUILD_DIR)/mod
GO_TMP_DIR := $(ROOT_DIR)/$(GO_BUILD_DIR)/tmp
VUE_BUILD_DIR := $(BUILD_DIR)/vue
VUE_CACHE_DIR := $(VUE_BUILD_DIR)/.vite
NPM_CACHE_DIR := $(VUE_BUILD_DIR)/npm-cache
AIR_TMP_DIR := $(BUILD_DIR)/air
ENV_FILE := .env
ENV_TEST_FILE := .env.test
ENV_TEMPLATE_FILE := .env.example
START_SCRIPT := scripts/prod-start.sh
DIST_ENV_FILE := $(DIST_DIR)/.env
DIST_ENV_TEMPLATE_FILE := $(DIST_DIR)/.env.example
DIST_START_SCRIPT := $(DIST_DIR)/prod-start.sh

# Go 构建参数
# 生产环境：去除调试信息、符号表，启用所有优化
GO_BUILD_FLAGS := -ldflags "-s -w -X 'main.appVersion=$(VERSION)' -X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)'" -trimpath
GO_BUILD_OUTPUT := $(DIST_DIR)/$(APP_NAME)
RUN_DIR := $(dir $(GO_BUILD_OUTPUT))
RUN_BIN := $(notdir $(GO_BUILD_OUTPUT))

# 平台和架构
PLATFORMS := linux windows darwin
ARCHS := amd64 arm64

# 读取环境变量（支持 .env 与 .env.test）
-include $(ENV_FILE)
-include $(ENV_TEST_FILE)
export

# 兼容变量映射（不写死端口）
VITE_BACKEND_PORT ?= $(PORT)
FRONTEND_PORT ?= 7786
GO_ENV := GOMODCACHE="$(GO_MOD_CACHE_DIR)" GOCACHE="$(GO_CACHE_DIR)" GOTMPDIR="$(GO_TMP_DIR)"
NPM_ENV := NPM_CONFIG_CACHE="$(NPM_CACHE_DIR)"

# ==================== 帮助信息 ====================

## help: 显示帮助信息
help:
	@echo "STRMSync 构建系统"
	@echo ""
	@echo "使用方法: make [target]"
	@echo ""
	@echo "开发目标:"
	@echo "  dev              一键启动开发环境（前后端，保留缓存）"
	@echo ""
	@echo "构建目标:"
	@echo "  build            完整构建（前端+后端）到 dist/"
	@echo "  frontend         仅构建前端到 dist/web_statics/"
	@echo "  backend          仅构建后端到 dist/ (优化体积)"
	@echo "  install-deps     安装所有依赖"
	@echo ""
	@echo "发布目标:"
	@echo "  release          构建多平台发布包到 dist/"
	@echo ""
	@echo "维护目标:"
	@echo "  clean            清理构建产物"
	@echo "  clean-all        清理构建产物和开发缓存"
	@echo "  free-ports       释放开发端口"
	@echo "  test             运行测试"
	@echo "  run              运行构建后的程序"
	@echo ""
	@echo "当前版本: $(VERSION)"

# ==================== 依赖安装 ====================

## install-deps: 安装前后端依赖
install-deps:
	@echo "==> 安装前端依赖..."
	cd $(FRONTEND_DIR) && $(NPM_ENV) npm install
	@echo "==> 安装后端依赖..."
	cd $(BACKEND_DIR) && $(GO_ENV) go mod download
	@echo "✓ 依赖安装完成"

# ==================== 前端构建 ====================

## frontend: 构建前端静态文件到 dist/web_statics/
frontend:
	@echo "==> 构建前端 ($(VERSION))..."
	@if [ ! -d "$(FRONTEND_DIR)/node_modules" ]; then \
		echo "前端依赖未安装，正在安装..."; \
		cd $(FRONTEND_DIR) && $(NPM_ENV) npm install; \
	fi
	@rm -rf $(WEB_STATICS_DIR)
	cd $(FRONTEND_DIR) && $(NPM_ENV) npm run build
	@mkdir -p "$(WEB_STATICS_DIR)"
	@cp -r "$(VUE_BUILD_DIR)/dist/." "$(WEB_STATICS_DIR)/"
	@echo "✓ 前端构建完成: $(WEB_STATICS_DIR)/"

# ==================== 后端构建 ====================

## backend: 构建后端可执行文件到 dist/
backend:
	@echo "==> 构建后端 ($(VERSION))..."
	@mkdir -p $(DIST_DIR)
	@mkdir -p $(GO_CACHE_DIR) $(GO_MOD_CACHE_DIR) $(GO_TMP_DIR)
	cd $(BACKEND_DIR) && $(GO_ENV) CGO_ENABLED=1 go build -p $(shell nproc) $(GO_BUILD_FLAGS) -o ../$(GO_BUILD_OUTPUT) ./cmd/server
	@echo "✓ 后端构建完成: $(GO_BUILD_OUTPUT)"
	@ls -lh $(GO_BUILD_OUTPUT)

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

## dev: 一键启动开发环境（缓存集中到 build/）
dev:
	@echo "==> 一键启动开发环境..."
	@bash "./scripts/dev.sh"

# ==================== 测试 ====================

## test: 运行测试
test:
	@echo "==> 运行后端测试..."
	cd $(BACKEND_DIR) && $(GO_ENV) go test -v -race -coverprofile=coverage.out ./...
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

## clean-all: 清理开发缓存并释放端口（不清理依赖）
clean-all: clean
	@echo "==> 清理开发缓存..."
	rm -rf "$(BUILD_DIR)"
	rm -rf "$(BACKEND_DIR)/tmp"
	@echo "==> 释放开发端口..."
	@-if command -v lsof > /dev/null; then \
		lsof -nP -t -iTCP:$(PORT) -sTCP:LISTEN | xargs -r kill -9; \
		lsof -nP -t -iTCP:$(FRONTEND_PORT) -sTCP:LISTEN | xargs -r kill -9; \
	fi
	@echo "✓ 清理完成"

## free-ports: 释放开发端口
free-ports:
	@echo "==> 释放开发端口..."
	@-if command -v lsof > /dev/null; then \
		lsof -nP -t -iTCP:$(PORT) -sTCP:LISTEN | xargs -r kill -9; \
		lsof -nP -t -iTCP:$(FRONTEND_PORT) -sTCP:LISTEN | xargs -r kill -9; \
	fi
	@echo "✓ 端口释放完成"

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
	@echo "⚠️  注意：跨平台编译禁用 CGO，sqlite3 将使用纯 Go 实现（性能略低）"
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
				cd $(BACKEND_DIR) && go build -p $(shell nproc) $(GO_BUILD_FLAGS) \
				-o "../$$release_dir/$${output_name}" \
				./cmd/server; \
			\
			cp -r $(WEB_STATICS_DIR) "$$release_dir/"; \
			cp VERSION "$$release_dir/" 2>/dev/null || true; \
			cp DEVELOPMENT.md "$$release_dir/" 2>/dev/null || true; \
			\
			size=$$(ls -lh "$$release_dir/$${output_name}" | awk '{print $$5}'); \
			echo "✓ $$platform/$$arch 构建完成 ($$size)"; \
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
