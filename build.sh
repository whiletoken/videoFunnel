#!/bin/bash

# 获取当前系统架构信息
ARCH=$(dpkg --print-architecture)

# 输出架构
echo "当前系统架构: $ARCH"

# 将架构信息导入到环境变量
export TARGETARCH=$ARCH

# 运行 docker-compose 构建
docker-compose up -d

