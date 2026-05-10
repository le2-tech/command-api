# AGENTS.md

本文件给 Codex/自动化代理提供仓库约定与操作指引，便于安全、可重复地修改该项目。

## 项目概览
- 语言与框架：Go + Gin
- 入口：`main.go`
- 服务端口：`8080`
- 功能：提供 `POST /execute` 执行命令并流式输出

## 本地构建与运行
- 构建（推荐）：`make build`
- 直接构建：`CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`
- 运行（开发）：`go run .`
- 运行（编译后）：`./command-api`

## Docker
- 构建镜像：`docker build -t command-api .`
- 可选构建参数：`APP_ENV=dev` 时使用阿里云镜像源
- 运行镜像：`docker run -p 8080:8080 command-api`

## 环境变量
- `WHITELIST_CMDS`：逗号分隔的命令白名单，启用后仅允许列表内命令
- `GIN_MODE`：容器内默认 `release`

## API 约定
- `POST /execute`
- 请求体示例：`{"cmd":"ls","args":["-lah"]}`
- 返回：文本流（stdout+stderr 合并），`chunked` 传输

## 开发约定
- 格式化：使用 `gofmt`，避免非必要改动
- 依赖：如需更新依赖，先 `go mod tidy` 再提交
- 安全：涉及命令执行逻辑时必须保留白名单校验与超时控制
