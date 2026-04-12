# Redis 禁止红线

适用于所有使用 Redis 的项目。Go 见 `go.md`，MySQL 见 `mysql.md`，缓存模式见 `cache.md`。开发规范见 `../dev-rules/redis.md`。

## 禁止高危操作

- **禁止**用 `SCAN` + 批量 `DEL` 清除缓存。SCAN 非原子、key 量大时阻塞。用版本号方案：缓存 key 带版本号，失效时 INCR 版本号，旧 key 自然过期

## 禁止错误静默

- **禁止** Redis `DEL`/`Unlock` 不检查 error。锁泄漏和缓存脏数据的根源
