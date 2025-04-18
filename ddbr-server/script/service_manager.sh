#!/bin/bash

# 设置颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

BASEDIR=$(dirname "$0")
cd $BASEDIR/..
SERVICE_NAME="ddbr-server"
PID_FILE="./logs/server.pid"
BINARY="./$SERVICE_NAME"
CONFIG="./configs/server_config.json"

# 获取参数
MODE=$1  # master或者slave模式
PORT=$2  # 端口号

# 函数: 打印用法
function print_usage {
    echo -e "${YELLOW}使用方法: $0 {start|stop|restart|status} [master|slave] [port]${NC}"
    echo "  start    - 启动服务"
    echo "  stop     - 停止服务"
    echo "  restart  - 重启服务"
    echo "  status   - 查看服务状态"
    echo "  master   - 主节点模式 (默认: slave)"
    echo "  port     - 服务端口号 (默认: 配置文件中的端口)"
    exit 1
}

# 函数: 启动服务
function start_service {
    echo -e "${YELLOW}正在启动 $SERVICE_NAME...${NC}"
    
    # 检查服务是否已在运行
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p $PID > /dev/null; then
            echo -e "${RED}服务已在运行, PID: $PID${NC}"
            return 1
        else
            rm "$PID_FILE"
        fi
    fi
    
    # 准备命令参数
    COMMAND="$BINARY -config=$CONFIG"
    
    # 设置节点模式
    if [ "$MODE" == "master" ]; then
        COMMAND="$COMMAND -master=true"
    else
        COMMAND="$COMMAND -master=false"
    fi
    
    # 如果指定了端口，添加端口参数
    if [ ! -z "$PORT" ]; then
        COMMAND="$COMMAND -port=$PORT"
    fi
    
    # 创建日志目录
    mkdir -p logs
    
    # 在后台启动服务
    echo "执行命令: $COMMAND"
    nohup $COMMAND > ./logs/stdout.log 2> ./logs/stderr.log &
    PID=$!
    
    # 保存PID
    echo $PID > "$PID_FILE"
    
    # 等待检查服务是否成功启动
    sleep 2
    if ps -p $PID > /dev/null; then
        echo -e "${GREEN}服务已启动, PID: $PID${NC}"
        return 0
    else
        echo -e "${RED}服务启动失败，请检查日志文件${NC}"
        return 1
    fi
}

# 函数: 停止服务
function stop_service {
    echo -e "${YELLOW}正在停止 $SERVICE_NAME...${NC}"
    
    # 检查PID文件是否存在
    if [ ! -f "$PID_FILE" ]; then
        echo -e "${RED}PID文件不存在, 服务可能未运行${NC}"
        return 1
    fi
    
    # 读取PID并尝试优雅停止
    PID=$(cat "$PID_FILE")
    if ps -p $PID > /dev/null; then
        kill -15 $PID
        echo "发送SIGTERM信号到进程 $PID"
        
        # 等待进程结束
        COUNTER=0
        while ps -p $PID > /dev/null && [ $COUNTER -lt 10 ]; do
            sleep 1
            let COUNTER=COUNTER+1
        done
        
        # 如果进程仍在运行，强制终止
        if ps -p $PID > /dev/null; then
            echo -e "${YELLOW}服务未响应，强制终止...${NC}"
            kill -9 $PID
            sleep 1
        fi
        
        # 检查进程是否已终止
        if ps -p $PID > /dev/null; then
            echo -e "${RED}无法终止服务，PID: $PID${NC}"
            return 1
        else
            echo -e "${GREEN}服务已停止${NC}"
            rm "$PID_FILE"
            return 0
        fi
    else
        echo -e "${YELLOW}进程不存在，可能已停止${NC}"
        rm "$PID_FILE"
        return 0
    fi
}

# 函数: 检查服务状态
function check_status {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p $PID > /dev/null; then
            echo -e "${GREEN}$SERVICE_NAME 正在运行, PID: $PID${NC}"
            return 0
        else
            echo -e "${YELLOW}$SERVICE_NAME 未运行, 但PID文件存在${NC}"
            return 1
        fi
    else
        echo -e "${RED}$SERVICE_NAME 未运行${NC}"
        return 2
    fi
}

# 主程序逻辑
ACTION=$1
shift

case "$ACTION" in
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        stop_service
        sleep 2
        start_service
        ;;
    status)
        check_status
        ;;
    *)
        print_usage
        ;;
esac

exit $? 