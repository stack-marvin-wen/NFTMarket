#!/bin/bash
set -e

echo "开始构建 EasySwap Backend..."

# 进入项目目录
cd "$(dirname "$0")"

# 下载依赖
# echo "下载依赖..."
# go mod tidy

# 构建
echo "构建应用..."
go build -ldflags="-s -w" -o bin/easyswap-backend ./src/main.go

echo "构建完成！可执行文件位于: bin/easyswap-backend"
# echo "运行命令: ./bin/easyswap-backend -conf ./config/config.toml"