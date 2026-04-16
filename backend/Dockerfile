# 多阶段构建
FROM golang:1.24-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

# 依赖缓存
COPY go.mod go.sum ./
RUN go mod download

# 在 builder 阶段安装证书和时区（避免运行镜像需要访问 Alpine CDN）
RUN apk --no-cache add ca-certificates tzdata

# 编译
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /admin ./cmd/admin
RUN CGO_ENABLED=0 GOOS=linux go build -o /seed ./cmd/seed

# 运行镜像
FROM alpine:3.19

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /admin /app/admin
COPY --from=builder /seed /app/seed
COPY config.yaml /app/config.yaml

EXPOSE 9821

CMD ["/app/admin", "-config", "/app/config.yaml"]
