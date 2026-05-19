# FnStore

飞牛 fnOS 应用合集，统一管理、统一构建。

## 包含应用

| 应用 | 说明 | 标签 |
|------|------|------|
| [ClashLite](ClashLite/) | 基于 mihomo 内核的代理管理应用 | `clashlite-v*` |
| [SubStore](SubStore/) | 基于 Sub-Store 的订阅管理应用 | `substore-v*` |

## 构建

每个应用有独立的构建工作流，通过对应标签触发：

```bash
# 构建 ClashLite
git tag clashlite-v2.0.0 && git push origin clashlite-v2.0.0

# 构建 SubStore
git tag substore-v1.0.1 && git push origin substore-v1.0.1
```

也可以在 Actions 页面手动触发。
