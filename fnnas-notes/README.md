<div align="center">
  <img src="docs/fnnas-notes.png" alt="贴贴密笺应用海报" width="100%" />

  <h1>贴贴密笺</h1>

  <p>
    <strong>为 FnNAS / 飞牛 OS 打造的轻量、安全、优雅的本地便签应用</strong>
  </p>

  <p>
    Markdown 编辑 · 分类标签 · 多视图看板 · AES-256-GCM 加密 · 版本历史 · 分享 · 回收站 · 本地存储
  </p>

  <p>
    <img src="https://img.shields.io/badge/version-1.0.2-5B8DEF?style=for-the-badge" alt="version" />
    <img src="https://img.shields.io/badge/FnNAS-1.1.31+-24C8DB?style=for-the-badge" alt="FnNAS" />
    <img src="https://img.shields.io/badge/Flutter-Web-46D1FD?style=for-the-badge&logo=flutter&logoColor=white" alt="Flutter Web" />
    <img src="https://img.shields.io/badge/Python-HTTP_Server-3776AB?style=for-the-badge&logo=python&logoColor=white" alt="Python" />
    <img src="https://img.shields.io/badge/SQLite-Local_Data-003B57?style=for-the-badge&logo=sqlite&logoColor=white" alt="SQLite" />
  </p>
</div>

---

## ✨ 为什么选择贴贴密笺

贴贴密笺不是一个复杂臃肿的知识库，而是一款专注于 **快速记录、清晰整理、安全保存** 的 NAS 本地便签工具。  
它运行在 FnNAS / 飞牛 OS 上，数据默认保存在本地 SQLite 数据库中，适合记录灵感、备忘、待办、教程片段、运维笔记和个人资料。

<table>
  <tr>
    <td width="33%">
      <h3>📝 专注记录</h3>
      <p>支持 Markdown 编辑与实时预览，既能快速输入，也能写出结构清晰的内容。</p>
    </td>
    <td width="33%">
      <h3>🔒 本地安全</h3>
      <p>支持 AES-256-GCM 内容加密，敏感便签可单独设置密码保护。</p>
    </td>
    <td width="33%">
      <h3>🧭 高效整理</h3>
      <p>分类、标签、搜索、网格、列表、看板多种方式组合使用，便签不再杂乱。</p>
    </td>
  </tr>
</table>

## 🚀 功能亮点

| 功能 | 说明 |
|---|---|
| 📝 Markdown 编辑 | 支持 Markdown 输入、格式工具栏、实时预览 |
| 🏷️ 分类与标签 | 分类管理、标签筛选、全局搜索，快速定位内容 |
| 🧩 多视图模式 | 网格、列表、看板视图自由切换 |
| 🔐 私密加密 | AES-256-GCM 加密便签内容，密码校验后查看 |
| 📜 版本历史 | 自动记录修改历史，支持版本对比与一键回滚 |
| ⏰ 提醒通知 | 为重要便签设置提醒，通知中心集中查看 |
| 📤 分享链接 | 生成便签分享链接，支持访问统计与有效期管理 |
| 🗑️ 回收站 | 软删除保护，误删内容可恢复 |
| 🎨 主题外观 | 支持深浅主题、配色与纹理风格 |
| 💾 本地存储 | SQLite 本地存储，数据默认不上传外部服务器 |

## 🏗️ 技术栈

<table>
  <tr>
    <td><strong>前端</strong></td>
    <td>Flutter Web · Provider · CanvasKit</td>
  </tr>
  <tr>
    <td><strong>后端</strong></td>
    <td>Python HTTP Server · FnNAS Unix Socket 网关</td>
  </tr>
  <tr>
    <td><strong>存储</strong></td>
    <td>SQLite · WAL 模式 · 本地数据目录</td>
  </tr>
  <tr>
    <td><strong>打包</strong></td>
    <td>FnNAS .fpk · GitHub Actions 自动构建</td>
  </tr>
</table>

## 📦 部署形态

贴贴密笺以 FnNAS `.fpk` 应用包发布，安装后通过飞牛桌面入口访问：

```text
/app/fnnas-notes
```

运行时由 FnNAS 统一网关转发到后端 Unix Socket，前端 Flutter Web 与后端 API 由同一 Python 服务托管。

```text
FnNAS 桌面 iframe
      │
      ▼
/app/fnnas-notes
      │
      ▼
FnNAS 统一网关
      │
      ▼
fnnas-notes.sock
      │
      ▼
Python server.py
```

## 🧑‍💻 本地开发

> 本地环境主要用于开发调试，生产行为以 FnNAS 安装后的运行方式为准。

### 环境要求

- Python 3
- Flutter SDK
- Dart SDK `>=3.0.0 <4.0.0`

### 启动开发服务

```bash
python dev_server.py
```

默认访问：

```text
http://localhost:8080
```

跳过前端构建或指定端口：

```bash
python dev_server.py --skip-build
python dev_server.py --port 8080
```

## 📚 文档

| 文档 | 内容 |
|---|---|
| [开发者文档](docs/DEVELOPER.md) | 项目结构、开发调试、核心模块说明 |
| [部署文档](docs/DEPLOYMENT.md) | FnNAS 打包、安装、升级、卸载和运行维护 |
| [API 文档](docs/API.md) | 后端接口、认证策略、请求响应约定 |
| [数据库文档](docs/DATABASE.md) | SQLite 表结构、字段说明和迁移信息 |

## 🗂️ 项目结构

```text
.
├── fnnas.notes/              # FnNAS 应用包目录
│   ├── app/                  # 打包运行时文件
│   ├── cmd/                  # 生命周期脚本
│   ├── config/               # 权限与资源配置
│   ├── wizard/               # 安装/卸载向导
│   └── manifest              # 应用元数据
├── src/
│   ├── backend/              # Python 后端服务
│   └── frontend/             # Flutter Web 前端
├── docs/                     # 项目文档与展示图
└── dev_server.py             # 本地开发启动脚本
```

## 🔐 隐私与安全

- 敏感便签支持 AES-256-GCM 加密。
- 加密便签不会在分享接口中直接暴露原文。
- TCP 分享端口只开放公开分享所需的最小接口。
- 主应用管理入口依赖 FnNAS 统一网关访问控制。
- 数据默认保存在本地 SQLite 数据库中。

## 🤝 参与贡献

欢迎提交 Issue 和 Pull Request。建议在修改前先阅读：

- [开发者文档](docs/DEVELOPER.md)
- [部署文档](docs/DEPLOYMENT.md)
- [API 文档](docs/API.md)

## 📄 License

本项目基于 [MIT License](LICENSE) 开源。

---

<div align="center">
  <strong>贴贴密笺 · 让 NAS 里的记录更清晰、更安全、更优雅</strong>
</div>
