# 多阶段构建
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 依赖缓存
COPY go.mod go.sum ./
RUN go mod download

# 编译
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /admin ./cmd/admin
RUN CGO_ENABLED=0 GOOS=linux go build -o /seed ./cmd/seed

# 运行镜像
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /admin /app/admin
COPY --from=builder /seed /app/seed
COPY config.yaml /app/config.yaml

EXPOSE 9821

CMD ["/app/admin", "-config", "/app/config.yaml"]
