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
