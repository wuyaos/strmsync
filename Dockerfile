# 多阶段构建 Dockerfile（前后端一体）

# ==================== 阶段 1: 构建前端 ====================
FROM node:18-alpine AS frontend-builder

WORKDIR /frontend

# 复制前端依赖文件
COPY frontend/package*.json ./

# 安装依赖
RUN npm ci --production

# 复制前端源码
COPY frontend/ .

# 构建前端（生成 dist 目录）
RUN npm run build

# ==================== 阶段 2: 构建后端 ====================
FROM golang:1.21-alpine AS backend-builder

# 安装构建依赖
RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /build

# 复制后端依赖文件
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# 复制后端源码
COPY backend/ .

# 复制前端构建产物到后端的 static 目录
COPY --from=frontend-builder /frontend/dist ./static

# 构建后端（启用 CGO 以支持 SQLite，启用 embed）
RUN CGO_ENABLED=1 GOOS=linux go build -a \
    -ldflags '-extldflags "-static"' \
    -tags embed \
    -o strmsync ./cmd/server

# ==================== 阶段 3: 运行环境 ====================
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# 从构建阶段复制可执行文件
COPY --from=backend-builder /build/strmsync .

# 创建必要的目录
RUN mkdir -p /app/data /app/logs

# 暴露端口
EXPOSE 3000

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD wget --spider -q http://localhost:3000/api/health || exit 1

# 启动应用
CMD ["./strmsync"]
