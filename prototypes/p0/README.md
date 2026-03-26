# P0 `sing-box` 原型验证记录

验证日期：

- `2026-03-26`

环境：

- 系统：`darwin/arm64`
- 可执行文件：`/opt/homebrew/bin/sing-box`
- 安装方式：`Homebrew`
- 版本：`sing-box 1.13.3`

原型文件：

- `prototypes/p0/minimal-selector.json`
- `prototypes/p0/invalid-selector.json`

最小原型结构：

- 一个 `http` 入站：`127.0.0.1:18080`
- 一个 `selector` 出站：`tunnel-selector`
- 两个候选出站：
  - `node-direct`
  - `node-block`
- 一个 Clash API 控制器：`127.0.0.1:19090`

本次本地闭环验证使用：

- 本地目标 HTTP 服务：`127.0.0.1:28080`
- 通过代理访问 `http://127.0.0.1:28080/TASKS.md`

## 1. `check` 与 `format`

成功路径：

```bash
sing-box format -w -c prototypes/p0/minimal-selector.json
sing-box check -c prototypes/p0/minimal-selector.json
```

观测结果：

- `format -w` 退出码为 `0`
- `format -w` stdout 只返回被写入的文件路径，适合把“退出码是否为 0”作为成功标准
- `check` 对合法配置退出码为 `0`，正常情况下无 stdout

失败路径：

```bash
sing-box check -c prototypes/p0/invalid-selector.json
```

观测结果：

- 退出码为 `1`
- stderr：

```text
FATAL decode config at prototypes/p0/invalid-selector.json: inbounds[0].listen_port: json: cannot unmarshal string into Go value of type uint16
```

结论：

- 运行时配置落盘后，应固定走：
  1. `sing-box format -w`
  2. `sing-box check`
  3. 原子替换正式配置
- 不要依赖 `format` stdout 解析结构化结果

## 2. Clash API `selector` 切换

启动原型：

```bash
sing-box run -c prototypes/p0/minimal-selector.json
```

启动后日志包含：

```text
inbound/http[http-in]: tcp server started at 127.0.0.1:18080
clash-api: restful api listening at 127.0.0.1:19090
```

读取代理视图：

```bash
curl -sS -H 'Authorization: Bearer simplepool-p0-secret' http://127.0.0.1:19090/proxies
```

关键字段：

```json
{
  "tunnel-selector": {
    "type": "Selector",
    "name": "tunnel-selector",
    "now": "node-direct",
    "all": ["node-direct", "node-block"]
  }
}
```

切换请求：

```bash
curl -i -sS \
  -X PUT \
  -H 'Authorization: Bearer simplepool-p0-secret' \
  -H 'Content-Type: application/json' \
  -d '{"name":"node-block"}' \
  http://127.0.0.1:19090/proxies/tunnel-selector
```

响应：

```text
HTTP/1.1 204 No Content
```

切换后查询：

```bash
curl -i -sS \
  -H 'Authorization: Bearer simplepool-p0-secret' \
  http://127.0.0.1:19090/proxies/tunnel-selector
```

响应：

```json
{
  "type": "Selector",
  "name": "tunnel-selector",
  "now": "node-block",
  "all": ["node-direct", "node-block"]
}
```

失败路径：

- 无认证头：

```text
HTTP 401
{"message":"Unauthorized"}
```

- 切换到不存在的节点：

```text
HTTP 400
{"message":"Selector update error: not found"}
```

结论：

- 单实例装入整组候选出站后，可以通过 Clash API 热切换 `selector`
- 核心接口就是：
  - `GET /proxies`
  - `GET /proxies/<selector-tag>`
  - `PUT /proxies/<selector-tag>`
- `PUT` 请求体固定使用：

```json
{"name":"<outbound-tag>"}
```

## 3. 切换后是否需要等待、同步或健康检查

默认状态下，代理请求：

```bash
curl -sS -o /tmp/simplepool-p0-direct.out -w '%{http_code} %{size_download}' \
  --max-time 3 \
  -x http://127.0.0.1:18080 \
  http://127.0.0.1:28080/TASKS.md
```

结果：

```text
200 9203
```

切到 `node-block` 后，同一请求结果：

```text
502 0
```

`sing-box` 日志直接显示下一条请求已经走到新出站：

```text
outbound/block[node-block]: blocked connection to 127.0.0.1:28080
```

再切回 `node-direct` 后，同一请求立即恢复：

```text
200 9203
```

结论：

- `selector` 切换后不需要额外等待
- 不需要额外同步动作
- 不需要为了“让切换生效”再做一次 Clash API 查询
- 但业务层在切换前仍然必须自己完成候选节点健康检查
  - Clash API 只校验 tag 是否存在
  - 不保证目标节点一定可用

## 4. 延迟探测方案验证

本次验证了 Clash 兼容 `delay` 接口：

```bash
curl -sS \
  -H 'Authorization: Bearer simplepool-p0-secret' \
  'http://127.0.0.1:19090/proxies/node-direct/delay?url=http%3A%2F%2F127.0.0.1%3A28080%2FTASKS.md&timeout=3000'
```

实际结果：

```json
{"message":"Timeout"}
```

同时 `sing-box` 日志显示它仍在访问：

```text
outbound/direct[node-direct]: outbound connection to www.gstatic.com:443
```

失败节点结果：

```bash
curl -sS \
  -H 'Authorization: Bearer simplepool-p0-secret' \
  'http://127.0.0.1:19090/proxies/node-block/delay?url=http%3A%2F%2F127.0.0.1%3A28080%2FTASKS.md&timeout=3000'
```

响应：

```json
{"message":"An error occurred in the delay test"}
```

结论：

- 不采用 Clash API `delay` 作为主探测路径
- 原因：
  - 它依赖已运行的 `sing-box` 控制器，不适合节点管理页的离线/批量探测
  - 在 `1.13.3` 实测中，自定义 `url` 参数表现不可靠，日志仍回退到 `www.gstatic.com`
  - 失败响应过于笼统，不利于业务层记录失败原因
- V1 采用独立探测流程
  - 不绑定业务隧道实例
  - 不把 Clash API `delay` 当作核心协议
  - 明确采用临时 `sing-box` 探测实例
  - 不采用 Go 原生拨测，因为节点协议本身带加密和专有握手

默认参数定稿：

- 测试 URL：`https://cloudflare.com/cdn-cgi/trace`
- 超时：`3000ms`
- 并发：`8`
- 重试：`1`
- 结果缓存时长：`30s`

补充要求：

- 上述参数必须可配置
- 若部署环境对 `cloudflare.com` 可达性差，允许在应用配置中覆盖

执行模型定稿：

- 单节点探测：
  - 启动一个只装该节点的临时 `sing-box` 实例
  - 通过临时实例访问测试目标并计时
  - 记录结果后立即退出
- 批量探测：
  - 按批次启动临时 `sing-box` 实例
  - 单批装入多个待测节点
  - 逐个或受控并发发起测试
  - 收集结果后立即退出

## 5. 订阅源唯一标识与节点去重规则

最终规则：

- 订阅源内部唯一标识：
  - 使用数据库生成的不可变 `source_id`
  - 不使用“来源名称”或“订阅 URL”做主键
- 订阅源重复添加检测：
  - 额外保存 `fetch_fingerprint`
  - 由规范化后的抓取描述计算，例如 `scheme + host + path + query + auth/header 摘要`
- 同一订阅源内的节点覆盖键：
  - 使用 `source_node_key`
  - 由 `source_id + 规范化后的节点核心指纹` 计算
- 跨订阅源节点去重：
  - 使用 `dedupe_fingerprint` 标记“疑似同一节点”
  - 但不同 `source_id` 之间默认不互相覆盖

节点核心指纹至少包含：

- 协议
- `server`
- `server_port`
- 用户标识 / 密码 / `uuid` 等鉴权主字段摘要
- 传输层关键字段摘要
- TLS 关键字段摘要

节点名称处理规则：

- 名称只作为展示字段
- 绝不使用“来源 + 名称”作为覆盖键

## 6. 对实现阶段的直接约束

- 隧道刷新：
  - 先探测
  - 再通过 Clash API 切换 `selector`
  - 不要先切再探测
- 节点管理页探测：
  - 不依赖现有隧道实例
  - 走临时 `sing-box` 探测执行器
- 配置生成：
  - 落盘后必须先 `format` 再 `check`
- 运行时控制：
  - Clash API 只监听 `127.0.0.1`
  - `secret` 必填
