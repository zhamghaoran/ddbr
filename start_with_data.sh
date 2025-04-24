#!/bin/bash
# 生成彩色输出的函数
print_green() {
  echo -e "\033[0;32m$1\033[0m"
}
print_yellow() {
  echo -e "\033[0;33m$1\033[0m"
}
print_red() {
  echo -e "\033[0;31m$1\033[0m"
}

print_yellow "==== 启动DDBR集群 - 带初始数据 ===="

# 1. 确保先停止所有容器
print_yellow "停止现有容器..."
docker-compose down

# 2. 生成初始数据
print_yellow "生成初始化数据..."
chmod +x ./generate_initial_data.sh
./generate_initial_data.sh

# 3. 确保目录权限正确
print_yellow "设置目录权限..."
chmod -R 777 ./ddbr-server/data

# 4. 启动Docker容器
print_green "启动容器..."
docker-compose up -d

# 5. 查看容器启动状态
print_yellow "容器状态:"
docker-compose ps

print_green "==== 启动完成 ===="
print_yellow "提示: 使用 'docker-compose logs -f' 查看日志" 