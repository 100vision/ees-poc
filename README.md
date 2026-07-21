# EES Demo — Enterprise Elevation Service

**让普通用户无需管理员密码即可安装企业批准的软件。**

EES Demo 是一个技术验证原型（Technical Prototype），验证 Windows 提权链路是否可行——从右键菜单到 Named Pipe 通信、SHA256/Authenticode 验证、再到提权执行安装程序。

## 快速开始

### 系统要求

- Windows 10 / 11 / Server 2019+（x64）
- 管理员权限（仅安装时需要）

### 一键安装

```cmd
1. 将 dist/EES/ 文件夹复制到桌面
2. 右键 install.cmd → "以管理员身份运行"
3. 安装完成后，右键任意 .exe 即可看到 "Run with Enterprise Admin"
```

### 快速演示

```cmd
1. 找一个 ChromeSetup.exe 或 VSCodeSetup.exe（在白名单中的程序）
2. 右键 → "Run with Enterprise Admin"
3. 安装程序直接启动，无需管理员密码 ✨
```

## 完整流程

```
右键 .exe → "Run with Enterprise Admin"
  → Named Pipe IPC
  → SHA256 哈希计算
  → Authenticode 数字签名验证
  → 白名单匹配
  → ✅ 允许 → 提权 → 安装程序启动
  → 🚫 拒绝 → 弹窗提示
```

所有操作记录在 `%ProgramFiles%\EES\logs\agent.log`。

## 白名单配置

编辑 `config\whitelist.json`，支持按发布者（Publisher）或文件哈希（SHA256）匹配：

```json
{
  "entries": [
    { "SHA256": "",        "Publisher": "Google LLC",           "Description": "Chrome",    "Enabled": true },
    { "SHA256": "",        "Publisher": "Microsoft Corporation","Description": "VSCode",    "Enabled": true },
    { "SHA256": "6745fa..","Publisher": "",                    "Description": "7-Zip",     "Enabled": true },
  ]
}
```

修改后立即生效，无需重启服务。

## 卸载

```cmd
右键 uninstall.cmd → "以管理员身份运行"
```

清除全部：停止服务 → 卸载服务 → 删除右键菜单 → 清理文件。

## 设计目标

| 目标 | 说明 |
|------|------|
| 🎯 验证 Windows 提权链路 | 核心：普通用户能否无需管理员密码运行批准程序 |
| 🔬 技术原型 | 非正式产品，验证通过后作为 V1 技术基线 |
| 📦 最小可运行 | 每个阶段产出一个可演示的成果 |
| 🚫 无后端依赖 | 无数据库、无 REST API、无 Web Console |

## 项目资源

| 文档 | 说明 |
|------|------|
| `DemoGuide.md` | 安装、配置、演示流程 |
| `CLAUDE.md` | 代码库结构、开发命令、架构说明 |
