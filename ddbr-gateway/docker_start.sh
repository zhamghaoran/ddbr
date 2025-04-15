#!/bin/bash
# docker-start.sh - 停止、删除旧容器并启动新的Docker Compose服务

# 设置颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}===== DDBR服务Docker部署脚本 =====${NC}"
echo "当前目录: $(pwd)"

# 检查docker和docker-compose是否已安装
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker未安装${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}错误: Docker Compose未安装${NC}"
    exit 1
fi

# 停止现有的容器
echo -e "${YELLOW}正在停止现有的容器...${NC}"
docker-compose down 2>/dev/null || true

# 强制删除相关容器（以防某些容器未被docker-compose停止）
echo -e "${YELLOW}确保所有相关容器已删除...${NC}"
docker rm -f $(docker ps -a | grep 'ddbr-\(gateway\|server\)' | awk '{print $1}') 2>/dev/null || true

# 确保配置目录存在
echo -e "${YELLOW}确保配置目录存在...${NC}"
mkdir -p configs


# 构建并启动容器
echo -e "${YELLOW}构建并启动容器...${NC}"
docker-compose build
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败，请检查错误信息${NC}"
    exit 1
fi

echo -e "${YELLOW}启动容器...${NC}"
docker-compose up -d
if [ $? -ne 0 ]; then
    echo -e "${RED}启动失败，请检查错误信息${NC}"
    exit 1
fi

# 等待服务健康检查
echo -e "${YELLOW}等待服务启动...${NC}"
sleep 5

# 检查容器状态
echo -e "${YELLOW}检查容器状态...${NC}"
docker-compose ps