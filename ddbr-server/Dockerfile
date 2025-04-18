FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
# 下载依赖
# 设置Go模块代理为阿里云源
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

RUN go mod download
# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o ddbr-server .

# 使用轻量级基础镜像
FROM alpine:latest
# 安装基本工具
RUN apk --no-cache add ca-certificates netcat-openbsd
WORKDIR /app
COPY --from=builder /app/ddbr-server .
COPY --from=builder /app/server_config.json .
# 创建数据目录
RUN mkdir -p /app/data
# 暴露端口
EXPOSE 8082

ENTRYPOINT ["./ddbr-server","-config","/app/server_config.json"]