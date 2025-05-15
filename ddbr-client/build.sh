#!/bin/bash
# 生成彩色输出的函数
print_green() {
  echo -e "\033[0;32m$1\033[0m"
}
print_yellow() {
  echo -e "\033[0;33m$1\033[0m"
}

print_yellow "==== 编译DDBR客户端 ===="

# 编译客户端
print_yellow "编译中..."
go build -o ddbr-client .

if [ $? -eq 0 ]; then
  print_green "编译成功！"
  print_yellow "使用方法示例："
  echo "./ddbr-client -cmd set -key testkey -value testvalue -gateway localhost:8080"
  echo "./ddbr-client -cmd get -key testkey -gateway localhost:8080"
else
  echo -e "\033[0;31m编译失败\033[0m"
  exit 1
fi 