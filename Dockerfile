
# Build stage
FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/golang:1.25.7-bookworm AS builder
# 设置 Go 环境变量
ENV GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct \
    GO111MODULE=on \
    GOSUMDB=off \
    CGO_ENABLED=0 \
    GOOS=linux
# Install build dependencies
# RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy module files first so dependency download can be cached across source changes.
COPY go.mod go.sum ./
RUN mkdir -p plugins/kubernetes
COPY plugins/kubernetes/go.mod ./plugins/kubernetes/go.mod
RUN go mod download

# Copy source code
COPY . .
# Build the application and Linux agent bundles
RUN mkdir -p /build/agent-bundles && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /build/agent-bundles/opshub-agent-linux-amd64 ./cmd/agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o /build/agent-bundles/opshub-agent-linux-arm64 ./cmd/agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -installsuffix cgo -o /build/agent-bundles/opshub-agent-windows-amd64 ./cmd/agent && \
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -a -installsuffix cgo -o /build/agent-bundles/opshub-agent-windows-arm64 ./cmd/agent && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /build/opshub main.go

# Runtime stage
FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/selectdb/alpine:latest

# Install ca-certificates, tzdata, database backup clients and kubectl
RUN apk --no-cache add ca-certificates tzdata curl mariadb-client postgresql-client && \
    curl -LO "https://mirrors.aliyun.com/kubernetes/kubectl/v1.29.0/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

# Set timezone
ENV TZ=Asia/Shanghai

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/opshub .
COPY --from=builder /build/agent-bundles ./agent-bundles

# Copy config template as default config
COPY config/config.yaml.example config/config.yaml

# Create logs and data directories
RUN mkdir -p logs data/prometheus/file_sd data/terminal-recordings

# Expose port
EXPOSE 9876

# Run the application
CMD ["./opshub", "server"]
