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

## 迁移管理

- **Docker initdb.d 只在首次初始化时执行**：`docker-entrypoint-initdb.d` 目录的 SQL 文件只在数据卷为空时运行。已有数据的容器新增迁移文件不会自动执行。需要手动 `docker exec -i mysql ... < migration.sql` 或重建数据卷
- **迁移文件合并时机**：V3 重写期间模块未上线，迁移文件应定期合并为干净的单文件，不要在旧版本上层层叠加 ALTER TABLE。上线后再用增量迁移

## 操作标识选择

- **CRUD 操作用主键 ID 不用 name**：VARCHAR 比较慢于 BIGINT，field_refs 等关联表用 INT JOIN 更高效。name 只用于创建时写入和唯一性校验，所有后续操作（详情/编辑/删除/引用）走 ID

---

*踩到新坑时追加到对应分类下。*
