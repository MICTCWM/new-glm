# New API

🍥 **Next-Generation LLM Gateway and AI Asset Management System**

## 原项目 / Original Project

本项目 Fork 自 [QuantumNous/new-api](https://github.com/QuantumNous/new-api)

上游项目：[One API](https://github.com/songquanpeng/one-api) (MIT License)

---

## 项目简介

New API 是一个基于 Go 语言开发的高性能 LLM 网关与 AI 资产管理系统，支持将各种 LLM 模型转换为 OpenAI 兼容、Claude 兼容或 Gemini 兼容格式，提供多模型接入、负载均衡、计费管理等功能。

## 技术栈

- **后端**: Go
- **前端**: React 19 + TypeScript + Tailwind CSS + Rsbuild
- **数据库**: 支持 MySQL、PostgreSQL、SQLite 等多种数据库
- **认证**: OAuth 2.0 集成 (Discord, Telegram, OIDC 等)

## 功能特性

### 核心功能
- 多模型 API 接入与管理
- 渠道管理与智能负载均衡
- 令牌管理与计费系统
- 用户管理与权限控制
- 日志记录与监控
- 国际化支持 (i18n)

### 支持的 API 格式
- OpenAI Chat Completions
- OpenAI Responses
- OpenAI Realtime API
- Claude Messages
- Google Gemini
- Rerank (Cohere, Jina)

### 高级特性
- ⚖️ 渠道加权随机
- 🔄 失败自动重试
- 🚦 用户级模型限流
- 🔄 OpenAI ⇄ Claude 格式转换
- 🔄 OpenAI ⇄ Gemini 格式转换

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+ / Bun
- 数据库 (MySQL / PostgreSQL / SQLite)

### 安装与运行

```bash
# 克隆仓库
git clone https://github.com/MICTCWM/new-glm.git
cd new-glm

# 后端构建与运行
go build -o new-api
./new-api

# 前端开发
cd web/default
bun install
bun run dev
```

### Docker 部署

```bash
# 使用 SQLite
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

## 项目结构

```
.
├── common/         # 公共工具与常量
├── controller/     # 控制器层
├── model/          # 数据模型层
├── relay/          # API 中继与转发
├── router/         # 路由配置
├── middleware/     # 中间件
├── web/            # 前端项目
│   ├── default/    # 默认前端 (React + TypeScript)
│   └── classic/    # 经典前端
└── docs/           # 文档
```

## 开发规范

前端开发规范详见 `web/default/AGENTS.md`，包含：

- 国际化 (i18n) 规范
- TypeScript 类型规范
- React 组件开发规范
- 状态管理 (Zustand)
- API 请求 (React Query + Axios)
- 表单处理 (React Hook Form + Zod)
- 路由 (TanStack Router)
- 样式 (Tailwind CSS)
- 测试与部署

## 许可证

本项目采用 [GNU Affero General Public License v3.0 (AGPLv3)](LICENSE) 许可证。

根据 AGPLv3 第 7 条的附加条款：
- 修改版本必须保留原作者归属声明
- 修改版本必须保留指向原项目的可见链接：[https://github.com/QuantumNous/new-api](https://github.com/QuantumNous/new-api)

## 致谢

- 感谢 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 提供的原项目
- 感谢 [One API](https://github.com/songquanpeng/one-api) (MIT License) 提供的基础框架
- 感谢 JetBrains 提供的开源开发许可证

## 贡献

欢迎提交 Issue 和 Pull Request。

---

**原项目地址 / Original Repository**: [https://github.com/QuantumNous/new-api](https://github.com/QuantumNous/new-api)