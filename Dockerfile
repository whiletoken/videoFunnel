# 使用官方 Go 镜像作为构建阶段的基础镜像
FROM golang:1.18 AS builder

# 设置工作目录
WORKDIR /app

# 将 go.mod 和 go.sum 文件复制到工作目录
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 将其余的应用程序源代码复制到工作目录
COPY . .

# 构建 Go 应用程序为静态二进制文件，避免 CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o myapp .

# 使用更小的基础镜像
FROM scratch

# 将构建阶段生成的二进制文件和静态文件复制到最小镜像中
COPY --from=builder /app/myapp .
COPY --from=builder /app/static ./static

# 暴露应用程序的端口
EXPOSE 9800

# 运行应用程序
CMD ["./myapp"]