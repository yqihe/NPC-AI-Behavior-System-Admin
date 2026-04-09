# MongoDB 常见陷阱

编写 MongoDB 相关代码时主动检查。

## 连接与生命周期

- **连接泄漏**：`mongo.Client` 必须在 shutdown 时 `Disconnect`
- **Context 超时**：所有操作带 `context.WithTimeout`

## 操作

- **FindOne 无结果**：返回 `mongo.ErrNoDocuments`，用 `errors.Is` 判断
- **UpdateOne 无匹配**：`result.MatchedCount == 0` 返回 404
- **Duplicate key**：检查 `WriteErrors[i].Code == 11000` 转 409
- **bson.M key 顺序**：`bson.M` 是 map 无序，需要有序用 `bson.D`

## 集成测试

- **连真实 MongoDB**：不 mock，用 Docker 起测试库
- **测试清理**：每个用例用独立 collection 或 `TestMain` 清库

---

*踩到新坑时追加到对应分类下。*
