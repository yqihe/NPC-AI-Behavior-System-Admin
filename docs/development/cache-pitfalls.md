# 缓存模式常见陷阱

缓存设计模式层面的陷阱，不针对具体中间件。Redis 操作见 `redis-pitfalls.md`，缓存红线见 `../standards/cache-red-lines.md`。

## Cache-Aside 模式

- **写后必须清缓存**：DB 写入成功后，必须删除对应的缓存 key。先删缓存再写 DB 会导致并发请求写回旧数据
- **批量更新必须清 detail 缓存**：`BatchUpdateCategory` 修改了 category 字段，但如果只清 list 缓存不清 detail 缓存，用户查看字段详情时会看到旧数据。规则：凡是修改了 detail 中包含的字段，都要 `DelDetail`
- **级联操作必须清关联方缓存**：删除 reference 字段时会 `DecrRefCount` 被引用方，如果只清自己的 detail 缓存不清被引用方的缓存，会导致被引用方的 `ref_count` 在缓存中过期前仍是旧值。规则：事务中修改了其他记录的字段值，提交后必须清除这些记录的缓存

## 缓存穿透

- **空值标记**：查询不存在的 key 时，缓存一个 null marker（如 `{"_null":true}`），防止重复穿透到 DB。null marker 也要设 TTL

## 缓存雪崩

- **TTL 加随机抖动**：批量写入的缓存如果 TTL 相同，会在同一时刻过期，瞬间打穿 DB。`ttl(base, jitter)` 加随机偏移

## 缓存失效策略

- **版本号方案 vs SCAN 方案**：列表缓存用版本号（INCR version key，旧 key 自然过期），不用 SCAN 批量删除。SCAN 非原子且 key 量大时阻塞

---

*踩到新坑时追加到对应分类下。*
