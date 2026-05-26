# EasyTier

基于 [EasyTier](https://github.com/EasyTier/EasyTier) 的飞牛 fnOS 去中心化 VPN 组网应用，集成 Web 控制台管理，支持 P2P 打洞、零配置组网。

## 功能特性

- **Web 控制台管理**：内置 easytier-web-embed 前端，通过浏览器配置网络
- **P2P 打洞**：支持 NAT 穿透，节点直连
- **去中心化组网**：无需中心服务器，节点间自动发现
- **加密通信**：支持网络加密，保障数据安全
- **多架构支持**：X86_64 / ARM64 双架构自动构建

## 项目结构

```
.
├── .github/workflows/build-easytier.yml  # GitHub Actions 双架构构建
├── app/
│   ├── server/data/easytier/             # EasyTier 二进制存放目录
│   └── ui/
│       ├── config                        # 飞牛桌面入口（iframe 模式）
│       └── images/                       # 应用图标
├── cmd/
│   ├── main                              # 生命周期管理（core + web 双进程）
│   ├── install_callback                  # 安装后：创建目录
│   ├── config_callback                   # 配置变更后重启
│   └── uninstall_callback                # 卸载清理
├── config/
│   ├── privilege                         # 权限配置（run-as: root，TUN 需要）
│   └── resource                          # data-share 共享目录
├── wizard/
│   ├── install                           # 安装向导（欢迎提示）
│   └── uninstall                         # 卸载向导（数据保留选项）
└── manifest                              # 飞牛应用清单
```

## 架构说明

```
┌─────────────────────────────────────────────────────────┐
│  浏览器 → fnOS 网关(5666) → EasyTier Web (11211)       │
│                                    ↓                    │
│                    easytier-web-embed                    │
│                    - Web 控制台 (端口 11211)             │
│                    - 配置服务 (UDP 127.0.0.1:22020)     │
│                                    ↓                    │
│                    easytier-core                         │
│                    - TUN 虚拟网卡（需 root）             │
│                    - P2P 打洞 / 中继                     │
│                    - 加密通信                            │
│                    - 状态持久化 ($TRIM_PKGVAR)           │
└─────────────────────────────────────────────────────────┘
```

## 安装与使用

### 从 GitHub Release 安装

1. 前往 [Releases](https://github.com/cliii-one/FnDepot/releases) 下载对应架构的 FPK
2. 在飞牛应用中心手动安装
3. 安装完成后自动启动，点击桌面图标打开控制台

### 登录信息

| 项目 | 值 |
|------|-----|
| 访问地址 | 飞牛桌面图标或 `http://NAS-IP:5666/app/easytier` |
| 初始账号 | `admin` |
| 初始密码 | `admin` |

> 首次登录后请及时修改密码。

### 配置方式

安装时不预设任何网络参数，启动后打开 Web 控制台，根据实际需求自行配置：
- 网络名称与密钥
- 虚拟 IP 地址
- 公共服务器地址
- 加密选项

## 构建与发布

### 触发构建

在 GitHub Actions 页面手动触发 "构建 EasyTier" workflow。

### 版本号规则

| 触发方式 | 版本号 |
|---------|--------|
| 推送 `easytier-v*` 标签 | 标签中的版本号 |
| 手动输入版本号 | 输入的版本号 |
| 不填版本 | 读取 manifest 默认版本 |

### 构建流程

1. 从 EasyTier/EasyTier 获取最新 Release 版本
2. 下载对应架构的 easytier-core + easytier-web-embed + easytier-cli
3. 更新 manifest 版本号和平台
4. 打包 FPK
5. 创建 GitHub Release

## 关键文件说明

### cmd/main

管理 easytier-core + easytier-web-embed 双进程：
- `copy_bundled()`：从安装目录复制二进制到可写目录（已存在则跳过）
- `start_core()`：启动 easytier-core，仅开启配置服务端口 `-w udp://127.0.0.1:22020/admin`
- `start_web()`：启动 easytier-web-embed，监听 11211 端口
- `stop_by_pid_file()`：通用停止函数，PID 文件优先，pgrep 兜底
- `wait_for_pid()`：通用启动等待函数

### 应用权限 (config/privilege)

```json
{
    "defaults": { "run-as": "root" },
    "username": "easytier",
    "groupname": "easytier"
}
```

使用 root 权限运行，因为 easytier-core 的 TUN 模式需要创建虚拟网卡。

## 维护指南

| 更新项 | 方式 |
|--------|------|
| EasyTier 版本 | 构建时自动从 GitHub Release 获取最新版 |
| Web 端口 | 修改 `app/ui/config` 中的 `port` 和 `cmd/main` 中的 `--api-server-port` |
| 配置服务端口 | 修改 `cmd/main` 中的 `-w` 和 `--config-server-port` |
