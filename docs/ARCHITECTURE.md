# SimplePool 架构设计（V1）

## 1. 目标

`SimplePool` 的 V1 是一个单管理员、单机自托管的节点池系统，负责：

- 管理代理节点
- 通过正则规则形成动态节点组
- 从组中创建 HTTP 代理隧道
- 为每个隧道分配独立 `sing-box` 运行时
- 在用户触发刷新时，从组内重新选择一个可用节点并切换

首版明确边界：

- 只支持 HTTP 隧道
- 不做多租户
- 不做多机调度
- 不做自动故障切换
- 不做自动定时切优

## 2. 已验证的 `sing-box` 事实

以下结论来自官方文档与 `2026-03-26` 的本机原型实测。

本机环境：

- 安装方式：`Homebrew`
- 版本：`sing-box 1.13.3`
- 可执行文件：`/opt/homebrew/bin/sing-box`
- 原型记录：`prototypes/p0/README.md`

文档层已确认：

1. `selector` 出站可维护一个候选出站列表，并可设置默认出站。
2. `selector` 当前只能通过 Clash API 控制。
3. `urltest` 出站可对一组出站做固定 URL 测试，支持 `url`、`interval`、`tolerance`、`idle_timeout`。
4. `experimental.clash_api` 可启用本地 REST 控制器，并支持 `secret`。
5. `http` 入站支持 `users`，为空时不要求认证。
6. `sing-box` 官方提供 `check` 和 `format` 命令，适合在生成配置后做校验和格式化。

参考：

- `selector`：https://sing-box.sagernet.org/zh/configuration/outbound/selector/
- `urltest`：https://sing-box.sagernet.org/zh/configuration/outbound/urltest/
- `clash_api`：https://sing-box.sagernet.org/zh/configuration/experimental/clash-api/
- `http` 入站：https://sing-box.sagernet.org/zh/configuration/inbound/http/
- 配置总览：https://sing-box.sagernet.org/zh/configuration/

实测层已确认：

1. 最小原型 `http inbound + selector + clash_api` 可直接启动。
2. `GET /proxies` 与 `GET /proxies/<selector-tag>` 可读到候选出站与当前 `now`。
3. `PUT /proxies/<selector-tag>`，请求体 `{"name":"<outbound-tag>"}`，返回 `204 No Content`，可热切换 `selector`。
4. 缺失认证时返回 `401 {"message":"Unauthorized"}`。
5. 切换到不存在的出站时返回 `400 {"message":"Selector update error: not found"}`。
6. 切换后下一条代理请求立即走新出站，不需要额外等待或同步。
7. `sing-box format -w` 适合放到配置写盘后执行；成功时主要看退出码。
8. `sing-box check` 适合放到启动前做最终校验；成功时退出码为 `0` 且通常无输出。
9. Clash API `delay` 在 `1.13.3` 实测中不适合作为业务主探测路径，自定义 `url` 参数表现不稳定，失败响应也过于笼统。
10. V1 的“独立探测”明确采用临时 `sing-box` 探测实例，不采用 Go 原生拨测。

关键结论：

- “不重启实例切换节点”已经实测成立，核心手段是 `selector + Clash API`。
- 切换接口已经确认，不再是阻塞项。
- `urltest` 更适合作为测量工具，而不是直接作为业务流量的最终出站，否则会天然偏向自动优选，与“手动刷新锁定”语义冲突。
- V1 节点延迟测试采用独立探测流程，不把 Clash API `delay` 当作核心协议。
- 所谓“独立探测”指按批次启动临时 `sing-box` 实例完成测试，测完即退出，不复用业务隧道实例。

## 3. 五层拆解

### 第 1 层：前端控制台

职责：

- 登录与会话维持
- 节点新增、导入、订阅管理
- 节点列表与延迟状态展示
- 组 CRUD 与成员预览
- 隧道 CRUD、刷新、状态展示

技术：

- `React`
- `shadcn/ui`
- 面板端口 `7891`

### 第 2 层：HTTP API

职责：

- 对外提供 REST API
- 统一参数校验、鉴权、错误码、审计日志

技术：

- `Gin`

### 第 3 层：领域服务

职责：

- `AuthService`
- `NodeService`
- `SubscriptionService`
- `GroupService`
- `TunnelService`
- `LatencyService`

这是业务规则层，不能直接依赖具体 UI。

### 第 4 层：运行时与适配器

职责：

- `SingboxRuntimeManager`
- `ConfigRenderer`
- `PortAllocator`
- `ProcessSupervisor`
- `ClashAPIClient`
- `SecretCipher`

这是系统最脆弱的一层，负责把数据库中的业务对象投影为实际运行中的 `sing-box` 进程。

### 第 5 层：持久化与系统资源

职责：

- `SQLite`
- 本地配置文件目录
- 本地运行时目录
- `sing-box` 可执行文件
- OS 端口与进程

## 4. 架构选项比较

### 方案 A：每个隧道一个 `sing-box` 实例

优点：

- 完全贴合已确认需求
- 隧道之间隔离清晰
- 刷新、删除、重建某个隧道时不会影响其他隧道
- 单个隧道的配置文件和日志容易定位

缺点：

- 进程数随隧道数量增长
- 资源占用高于单实例方案

潜在缺陷：

- 如果未来隧道数增长明显，进程管理和端口管理会变复杂

结论：

- V1 采用此方案

### 方案 B：单个全局 `sing-box` 实例承载所有隧道

优点：

- 进程少
- 统一管理容易

缺点：

- 配置规模会快速膨胀
- 任意一处配置重建都可能影响全局
- 单点故障更明显

潜在缺陷：

- 一次配置错误会让所有隧道一起失效

结论：

- 不适合 V1

### 方案 C：全局探测实例 + 每隧道业务实例

优点：

- 探测与业务流量隔离
- 有利于后续做更复杂的延迟探测

缺点：

- 组件更多
- 首版实现成本偏高

潜在缺陷：

- 容易为了“未来也许需要”而过度设计

结论：

- 暂不采用，后续若延迟探测成为瓶颈再引入

### 方案 D：按批次启动临时 `sing-box` 探测实例

优点：

- 复用 `sing-box` 对各类加密代理协议的兼容能力
- 不依赖任何运行中的业务隧道实例
- 不需要在 Go 内重复实现各协议握手和传输细节
- 生命周期短，状态简单，失败后容易整体回收

缺点：

- 每次探测都需要生成配置并拉起子进程
- 批量探测的冷启动成本高于复用现有实例

潜在缺陷：

- 如果批量节点数很大，单批配置体积和探测耗时需要继续控制

结论：

- V1 延迟测试采用此方案

## 5. 选定架构

V1 采用：

- 后端 API 进程：1 个
- 前端面板：1 个
- 每个隧道：1 个独立 `sing-box` 进程
- 工程实现直接嵌入 `sing-box` Go 库，不依赖额外 `sing-box` 二进制；`P0` 的命令行只用于原型验证

### 5.1 隧道运行时结构

每个隧道对应一份独立配置目录，例如：

```text
data/
  app.db
  runtime/
    <group-name>-<tunnel-name>/
      stdout.log
      stderr.log
```

补充：

- 渲染后的最新运行时配置持久化在 SQLite `tunnels.runtime_config_json`
- 运行时目录只保留日志，不再把 `config.json` 作为业务回滚真源，也不再为每个隧道落单独的 `cache.db`

对应配置的核心结构：

- `inbounds`
  - 一个 `http` 入站
  - 监听随机分配端口
  - 若用户提供认证，则填充 `users`
- `outbounds`
  - 组内每个节点一个真实代理出站
  - 一个 `selector` 出站，标签固定为 `tunnel-selector`
  - 一个 `direct` 出站作为系统用途保底
- `route`
  - 默认流量走 `tunnel-selector`
- `experimental.clash_api`
  - 监听 `127.0.0.1:<controller_port>`
  - 使用随机生成的 `secret`

### 5.2 刷新语义

刷新时执行：

1. 读取组快照
2. 排除当前正在使用的节点
3. 优先读取其余候选节点的近期成功探测缓存
4. 若存在成功缓存：
   - 从缓存命中的可用节点里随机选一个新节点
5. 若不存在成功缓存：
   - 并发探测其余候选节点
   - 首个成功结果立即用于刷新
   - 其他候选继续在后台完成探测并写入缓存
6. 若没有可用节点：
   - 保留当前已锁定节点
   - 返回错误
   - 记录失败原因
7. 若选出新节点：
   - 通过 Clash API 切换 `selector`
   - 更新数据库中的 `current_node_id`
   - 记录刷新时间与结果

补充结论：

- `selector` 切换一旦返回 `204`，下一条新请求即可按新节点生效。
- 不需要额外 sleep、轮询或同步。
- 但切换前必须先完成业务层健康检查，因为 Clash API 只校验 tag 是否存在，不校验目标节点是否可用。

### 5.3 组变更语义

V1 采用“创建时快照”：

- 组规则变化不会自动影响运行中隧道
- 订阅刷新导致节点集合变化也不会自动影响运行中隧道
- 隧道只有在“创建”或“刷新”时重新读取组当前成员

这样做的原因：

- 行为稳定
- 易于测试
- 可避免用户不知情的热变更

## 6. 数据模型

### 6.1 `admin_users`

用途：

- 管理后台登录账号

字段建议：

- `id`
- `username`
- `password_hash`
- `created_at`
- `updated_at`

说明：

- V1 可以只允许一个管理员，但表结构不必写死成单条配置。

### 6.2 `sessions`

用途：

- 后台会话

字段建议：

- `id`
- `user_id`
- `token_hash`
- `expires_at`
- `created_at`
- `last_seen_at`

### 6.3 `subscription_sources`

用途：

- 订阅源管理

字段建议：

- `id`
- `name`
- `url_ciphertext`
- `url_nonce`
- `enabled`
- `last_refresh_at`
- `last_error`
- `created_at`
- `updated_at`

说明：

- `id` 必须是系统生成的不可变主键，不要拿 URL 或名称充当源唯一标识。
- 新增 `fetch_fingerprint`，用于识别“同一抓取源被重复添加”。
- 订阅 URL 不能明文存库。

### 6.4 `nodes`

用途：

- 节点主表

字段建议：

- `id`
- `name`
- `source_node_key`
- `dedupe_fingerprint`
- `source_kind` `manual|import|subscription`
- `subscription_source_id` 可空
- `protocol`
- `server`
- `server_port`
- `credential_ciphertext`
- `credential_nonce`
- `transport_json`
- `tls_json`
- `raw_payload_json`
- `enabled`
- `last_latency_ms`
- `last_status` `unknown|healthy|unreachable`
- `last_checked_at`
- `created_at`
- `updated_at`

说明：

- 节点协议私有字段不要拆太碎，首版用 `raw_payload_json` 保底。
- 认证信息和敏感字段统一加密存储。
- 同一订阅源刷新时按 `source_node_key` 覆盖，不按名称覆盖。
- `dedupe_fingerprint` 只用于标记跨来源疑似重复节点，不用于跨来源强制合并。

### 6.5 `groups`

用途：

- 动态节点组

字段建议：

- `id`
- `name`
- `filter_regex`
- `description`
- `created_at`
- `updated_at`

说明：

- 组成员不落表，按正则实时计算。

### 6.6 `tunnels`

用途：

- HTTP 隧道主表

字段建议：

- `id`
- `name`
- `group_id`
- `listen_host`
- `listen_port`
- `status` `stopped|starting|running|degraded|error`
- `current_node_id` 可空
- `auth_username_ciphertext` 可空
- `auth_password_ciphertext` 可空
- `auth_nonce` 可空
- `controller_port`
- `controller_secret_ciphertext`
- `controller_secret_nonce`
- `runtime_dir`
- `last_refresh_at`
- `last_refresh_error`
- `created_at`
- `updated_at`

说明：

- 隧道认证信息不能只做哈希，因为运行时需要解密后下发给 `sing-box`。

### 6.7 `tunnel_events`

用途：

- 记录重要状态变更

字段建议：

- `id`
- `tunnel_id`
- `event_type`
- `detail_json`
- `created_at`

### 6.8 `latency_samples`

用途：

- 保存最近的延迟测试结果，便于 UI 展示

字段建议：

- `id`
- `node_id`
- `tunnel_id` 可空
- `test_url`
- `latency_ms`
- `success`
- `error_message`
- `created_at`

## 7. 后端模块划分

建议目录：

```text
cmd/
  simplepool-api/
internal/
  app/
  auth/
  config/
  crypto/
  domain/
  group/
  httpapi/
  node/
  runtime/
    singbox/
  store/
    sqlite/
  subscription/
  tunnel/
  testutil/
web/
```

核心原则：

- 业务规则不要散落在 handler
- 运行时控制不要混进 store
- `sing-box` 配置渲染必须可单测

## 8. API 草案

### 8.1 认证

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

### 8.2 节点

- `GET /api/nodes`
- `POST /api/nodes`
- `GET /api/nodes/:id`
- `PUT /api/nodes/:id`
- `DELETE /api/nodes/:id`
- `POST /api/nodes/import`
- `POST /api/nodes/:id/probe`
- `POST /api/nodes/probe`

### 8.3 订阅

- `GET /api/subscriptions`
- `POST /api/subscriptions`
- `PUT /api/subscriptions/:id`
- `DELETE /api/subscriptions/:id`
- `POST /api/subscriptions/:id/refresh`

### 8.4 组

- `GET /api/groups`
- `POST /api/groups`
- `GET /api/groups/:id`
- `PUT /api/groups/:id`
- `DELETE /api/groups/:id`
- `GET /api/groups/:id/members`

### 8.5 隧道

- `GET /api/tunnels`
- `POST /api/tunnels`
- `GET /api/tunnels/:id`
- `PUT /api/tunnels/:id`
- `DELETE /api/tunnels/:id`
- `POST /api/tunnels/:id/start`
- `POST /api/tunnels/:id/stop`
- `POST /api/tunnels/:id/refresh`
- `GET /api/tunnels/:id/events`

## 9. 关键流程

### 9.1 创建隧道

1. 读取组成员
2. 若组为空，返回错误
3. 执行延迟测试
4. 选出最优可用节点
5. 分配监听端口和本地 Clash API 端口
6. 生成 `sing-box` 配置
7. 执行 `sing-box format` 和 `sing-box check`
8. 启动进程
9. 写入 `tunnels` 状态为 `running`

### 9.2 刷新隧道

1. 读取组当前成员
2. 对每个候选节点测试可用性
3. 若存在更优可用节点，则切换 `selector`
4. 更新 `current_node_id`
5. 记录事件

### 9.3 删除隧道

1. 停止对应进程
2. 释放端口占用记录
3. 删除运行时目录
4. 删除数据库记录或标记软删除

V1 建议：

- 先做硬删除
- 事件表保留即可

## 10. 延迟测试设计

V1 目标：

- 有稳定、可复现的“可用/不可用”判定
- 在可用节点中按延迟排序
- 为 UI 提供最近一次测试结果

### 10.1 备选方案

方案一：直接依赖 `urltest`

优点：

- 借助 `sing-box` 原生能力

缺点：

- 天然偏自动切优
- 与“手动刷新锁定节点”语义存在张力

潜在缺陷：

- 若把业务流量直接绑到 `urltest`，可能在后台自动变更出口

方案二：使用 Clash 兼容探测接口

优点：

- 更接近真实代理路径
- 可按需触发

缺点：

- 官方文档没有展开具体 API 细节

潜在缺陷：

- 依赖未完全文档化的兼容接口

方案三：应用层自管探测任务

优点：

- 语义最清晰
- 完全由业务层控制

缺点：

- 需要额外构造每类代理协议的拨号与探测逻辑

潜在缺陷：

- 会重复造轮子
- 对加密代理协议基本不可接受，工程风险过高

### 10.2 V1 结论

V1 不把 `urltest` 直接作为业务流量出口。

P0 实测后的最终结论：

- 业务隧道靠 `selector` 锁定节点
- 延迟测试走独立探测流程，不依赖隧道实例上的 Clash API `delay`
- 独立探测明确为“按批次启动临时 `sing-box` 探测实例”

原因：

- Clash API `delay` 依赖已运行的控制器，不适合节点管理页的离线/批量探测
- `1.13.3` 实测中，自定义 `url` 参数表现不稳定，日志仍回退到 `www.gstatic.com`
- 失败响应过于笼统，不利于落库和 UI 展示
- Go 原生拨测无法覆盖这类加密代理协议，不能作为 V1 正解

执行语义：

- 单节点探测：
  - 生成只包含该节点的临时 `sing-box` 配置
  - 启动临时实例
  - 通过该实例访问测试目标并计时
  - 写入 `latency_samples`
  - 立即停止并清理实例
- 批量探测：
  - 按批次生成临时 `sing-box` 配置
  - 单批实例内装入待测节点
  - 逐个或受控并发地发起测试请求
  - 收集结果后停止并清理实例

### 10.3 默认探测参数

V1 默认值：

- 测试 URL：`https://cloudflare.com/cdn-cgi/trace`
- 超时：`3000ms`
- 并发：`8`
- 重试：`1`
- 结果缓存时长：`30s`

约束：

- 以上参数必须可配置
- 若部署环境对 `cloudflare.com` 可达性差，允许由应用配置覆盖

## 11. 安全设计

### 11.1 后台鉴权

- 固定账号密码登录
- 会话令牌只存哈希
- Cookie 使用 `HttpOnly`
- 本地自托管默认可不强制 HTTPS，但生产环境需要反代层兜底

### 11.2 敏感数据

以下字段必须加密落库：

- 订阅 URL
- 节点认证信息
- 隧道代理认证信息
- Clash API secret

建议：

- 使用单一应用主密钥
- 主密钥通过环境变量提供
- 加密算法使用 `AES-GCM`

### 11.3 本地控制面

- Clash API 只监听 `127.0.0.1`
- 始终设置 `secret`
- 不暴露外网

## 12. 前端信息架构

参考图可抽象为三栏逻辑：

1. 全局状态栏
   - 当前后台状态
   - 活跃隧道数
   - 节点可用数
2. 左侧导航/列表
   - 节点
   - 组
   - 隧道
   - 订阅
3. 右侧详情工作区
   - 统计卡片
   - 数据表格
   - 表单抽屉/弹窗
   - 事件与刷新结果

V1 页面建议：

- `/login`
- `/nodes`
- `/groups`
- `/tunnels`
- `/subscriptions`

## 13. 测试策略

### 13.1 优先级

首版测试优先级已经确认：

- 后端单元 + 集成优先
- 验收重点是隧道生命周期
- 失败路径必须覆盖：
  - 无可用节点 / 刷新失败
  - 订阅导入异常
  - 鉴权会话异常

### 13.2 单元测试

必须覆盖：

- 正则组成员计算
- 节点去重与订阅覆盖策略
- 端口分配器
- `sing-box` 配置渲染
- 敏感字段加解密
- 刷新策略选择器

### 13.3 集成测试

建议覆盖：

- SQLite 仓储
- 登录与会话
- 创建隧道
- 隧道刷新失败保留旧节点
- 删除隧道时清理运行时目录

### 13.4 暂不要求

- 前端端到端全量自动化
- 大规模性能压测

## 14. 已完成的技术验证

已完成 3 个最小原型：

1. 原型 A：最小 `selector + http inbound + clash_api` 配置已成功启动
2. 原型 B：已通过 Clash API 热切换 `selector`，无需重启实例
3. 原型 C：已验证 Clash API `delay` 不适合作为 V1 主探测路径，改为独立探测流程

结果摘要：

- 能创建 HTTP 代理
- 能在实例运行中切换当前节点
- 切换后下一条请求立即生效
- 失败时可以明确拿到 `401` / `400` / `502` 等结果
- 详细记录见 `prototypes/p0/README.md`

## 15. 实施顺序

1. 先做技术原型，确认 `sing-box` 控制链路
2. 落后端基础骨架
3. 落数据库迁移和仓储
4. 落节点、组、订阅 API
5. 落隧道运行时管理
6. 落前端控制台
7. 补测试与收尾

## 16. 当前阻塞点

`P0` 的 `sing-box` 技术阻塞已清理完成。

当前不再存在运行时方案上的硬阻塞。

后续只剩实现细节：

- 临时 `sing-box` 探测执行器如何落到代码结构
- 订阅源与节点指纹字段如何入库
- 删除策略是否保留软删除
