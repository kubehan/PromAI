# ---- Stage 1: 前端依赖下载 ----
FROM node:20-alpine AS frontend-deps
WORKDIR /build
COPY frontend/package*.json ./
RUN npm ci --prefer-offline --no-audit --omit=dev

# ---- Stage 2: 前端编译 ----
FROM node:20-alpine AS frontend-builder
WORKDIR /build
COPY --from=frontend-deps /build/node_modules ./node_modules/
COPY frontend/ .
# 跳过类型检查，仅运行vite打包 (Docker构建场景下类型检查在CI阶段已完成)
RUN npm run build:docker

# ---- Stage 3: Go依赖下载 ----
FROM golang:1.25-alpine AS go-deps
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
# 复制go模块文件和本地模块以支持 replace directive
COPY go.mod go.sum ./
COPY pi-local/ ./pi-local/
# go mod download 将依赖缓存到 $GOPATH/pkg/mod (默认为 /go/pkg/mod)
RUN go mod download

# ---- Stage 4: Go后端编译 ----
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
# 直接从官方 golang 镜像继承依赖缓存（buildx 跨阶段缓存）
# 或手动复制缓存的模块
COPY --from=go-deps /go/pkg/mod /go/pkg/mod
COPY go.mod go.sum ./
COPY pi-local/ ./pi-local/
COPY . .
# CGO编译，支持多架构构建（由buildx自动处理）
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -a -installsuffix cgo -o PromAI .

# ---- Stage 5: 最终运行镜像 ----
FROM alpine:3.21
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY --from=backend-builder /build/PromAI .
COPY --from=frontend-builder /build/dist ./frontend/dist/
COPY templates ./templates/
COPY config/config.yaml ./config/config.yaml
RUN mkdir -p /app/data /app/reports
EXPOSE 8091
VOLUME ["/app/data", "/app/reports"]
ENTRYPOINT ["./PromAI"]
CMD ["-config", "/app/config/config.yaml"]
