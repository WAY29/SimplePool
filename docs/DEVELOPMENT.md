# 开发约定

## 本地目录

默认目录如下：

- 数据目录：`./.simplepool/data`
- SQLite：`./.simplepool/data/simplepool.db`
- 运行时目录：`./.simplepool/runtime`
- 临时目录：`./.simplepool/tmp`

这些路径都可以通过环境变量覆盖。

## 关键环境变量

- `SIMPLEPOOL_DB_PATH`：SQLite 文件路径
- `SIMPLEPOOL_DATA_DIR`：数据目录，默认 `./.simplepool/data`
- `SIMPLEPOOL_RUNTIME_DIR`：`sing-box` 运行时目录
- `SIMPLEPOOL_TEMP_DIR`：临时文件目录
- `SIMPLEPOOL_MASTER_KEY`：Base64 编码的 32 字节主密钥
- `SIMPLEPOOL_MASTER_KEY_FILE`：主密钥文件路径，与 `SIMPLEPOOL_MASTER_KEY` 二选一
- `SIMPLEPOOL_ADMIN_USERNAME`：后台初始账号，默认 `admin`
- `SIMPLEPOOL_ADMIN_PASSWORD`：后台初始密码，必填
- `SIMPLEPOOL_HTTP_ADDR`：后端监听地址，默认 `127.0.0.1:7891`
- `SIMPLEPOOL_LOG_LEVEL`：日志级别，支持 `debug|info|warn|error`，同时用于 SimplePool 自身日志和所有嵌入式 `sing-box` 实例日志（包括隧道实例与延迟测试探测实例）

## 通过配置文件启动

后端入口支持通过 `--config` 显式指定一份 ENV 风格配置文件；文件内容仍然使用现有的 `SIMPLEPOOL_*` 键。Gin 默认以 `release` 模式启动，只有显式追加 `-debug` 才会启用 `debug` 模式。

- 示例文件：`.env.example`
- 直接运行：`go run ./cmd/simplepool-api --config .env.example`
- 调试运行：`go run ./cmd/simplepool-api --config .env.example -debug`
- 或使用任务：`mise run api -- --config .env.example`

## 前端嵌入

- `npm --prefix web run build` 会将前端打包到 `internal/httpapi/webui/dist`
- 后端启动后会直接托管嵌入后的前端静态资源
- 本地前端开发服务器改为 `5173`，并代理到后端默认端口 `7891`

## 常用任务

统一通过 `mise` 执行：

- `mise run api`：启动后端
- `mise run web`：启动前端开发服务器
- `mise run fmt`：格式化 Go 代码
- `mise run test`：运行 Go 单元测试
- `mise run check`：执行格式检查、静态检查、单元测试；若未安装前端依赖则跳过前端类型检查

补充：

- 项目运行时直接嵌入 `sing-box` Go 库，不要求额外安装 `sing-box` 可执行文件
