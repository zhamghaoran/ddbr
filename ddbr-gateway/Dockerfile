FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
# 下载依赖
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

RUN go mod download
# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o ddbr-gateway .
# 使用轻量级基础镜像
FROM alpine:latest
# 安装基本工具和tzdata时区包
RUN apk --no-cache add ca-certificates netcat-openbsd tzdata
# 设置时区为中国时区
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/ddbr-gateway .
# 创建数据目录
RUN mkdir -p /app/data
# 暴露端口
EXPOSE 8080 8081

ENTRYPOINT ["./ddbr-gateway"]