# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS frontend-builder
WORKDIR /build
COPY frontend/package*.json ./
RUN npm ci --prefer-offline --no-audit
COPY frontend/ .
RUN npm run build

# ---- Stage 2: Build Go backend ----
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
COPY pi-local/ ./pi-local/
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -a -installsuffix cgo -o PromAI .

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
