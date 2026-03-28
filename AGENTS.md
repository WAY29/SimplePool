前端的按钮都采用纯图标+tooltip，说明放在tooltip
隧道运行时写入 `stdout.log` / `stderr.log` 的日志级别必须严格跟随 `SIMPLEPOOL_LOG_LEVEL`，禁止出现高于该级别的 `debug` / `trace` 日志
修改前端的时候不要添加莫名其妙的description和hint，不需要向使用者解释设计或者进行说教