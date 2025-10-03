# Build stage for the frontend
FROM node:20-alpine AS frontend-build
WORKDIR /app
COPY webui/package.json webui/pnpm-lock.yaml ./
RUN npm install -g pnpm && \
    pnpm install --frozen-lockfile
COPY webui/ .
RUN pnpm run build

# Build stage for the backend
FROM golang:1.25-alpine AS backend-build
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.io,direct go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -extldflags '-static'" -o llmio .

# Final stage
FROM alpine:latest

# 安装运行时依赖和时区数据
RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/db

WORKDIR /app

# Copy the binary from backend build stage
COPY --from=backend-build /app/llmio .

# Copy the built frontend from frontend build stage
COPY --from=frontend-build /app/dist ./webui/dist

# 创建非root用户
RUN addgroup -g 1000 llmio && \
    adduser -D -u 1000 -G llmio llmio && \
    chown -R llmio:llmio /app

USER llmio

EXPOSE 7070

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7070/ || exit 1

# Command to run the application
CMD ["./llmio"]