# ---- Stage 1: Build frontend ----
# 前端产物为架构无关静态文件，只在宿主架构(BUILDPLATFORM)下构建，
# 避免 npm ci 在 QEMU(arm64) 模拟下极慢/挂起；同时多架构镜像仍只针对 Go 做交叉编译。
# npm 从境外 runner 直连官方源(registry.npmjs.org)，replace-registry-host=always 强制重写
# lockfile 里固定的 npmmirror.com resolved 地址，避免跨国拉国内 CDN。
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder
WORKDIR /build
ENV NPM_CONFIG_REGISTRY=https://registry.npmjs.org \
    NPM_CONFIG_REPLACE_REGISTRY_HOST=always \
    NPM_CONFIG_FETCH_TIMEOUT=60000 \
    NPM_CONFIG_FETCH_RETRIES=2 \
    NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=30000
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ .
RUN npm run build

# ---- Stage 2: Build Go backend ----
FROM --platform=$TARGETPLATFORM golang:1.25-alpine AS backend-builder
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
COPY pi-local/ ./pi-local/
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -a -installsuffix cgo -o PromAI .

# ---- Stage 3: Runtime ----
FROM --platform=$TARGETPLATFORM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
  && apk add --no-cache tzdata ca-certificates curl sqlite
WORKDIR /app
COPY --from=backend-builder /build/PromAI .
COPY --from=frontend-builder /build/dist ./frontend/dist/
COPY deploy/sql ./deploy/sql/
COPY templates ./templates/
COPY config/config.yaml ./config/config.yaml
COPY skills   ./skills/
RUN mkdir -p /app/data /app/reports
EXPOSE 8091
VOLUME ["/app/data", "/app/reports"]
ENTRYPOINT ["./PromAI"]
CMD ["-config", "/app/config/config.yaml"]
