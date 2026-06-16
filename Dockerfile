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
COPY go.mod go.sum ./
RUN go mod download -x

# ---- Stage 4: Go后端编译 ----
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY --from=go-deps /build/go /go
COPY go.mod go.sum ./
COPY . .
# 优化编译参数：GOOS=linux GOARCH=amd64 明确目标平台
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -a -installsuffix cgo -o PromAI .

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
