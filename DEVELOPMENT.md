# EES Demo — 开发文档

部署和使用说明请参见 [docs/DEPLOY.md](docs/DEPLOY.md)。

---

## 技术栈

| 模块 | 技术 | 说明 |
|------|------|------|
| 开发语言 | Go 1.24+ | 与未来 V1 保持一致 |
| 目标平台 | Windows 10/11/Server 2019+ | x86_64 (amd64) |
| Windows Service | `golang.org/x/sys/windows/svc` | SCM 协议、Install/Uninstall/Start/Stop |
| Explorer 右键菜单 | `golang.org/x/sys/windows/registry` | HKEY_CLASSES_ROOT\exefile\shell |
| IPC | Windows Named Pipe | `\\.\pipe\ees`，消息模式 |
| 哈希算法 | `crypto/sha256` | 标准库，零依赖 |
| 数字签名 | Windows Authenticode API | WinVerifyTrust + CryptQueryObject + CertGetNameString |
| 提权接口 | Windows API | WTSQueryUserToken → DuplicateTokenEx → CreateProcessAsUser |
| 配置文件 | JSON | whitelist.json + config.json |
| 日志 | Go 标准 `log` 包 | 分级输出到文件 |
| 构建 | Go 交叉编译 | WSL2 → Windows (CGO_ENABLED=0) |

---

## 架构总览

```
┌─────────────────┐       ┌──────────────────────────────────────────┐
│  Explorer        │       │  Windows Agent                          │
│  Context Menu    │       │  (NT AUTHORITY\SYSTEM — Windows Service)│
│  "Run with EA"   │       │                                          │
│       │          │       │  ┌──────────────────────────────────┐   │
│       ▼          │       │  │  PipeServer                      │   │
│  ees-client.exe  │──────▶│  │  ┌────────────────────────────┐  │   │
│  - Registry Menu │ Pipe  │  │  │  processRequest()          │  │   │
│  - Pipe Client   │◀──────│  │  │  ├─ verifyFile()           │  │   │
│  - MessageBox    │       │  │  │  │  ├ sha256File()         │  │   │
└─────────────────┘       │  │  │  │  └ getPublisher()        │  │   │
                          │  │  │  ├─ loadWhitelist()         │  │   │
                          │  │  │  │  └ decide()              │  │   │
                          │  │  │  ├─ ElevationEngine.Launch()│  │   │
                          │  │  │  │  ├ WTSQueryUserToken     │  │   │
                          │  │  │  │  ├ CreateProcessAsUser   │  │   │
                          │  │  │  │  └ ExitCode              │  │   │
                          │  │  │  └─ Response (Allow/Deny)   │  │   │
                          │  │  └────────────────────────────┘  │   │
                          │  └──────────────────────────────────┘   │
                          └──────────────────────────────────────────┘
```

完整流程：

```
右键 .exe → "Run with Enterprise Admin"
  → Named Pipe IPC
  → SHA256 哈希计算
  → Authenticode 数字签名验证
  → 白名单匹配
  → ✅ Allow → 立即响应 Client → 后台提权 → 安装程序启动
  → 🚫 Deny → 弹窗提示
```

---

## 模块说明

### `agent/` — Windows Agent

| 文件 | 职责 | 关键依赖 |
|------|------|---------|
| `main.go` | CLI 入口：install / uninstall / debug | `golang.org/x/sys/windows/svc` |
| `service.go` | svc.Handler 实现，SCM 生命周期管理 | `windows/svc`, `windows/svc/mgr` |
| `pipe_server.go` | Named Pipe Server 循环 + Request 处理 | `golang.org/x/sys/windows` |
| `elevate.go` | ElevationEngine — 提权链封装 | `WTSQueryUserToken`, `CreateProcessAsUser` |
| `verify.go` | SHA256 哈希 + Authenticode Publisher 提取 | `crypto/sha256`, `WinVerifyTrust`, `CryptQueryObject` |
| `whitelist.go` | 白名单加载 + SHA256/Publisher 双重匹配 | 纯 Go |
| `paths.go` | 可执行文件路径解析 | `os.Executable` |

### `client/` — Explorer Client

| 文件 | 职责 | 关键依赖 |
|------|------|---------|
| `main.go` | CLI 入口：install / uninstall / `<path>` | — |
| `menu.go` | Registry 右键菜单注册/卸载 | `golang.org/x/sys/windows/registry` |
| `pipe.go` | Named Pipe 客户端连接/读写 | `golang.org/x/sys/windows` |
| `prompt.go` | MessageBox 结果展示 | `user32.dll/MessageBoxW` |

### `common/` — 公共库

| 包 | 职责 |
|----|------|
| `common/config` | Config 结构体、JSON 加载、校验、默认值 |
| `common/log` | INFO/WARN/ERROR 三级文件日志 |
| `common/constants` | PipeName、错误码、Result 常量 |
| `common/types` | Request / Response 结构体 |

---

## 关键技术决策

### 1. 为什么用 `CreateProcessAsUser` 而不是其他方案？

`CreateProcessAsUser` 是从 Windows Service（SYSTEM）启动用户桌面进程的标准方案。核心流程：

```
WTSGetActiveConsoleSessionId()           → 找到当前登录用户
WTSQueryUserToken(sessionID, &token)     → 获取用户 Token
DuplicateTokenEx(token → primary)        → 转为 Primary Token
GetTokenInformation(LinkedToken=19)      → 提升为管理员权限（UAC）
CreateEnvironmentBlock()                 → 构建用户环境变量
CreateProcessAsUser(token, path, env)    → 启动进程（winsta0\default）
WaitForSingleObject()                    → 等待退出
GetExitCodeProcess()                     → 获取退出码
```

### 2. 为什么 `ees-client.exe` 有黑色窗口一闪？

原来设计是控制台程序，右键菜单调用后会短暂显示 CMD 窗口。解决方案：Agent 在处理 Allow 后立即响应，提权在后台 goroutine 执行，CMD 窗口在 MessageBox 弹出后立即关闭。

### 3. Authenticode 签名验证策略

`getPublisher()` 的策略是**先提取 Publisher 名称，再验证签名链**：
1. `CryptQueryObject` 解码 PKCS7 签名 → 获取证书商店
2. 枚举所有证书，跳过 CA 证书（名称含 PCA / Root / CA）
3. `CertGetNameString` 提取发布者名称
4. `WinVerifyTrust` 验证签名有效性（非致命——即使证书过期也能拿到 Publisher 名称）

### 4. 白名单热加载

白名单每次请求时重新从磁盘加载。修改 `whitelist.json` 后立即生效，无需重启 Service。

### 5. Named Pipe 安全性

`CreateNamedPipe` 使用 NULL DACL（通过 `InitializeSecurityDescriptor` + `SetSecurityDescriptorDacl`）允许标准用户连接。这是 Demo 原型阶段的选择——生产环境应配置细粒度 ACL。

### 6. Exit code 解读

提权完成后记录的 exit code 来自安装程序本身：

| Exit Code | 含义 |
|-----------|------|
| 0 | 安装完成 ✅ |
| 1 | 用户中途取消或安装失败 |
| 2 | 参数错误或文件不存在 |
| 1602 | Windows Installer：用户取消 |
| 1603 | Windows Installer：致命错误 |

---

## 构建与测试

```bash
# 构建所有 Windows 目标
for target in agent client; do
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
    go build -o "build/ees-$target.exe" "./$target"
done

# 单独构建提权预研工具
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -o build/ees-elevation.exe ./research/elevation

# 运行测试（Linux 下只跑 common/）
go test ./common/...

# 构建发行包
mkdir -p dist/EES/config
cp build/ees-agent.exe dist/EES/
cp build/ees-client.exe dist/EES/
cp config/whitelist.json dist/EES/config/
cp scripts/install.cmd dist/EES/
cp scripts/uninstall.cmd dist/EES/
```

从 WSL2 交叉编译到 Windows 时，使用 `CGO_ENABLED=0` 避免 MinGW 依赖。

---

## 风险项（已验证）

| 风险 | 验证结果 |
|------|---------|
| Windows Service → 管理员进程 | ✅ 已验证（Win11 24H2） |
| Explorer ↔ Service 通信 | ✅ Named Pipe 稳定 |
| Authenticode 签名读取 | ✅ WinVerifyTrust + CryptQueryObject |
| SHA256 一致性 | ✅ crypto/sha256 |
| UAC 兼容性 | ✅ Linked Token 获取成功 |
| Session 隔离 | ✅ 进程在用户桌面，非 Session 0 |

---

## 已知限制（→ V1 改进方向）

| 限制 | 说明 | V1 建议 |
|------|------|---------|
| 无签名链验证 | 仅验证签名是否存在，不检查吊销 | 集成 CRL/OCSP 检查 |
| 无审计数据库 | 日志只写入文件 | 改用结构化日志 + 审计数据库 |
| 无 Web 控制台 | 白名单手动编辑 JSON | Web Console 集中管理策略 |
| 无 AD 集成 | 用户/计算机策略本地管理 | AD 组策略分发 |
| 单实例 Pipe | 一次处理一个请求 | 多线程 Pipe Server |
| 无 Policy Server | 策略完全本地化 | 中心化策略分发 |

---

## 开发原则

1. **优先验证技术可行性，而非功能完整性。**
2. **最小可运行实现（Minimum Viable Implementation），避免过度设计。**
3. **本地配置（JSON）代替服务端和数据库。**
4. **每个阶段产出可运行、可演示的成果。**
5. **超出 Demo 验证目标的需求延后至 V1。**

---

## 文件清单（29 个源文件）

```
agent/elevate.go
agent/main.go
agent/paths.go
agent/pipe_server.go
agent/service.go
agent/verify.go
agent/whitelist.go
client/main.go
client/menu.go
client/pipe.go
client/prompt.go
common/config/config.go
common/config/config_test.go
common/constants/constants.go
common/log/log.go
common/log/log_test.go
common/types/types.go
common/types/types_test.go
config/config.json
config/whitelist.json
docs/DEPLOY.md
research/elevation/elevate.go
research/elevation/main.go
research/elevation/README.md
scripts/install.cmd
scripts/uninstall.cmd
go.mod
CLAUDE.md
```
