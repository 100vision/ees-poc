# EES V1 产品路线图

> 基于 PRD《Vibe coding - 企业自助软件安装平台（极简版）》

## V1 目标

从 Demo 原型演进为 **企业应用白名单提权系统**，实现集中策略管理 + 审计日志 + Web 管理后台。

### V1 包含范围

| 功能 | Demo | V1 |
|------|------|----|
| Windows Agent | ✅ 已有 | 升级为从 Server 获取策略 |
| Explorer 右键菜单 | ✅ 已有 | 不变 |
| SHA256 + Publisher 校验 | ✅ 已有 | 不变 |
| 自动提权执行 | ✅ 已有 | 不变 |
| 白名单管理 | ❌ 本地 JSON 编辑 | Web 后台在线管理 |
| 审计日志 | ❌ 本地文件 | 集中数据库存储 + 查询 |
| Web 管理后台 | ❌ 无 | 新增 |

### 非 V1 范围

不包含以下内容（来自 PRD 明确界定）：

- ❌ Software Center / 软件中心
- ❌ 软件仓库（Repository）
- ❌ 软件下载安装
- ❌ 软件升级 / 卸载
- ❌ AD 用户组同步
- ❌ 审批流程
- ❌ 临时管理员权限
- ❌ PowerShell / CMD 提权

---

## 架构变化

```
Demo (已完成)                           V1 (目标)
┌──────────────┐                    ┌──────────────────┐
│ ees-agent    │                    │ ees-agent        │
│ ees-client   │                    │ ees-client       │
│ ──────────── │                    │ ─────────────── │
│ 本地 whitelist│    →              │ HTTPS ↘          │
│ 本地日志      │                    │        ↗         │
└──────────────┘                    └───────┬──────────┘
                                            │
                                    ┌───────▼──────────┐
                                    │ ESC Server       │
                                    │ Go + PostgreSQL  │
                                    │ ├─ 白名单管理 API │
                                    │ ├─ 审计日志 API   │
                                    │ └─ Web 管理后台   │
                                    └──────────────────┘
```

**用户入口不变**：仍是 Explorer 右键菜单（`Run with Enterprise Admin`）。
**核心变化**：策略和日志从本地文件改为 Server 集中管理。

---

## 阶段划分

### Phase 1：Server 基础设施（3 周）

| 任务 | 工作量 | 说明 |
|------|--------|------|
| ESC Server 项目初始化 | 1d | Go 项目目录、配置、日志 |
| PostgreSQL + 数据库迁移 | 2d | 约 5 张表：whitelist_entries, audit_logs, users, agents |
| Agent Token 认证 | 1d | Server 端 Token 验证 |
| API 框架（路由 + HTTPS + 中间件） | 2d | REST + JSON |
| 管理员身份认证 | 1d | 管理员登录 API |
| **阶段输出** | | Server 可启动、API 可调用 |

### Phase 2：Agent 接入 Server（3 周）

| 任务 | 工作量 | 说明 |
|------|--------|------|
| Agent 注册流程 | 1d | 首次运行时向 Server 注册并获取 Token |
| Agent 获取白名单策略 | 2d | 替代本地 whitelist.json，Server 返回策略 |
| Agent 上报提权日志 | 1.5d | 每次提权完成后 POST 到 Server |
| Agent 心跳 | 1d | 定时上报状态 |
| 本地缓存降级 | 1d | Server 不可达时使用上次缓存的策略 |
| **阶段输出** | | Agent 接入 Server 管理，策略可在线更新 |

### Phase 3：Web 管理后台（3 周）

| 任务 | 工作量 | 说明 |
|------|--------|------|
| 项目初始化 + 登录页 | 1.5d | React/Vue 项目 + 管理员登录 |
| 仪表盘 | 1d | 统计数据：Agent 数、今日提权次数、成功率 |
| 白名单管理 | 3d | 列表、新增、编辑、删除、启用/禁用 |
| 提权日志查询 | 2d | 按用户/主机/时间/结果筛选 |
| **阶段输出** | | 管理员可通过 Web 完成全部运维操作 |

### Phase 4：集成与部署（1 周）

| 任务 | 工作量 | 说明 |
|------|--------|------|
| Agent 部署包制作 | 1d | Agent + config 打包 |
| Server 部署文档 | 1d | Docker / 手动部署说明 |
| 端到端集成测试 | 2d | 右键菜单 → Agent → Server 全链路 |
| **阶段输出** | | 可部署到企业测试环境 |

---

## 工作量汇总

| 阶段 | 人周 | 说明 |
|------|------|------|
| Phase 1: Server 基础设施 | 3w | Go + PostgreSQL + API 框架 |
| Phase 2: Agent 接入 Server | 3w | 策略远程化、日志上传、心跳 |
| Phase 3: Web 管理后台 | 3w | 白名单管理 + 审计日志查询 |
| Phase 4: 集成与部署 | 1w | 打包、文档、端到端验证 |
| 缓冲 | 1w | |
| **总计** | **11w** | **约 3 个月（单人）** |

---

## 技术栈

| 组件 | 技术 |
|------|------|
| Server | Go 1.24+（与 Demo 一致） |
| 数据库 | PostgreSQL |
| Web 后台 | React / Vue（选熟悉的即可） |
| API | REST + JSON + HTTPS |
| Agent ↔ Server | HTTPS |
| Agent（客户端程序） | 已有的 Go Agent 改造 |

---

## 关键设计约定

1. **Agent 是唯一具有提权能力的组件** — 沿用 Demo 设计
2. **用户入口不变** — 仍为右键菜单
3. **V1 不做软件中心** — 用户通过右键菜单发起提权
4. **V1 不做 AD 集成** — 白名单全局生效
5. **Server 采用单体架构** — 一人团队维护成本最低
