
# 构建阶段
FROM golang:latest AS builder

# 设置工作目录
WORKDIR /app

ARG APP_ENV=prod

RUN set -eux; \
  if [ "${APP_ENV:-}" = "dev" ]; then \
  sed -i "s|http://deb.debian.org|http://mirrors.aliyun.com|g" /etc/apt/sources.list.d/debian.sources; \
  fi

# 安装依赖工具
RUN set -eux; \
    apt-get update ; \
    apt-get install -y git ; \
    rm -rf /var/lib/apt/lists/*

# 复制 go.mod 和 go.sum 文件
COPY go.mod go.sum ./

# ENV GOPROXY=https://goproxy.cn,direct

# 单独下载依赖，缓存到 GOPATH/pkg/mod 目录中
RUN go mod download

# 将项目源代码复制到容器中
COPY . .

# 编译 Go 应用
#RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/your_project_name
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"

# 最终运行阶段
FROM alpine:latest

# 创建工作目录
WORKDIR /dist

# 将构建阶段的可执行文件复制到运行镜像中
COPY --from=builder /app/command-api .

# 暴露应用监听的端口
EXPOSE 8080

# 如果原镜像中有默认的 ENTRYPOINT，下面这行可以清空它
#ENTRYPOINT []

ENV GIN_MODE=release

# 启动应用
#CMD ["./command-api"]
ENTRYPOINT ["./command-api"]