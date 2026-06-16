# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS frontend-builder
WORKDIR /build
# 优化：先复制package.json，单独一层，提升缓存命中率
COPY frontend/package*.json ./
RUN npm ci --prefer-offline --no-audit
# 源码变化时只重新编译，不重新下载依赖
COPY frontend/ .
RUN npm run build

# ---- Stage 2: Build Go backend ----
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
# 优化：先复制go.mod/go.sum，与源码分离
# 这样源码变化时，依赖下载层仍然使用缓存
COPY go.mod go.sum ./
RUN go mod download
# 源码变化时只重新编译，不重新下载依赖
COPY . .
# 优化编译参数：GOOS=linux 明确目标系统
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -a -installsuffix cgo -o PromAI .

# ---- Stage 3: Runtime ----
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
