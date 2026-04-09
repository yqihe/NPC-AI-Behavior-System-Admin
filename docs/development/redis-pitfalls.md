# Redis 常见陷阱

编写 Redis 相关代码时主动检查。禁止红线见 `../standards/redis-red-lines.md`。

## 操作

- **Get 返回 redis.Nil**：key 不存在时返回 `redis.Nil`，用 `errors.Is(err, redis.Nil)` 判断
- **序列化一致**：存 `json.Marshal`，取 `json.Unmarshal` 到相同类型
- **key 命名**：统一前缀，避免与游戏服务端冲突。key 生成函数集中在 `store/redis/keys.go`

## 分布式锁

- **SetNX 锁必须设 expire**：防止持锁进程崩溃导致死锁
- **Unlock 用 DEL**：简单场景够用。高并发下需 Lua 脚本保证"只删自己的锁"

---

*踩到新坑时追加到对应分类下。*
