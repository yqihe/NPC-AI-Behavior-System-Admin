# MySQL 常见陷阱

编写 MySQL 相关代码时主动检查。禁止红线见 `../standards/mysql-red-lines.md`。

## 事务与锁

- **事务内查询用 `tx` 不用 `s.db`**：`RemoveByRef` 等方法接收 `tx` 参数但内部用 `s.db` 做 SELECT，导致查询和删除不在同一事务中。TOCTOU 竞态可能导致 ref_count 不一致
- **REPEATABLE READ 下 TOCTOU**：事务内普通 SELECT 是快照读（读事务开始时的快照），看不到并发 INSERT。防 TOCTOU 必须用 `FOR SHARE`（共享锁）或 `FOR UPDATE`（排他锁）获取当前读
- **FOR SHARE vs FOR UPDATE**：只需防止并发写入用 FOR SHARE（允许多个事务同时读），需要独占用 FOR UPDATE。删除前的引用检查用 FOR SHARE 即可

## 查询

- **LIKE 转义**：用户输入 `%` 或 `_` 会变通配符，匹配所有记录。必须 `escapeLike()` 转义
- **覆盖索引列顺序**：`idx_list (deleted, id, ...)` 中 `id` 排在 `type`/`category` 前面，导致按 type 过滤时只能用 deleted 前缀。1000 行规模下无影响，10万+ 需调整为 `(deleted, type, category, id, ...)`
- **乐观锁 rows==0 语义模糊**：`UPDATE ... WHERE version = ?` 返回 `rows==0` 可能是版本冲突也可能是记录不存在。通过 service 层预检查弥补

---

*踩到新坑时追加到对应分类下。*
