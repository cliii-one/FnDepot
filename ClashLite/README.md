# ClashLite

基于 vernesong/mihomo (Alpha-Smart) 内核的飞牛 fnOS 代理管理应用，集成 metacubexd 官方管理面板。

## 功能特性

- **Smart 代理组**：基于 LightGBM 的智能节点选择（vernesong/mihomo Alpha-Smart 独有）
- **metacubexd 前端**：mihomo 直接服务面板，无需额外代理层
- **多架构支持**：X86_64 / ARM64 双架构自动构建
- **OneSmart 配置**：内置一键智能策略组模板
- **预装数据**：CI 构建时预装 mihomo 内核 + LightGBM 模型 + GeoIP 数据，开箱即用
- **配置持久化**：配置文件存放在 data-share 共享目录，升级/重装不丢失
- **安装向导**：支持自定义 API 密钥和订阅源地址
- **配置向导**：安装后可通过飞牛应用设置修改密钥和订阅源

## 项目结构

```
.
├── .github/workflows/build.yml    # GitHub Actions 双架构构建
├── app/
│   ├── server/
│   │   ├── config/default.yaml    # mihomo 配置模板（占位符）
│   │   ├── data/mihomo/           # 预装内核 + 模型 + GeoData
│   │   └── public/                # metacubexd 前端文件
│   └── ui/
│       ├── config                 # 飞牛桌面入口（iframe → :9090/ui）
│       └── images/                # 应用图标
├── cmd/
│   ├── main                       # 直接管理 mihomo 启停
│   ├── install_callback           # 安装后：生成配置 + 自动启动
│   ├── config_callback            # 配置变更后重启
│   ├── upgrade_init/callback      # 升级前备份/恢复配置
│   └── uninstall_init/callback    # 卸载清理
├── config/
│   ├── privilege                  # 权限配置（run-as: package）
│   └── resource                   # data-share 共享目录
├── wizard/
│   ├── install                    # 安装向导
│   ├── config                     # 配置向导
│   └── uninstall                  # 卸载向导
└── manifest                       # 飞牛应用清单
```

## 架构说明

```
┌─────────────────────────────────────────┐
│  飞牛桌面 iframe → http://NAS:9090/ui   │
├─────────────────────────────────────────┤
│  mihomo (vernesong/mihomo Alpha-Smart)  │
│  ├─ 端口 9090: API + metacubexd 面板    │
│  ├─ 端口 7890/7891/7893: 代理服务       │
│  ├─ Smart 代理组: LightGBM 智能选路     │
│  └─ 配置: data-share/config.yaml        │
└─────────────────────────────────────────┘
```

mihomo 直接服务 API 和 metacubexd 面板，无需 Node.js 中间层。

## 安装与使用

### 从 GitHub Release 安装

1. 前往 [Releases](https://github.com/cliii-one/ClashLite/releases) 下载对应架构的 FPK
2. 在飞牛应用中心手动安装
3. 安装向导中配置密钥和订阅源地址
4. 安装完成后自动启动，点击桌面图标打开面板

### 连接信息

| 项目 | 值 |
|------|-----|
| 访问地址 | `http://NAS-IP:9090/ui` |
| 密钥 | 安装时设置（默认 `yyds666`） |

### 配置文件位置

```
/vol*/@appshare/clashlite/config.yaml
```

可通过飞牛文件管理器直接编辑，升级/重装不会丢失。

## ⚠️ 升级说明

| 操作 | 方式 |
|------|------|
| **内核升级** | ✅ 可直接在 metacubexd 面板操作（自动从 vernesong/mihomo 下载，保留 Smart 支持） |
| **面板升级** | ✅ 可直接在 metacubexd 面板操作 |

## 构建与发布

### 触发构建

```bash
git tag v2.0.0
git push origin v2.0.0
```

或在 GitHub Actions 页面手动触发。

### 版本号规则

| 触发方式 | 版本号 |
|---------|--------|
| 推送 tag `v3.0.0` | `3.0.0` |
| 手动输入版本号 | 输入的版本号 |
| 不填版本 | 读取 manifest 默认版本 |

### 构建流程

1. 下载 mihomo Alpha-Smart 内核
2. 下载 LightGBM Model
3. 下载 GeoIP/GeoSite/ASN 数据
4. 下载 metacubexd 前端
5. Python tarfile 打包 FPK
6. 创建 GitHub Release

## 关键文件说明

### cmd/main

直接管理 mihomo 进程：
- `copy_bundled()`：首次启动时复制预装的内核 + 模型 + GeoData
- `copy_ui()`：复制 metacubexd 到 mihomo ui 目录
- `ensure_config()`：确保配置文件存在（从模板生成）
- `start_mihomo` / `stop_mihomo`：直接启停 mihomo 二进制

### mihomo 配置模板 (default.yaml)

基于 YYDS/OneSmart 精简版，使用占位符：
- `MB_SECRET` → API 密钥
- `AIRPORT_URL1` → 优质订阅源
- `AIRPORT_URL2` → 备用订阅源

安装时由 `install_callback` 替换为用户输入值。

### 应用权限 (config/privilege)

```json
{
    "defaults": { "run-as": "package" },
    "username": "clashlite",
    "groupname": "clashlite"
}
```

## 维护指南

| 更新项 | 方式 |
|--------|------|
| mihomo 内核 | 修改 build.yml 中的 `MIHOMO_TAG` |
| metacubexd | 构建时自动从 gh-pages 下载最新版 |
| 默认配置 | 编辑 `app/server/config/default.yaml` |

