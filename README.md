# New API

**原作者 / Original Author: MICT**

## 项目简介

New API 是一个基于 Go 语言开发的高性能 API 网关与管理平台，提供多模型接入、负载均衡、计费管理等功能。

## 技术栈

- **后端**: Go
- **前端**: React 19 + TypeScript + Tailwind CSS + Rsbuild
- **数据库**: 支持 MySQL、PostgreSQL、SQLite 等多种数据库
- **认证**: OAuth 2.0 集成

## 功能特性

- 多模型 API 接入与管理
- 渠道管理与智能负载均衡
- 令牌管理与计费系统
- 用户管理与权限控制
- 日志记录与监控
- 国际化支持 (i18n)

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

请参阅 LICENSE 文件。

## 贡献

欢迎提交 Issue 和 Pull Request。

---

**原作者 / Original Author: MICT**