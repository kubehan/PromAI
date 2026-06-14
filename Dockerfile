# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS frontend-builder
WORKDIR /build
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# ---- Stage 2: Build Go backend ----
FROM golang:1.22-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o PromAI .

# ---- Stage 3: Runtime ----
FROM alpine:3.19
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY --from=backend-builder /build/PromAI .
COPY --from=frontend-builder /build/dist ./frontend/dist/
COPY templates ./templates/
COPY config/config.yaml ./config/config.yaml
RUN mkdir -p /app/data /app/reports
EXPOSE 8091
CMD ["./PromAI", "-config", "/app/config/config.yaml"]
