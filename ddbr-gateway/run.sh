#!/bin/bash
# 执行脚本 run.sh

# 停止可能已存在的进程
pkill -f ddbr-gateway
# 编译 ddbr-gateway
echo "正在编译 ddbr-gateway..."
cd ../ddbr-gateway
go build -o ddbr-gateway .
# 创建日志目录
mkdir -p ../logs
# 启动 ddbr-gateway master节点
echo "启动 ddbr-gateway master节点..."
./ddbr-gateway -gateway master -port 8080 > ../logs/gateway-master.log 2>&1 &
echo "ddbr-gateway master启动成功，PID: $!"
# 等待3秒，确保master节点完全启动
echo "等待3秒..."
sleep 3

# 启动 ddbr-gateway 从节点
echo "启动 ddbr-gateway 从节点..."
./ddbr-gateway -gateway 127.0.0.1:8080 -port 8081 > ../logs/gateway-slave.log 2>&1 &
echo "ddbr-gateway slave启动成功，PID: $!"
