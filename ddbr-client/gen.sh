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

print_yellow "==== 生成DDBR客户端代码 ===="

# 检查kitex命令是否存在
if ! command -v kitex &> /dev/null; then
  print_red "错误: kitex 命令未找到，请先安装 Kitex"
  echo "可以通过以下命令安装: go install github.com/cloudwego/kitex/tool/cmd/kitex@latest"
  exit 1
fi

# 检查IDL文件是否存在
if [ ! -f "../idl/gateway.thrift" ]; then
  print_red "错误: 找不到IDL文件 ../idl/gateway.thrift"
  exit 1
fi

# 生成代码
print_yellow "开始生成Kitex客户端代码..."
kitex -module zhamghaoran/ddbr-client ../idl/gateway.thrift

# 检查生成结果
if [ $? -eq 0 ]; then
  print_green "客户端代码生成成功！"
  print_yellow "生成的代码在 kitex_gen 目录中"
else
  print_red "生成代码失败，请检查错误信息"
  exit 1
fi 