# EES Demo 部署指南

## 系统要求

| 组件 | 要求 |
|------|------|
| OS | Windows 10 / 11 / Server 2019+ |
| 架构 | x64 (amd64) |
| 管理员权限 | 安装时需要 |
| 用户账户 | 任何类型（管理员或标准用户均可使用） |

## 交付文件

| 文件 | 用途 |
|------|------|
| `ees-agent.exe` | Windows Service — Pipe 服务端、验证、提权 |
| `ees-client.exe` | Explorer 右键菜单 — 注册、Pipe 客户端、弹窗 |
| `config/whitelist.json` | 白名单配置 |
| `install.cmd` | 一键安装脚本 |
| `uninstall.cmd` | 一键卸载脚本 |

## 安装

### 一键安装

```cmd
1. 将 dist/EES/ 文件夹复制到桌面
2. 右键 install.cmd → "以管理员身份运行"
3. 安装完成后，右键任意 .exe 即可看到 "Run with Enterprise Admin"
```

### 手动安装

```cmd
REM 1. 复制文件到固定位置（如桌面或 Program Files）
REM 2. 安装服务（需要管理员）
ees-agent.exe install

REM 3. 启动服务
net start EESAgent

REM 4. 注册右键菜单（需要管理员）
ees-client.exe install

REM 5. 验证服务运行中
sc query EESAgent
```

## 配置白名单

编辑 `config\whitelist.json`，每个条目支持按发布者或文件哈希匹配：

```json
{
  "SHA256": "6745fa76...",
  "Publisher": "Google LLC",
  "Description": "Google Chrome Installer",
  "Enabled": true
}
```

匹配规则：
- **SHA256**— 精确匹配（空则跳过哈希检查）
- **Publisher**— 精确匹配证书签名者（空则跳过发布者检查）
- 两者均需通过才 Allow（除非字段为空）

### 预置条目

| 程序 | 匹配方式 |
|------|----------|
| Google Chrome Installer | Publisher: `Google LLC` |
| Visual Studio Code Installer | Publisher: `Microsoft Corporation` |
| 7-Zip (签名版) | Publisher: `Igor Pavlov` |
| 7-Zip (v26.02 x64 无签名) | SHA256 hash |

### 添加新程序

```cmd
REM 1. 获取 SHA256（PowerShell）
Get-FileHash "C:\path\to\setup.exe" -Algorithm SHA256

REM 2. 查看数字签名
Get-AuthenticodeSignature "C:\path\to\setup.exe"

REM 3. 添加条目到 whitelist.json
REM 修改后立即生效，无需重启服务
```

## 演示场景

### 场景 1：白名单程序 ✅

```cmd
1. 右键 ChromeSetup.exe → "Run with Enterprise Admin"
2. 弹窗 "Elevation Successful"
3. Chrome 安装程序启动
```

### 场景 2：非白名单程序 🚫

```cmd
1. 右键一个不在白名单的 .exe
2. 弹窗 "Application Not Approved"
```

### 场景 3：服务未运行 ❌

```cmd
1. net stop EESAgent
2. 右键任意 .exe
3. 弹窗连接错误
```

## 日志

路径：`%ProgramFiles%\EES\logs\agent.log`

格式示例：

```
2026-07-21 14:57:31.813 [INFO] Verify Start: D:\setup.exe
2026-07-21 14:57:31.817 [INFO] SHA256: 6745fa76dc2ea031596d8678f6f6b99c...
2026-07-21 14:57:31.818 [INFO] Publisher: Google LLC
2026-07-21 14:57:31.818 [INFO] Allow: D:\setup.exe
2026-07-21 14:57:31.818 [INFO] Elevation start: D:\setup.exe
2026-07-21 14:57:31.818 [INFO]   Session ID: 1
2026-07-21 14:57:31.818 [INFO]   User token obtained
2026-07-21 14:57:31.818 [INFO]   Elevated (linked) token obtained
2026-07-21 14:57:31.920 [INFO] Elevation complete (exit code: 0) — installer completed successfully
```

## 卸载

```cmd
右键 uninstall.cmd → "以管理员身份运行"
```

自动完成：停止服务 → 删除服务 → 删除右键菜单 → 清理文件。

手动：

```cmd
net stop EESAgent
ees-agent.exe uninstall
ees-client.exe uninstall
```

## 排错

| 现象 | 原因 | 解决 |
|------|------|------|
| "Access is denied" | Pipe 安全策略 | 安装最新 ees-agent.exe |
| 服务无法启动 | 缺少 config/whitelist.json | 确保 config\ 目录在 exe 旁 |
| "WinVerifyTrust" 警告 | 签名证书过期 | 仍会提取 Publisher（看日志确认）|
| 提权无反应 | Session 0 隔离 | 确认用户在控制台登录 |
| Exit code 非零 | 安装程序本身退出码 | 提权成功了，退出码来自安装程序 |
