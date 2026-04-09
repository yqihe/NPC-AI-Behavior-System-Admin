# MySQL 禁止红线

适用于所有使用 MySQL 的项目。

## 禁止事务一致性破坏

- **禁止**事务内方法混用 `s.db`（非事务连接）和 `tx`（事务连接）。SELECT 和 DELETE 必须在同一个 `tx` 上执行，否则 TOCTOU 竞态导致数据不一致
- **禁止**事务内检查引用（TOCTOU 防护）使用普通 SELECT。在 InnoDB REPEATABLE READ 下，普通 SELECT 是快照读，看不到并发事务的 INSERT。必须加 `FOR SHARE` 获取当前读 + 阻止并发写入

## 禁止查询注入风险

- **禁止** `LIKE` 查询不转义通配符 `%` 和 `_`。用户输入 `%` 会匹配所有记录。必须用 `escapeLike()` 转义
