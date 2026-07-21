# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

EES Demo (Enterprise Elevation Service) — a technical prototype that verifies whether a standard Windows user can safely launch enterprise-approved software installers **without needing an administrator account or password**.

The core flow: Explorer right-click → Named Pipe IPC → Windows Service → SHA256 + Publisher verification → Create elevated process.

**Key constraint**: This is a *technical prototype*, not a product. Every feature must be evaluated against the verification goals. Anything outside scope is deferred to V1.

## Design Doc

See `/mnt/d/tlin/my_Obsidian_Notes/Obsidian/Obisidian/Notes/Vibe coding - 企业自助软件安装平台（极简版） EES Demo 验证版设计说明.md` for the full design specification.

## Tech Stack

| Module | Technology |
|--------|-----------|
| Language | Go 1.24+ (keep consistent with V1) |
| OS | Windows 10 / 11 / Server 2019+ |
| Windows Service | Go + Windows Service API (`golang.org/x/sys/windows/svc`) |
| Explorer Client | Go + COM / Registry |
| IPC | Windows Named Pipe (`golang.org/x/sys/windows`) |
| Config | JSON |
| Logging | Go standard `log` package (switch to Zap for V1) |
| Whitelist | Local JSON file |
| Hash | SHA-256 |
| Digital Signature | Windows Authenticode API |
| Elevation | Windows API (final approach TBD by validation) |

### Build

```bash
# Cross-compile all Windows targets from WSL2
for target in agent client research/elevation; do
  name=$(basename $target)
  out="ees-$name"
  [ "$name" = "elevation" ] && out="ees-elevation"
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "build/$out.exe" ./$target
done

# Build a single Windows target
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/ees-agent.exe ./agent
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/ees-client.exe ./client

# Linux packages only (common/ is cross-platform)
go build ./common/...
```

Note: `go build ./...` on Linux will skip Windows-only packages (`agent/`, `client/`, `research/elevation/`). Build them individually with `GOOS=windows`.

### Test

```bash
# Run all tests (Linux-compatible packages)
go test ./...

# Run tests for a specific package
go test ./common/...

# Run single test
go test -run TestName ./common/config

# Verbose
go test -v ./...
```

### Common operations

```bash
# Tidy dependencies (requires network for golang.org/x/sys)
go mod tidy

# Format code
go fmt ./...

# Vet
go vet ./...
```

## Architecture

```
Explorer Context Menu ("Run with Enterprise Admin")
         │
         ▼
 Explorer Client (client.exe)
    ├── Get target program path (from right-click context)
    ├── Named Pipe Client → \\.\pipe\ees
    └── Show user prompt result (message box)
         │
     Named Pipe (IPC)
         │
         ▼
 Windows Agent (agent.exe) — Windows Service
    ├── Named Pipe Server (listens on \\.\pipe\ees)
    ├── Verification Engine
    │   ├── Load whitelist.json
    │   ├── SHA256 hash check
    │   └── Publisher (Authenticode digital signature) check
    ├── Elevation Engine
    │   └── CreateProcessAsUser (or final selected approach)
    └── Logging → logs\agent.log
```

All policy comes from local JSON files (`config.json`, `whitelist.json`) — no server, no database, no REST API, no web console in scope.

## Project Structure

```
ees-demo/
├── agent/              # Windows Service (cmd entry point)
│   └── main.go         #   (placeholder — Phase 3)
├── client/             # Explorer Client (cmd entry point)
│   └── main.go         #   (placeholder — Phase 2)
├── common/             # Shared Go package ✅ (Phase 1 complete)
│   ├── config/         #   Config loading, defaults, validation
│   ├── log/            #   INFO/WARN/ERROR file logger
│   ├── constants/      #   PipeName, error codes, result values
│   └── types/          #   Request/Response structs
├── research/           # Technical pre-research
│   └── elevation/      #   Phase 0: Elevation chain prototype ⚡
│       ├── main.go     #   CLI entry: -session, -path, -list
│       ├── elevate.go  #   API chain (WTSQueryUserToken → CreateProcessAsUser)
│       └── README.md   #   Test results log
├── config/             # Deployed config files
│   ├── config.json     #   PipeName, Whitelist path, LogPath
│   └── whitelist.json  #   SHA256 + Publisher entries
├── build/              # Build output (gitignored)
│   └── elevation.exe   #   Cross-compiled Phase 0 binary
├── logs/               # Runtime log output (gitignored)
├── scripts/            # Install / uninstall batch scripts (Phase 6)
├── docs/               # Documentation (Phase 6)
├── go.mod
└── CLAUDE.md
```

Tests live inside each package (`*_test.go` alongside source), following Go conventions.

## Development Status

| Phase | Status | Description |
|-------|--------|-------------|
| 0 | ✅ Verified on Win11 24H2 | Elevation chain — `research/elevation/` |
| 1 | ✅ Done | Project init — `common/` packages, go.mod, tests |
| 2 | ✅ Done | Explorer Client — right-click menu, Named Pipe client |
| 3 | ✅ Done | Windows Agent — Service, Named Pipe server |
| 4 | ✅ Done | Verification Engine — SHA256, Authenticode |
| 5 | ✅ Done | Elevation Engine — integrate Phase 0 into Agent |
| 6 | ✅ Done | Install & Demo — scripts, docs |
| 7 | ✅ Done | Verification & Acceptance — all scenarios verified |

## Development Phases

| # | Phase | Key Deliverables |
|---|-------|------------------|
| 0 | Elevation Pre-Research | Standalone CLI verifying `WTSQueryUserToken → CreateProcessAsUser` chain |
| 1 | Project Init | go.mod, common package (config, log, constants, types), directory structure |
| 2 | Explorer Client | Context menu registration, path retrieval, Named Pipe Client, user prompts |
| 3 | Windows Agent | Windows Service (install/uninstall/start/stop), Named Pipe Server, request handling |
| 4 | Verification Engine | Whitelist loading, SHA256 verification, Publisher (Authenticode) verification |
| 5 | Elevation Engine | Integrate Phase 0 into Agent, session check, error handling |
| 6 | Install & Demo | install.cmd, uninstall.cmd, demo whitelist, DemoGuide.md |
| 7 | Verification & Acceptance | Full functional and edge-case validation |

## Development Principles

1. **Verify technical feasibility first**, not feature completeness.
2. **Minimum Viable Implementation** — avoid over-engineering.
3. **Local config over server/database** — JSON is enough for the prototype.
4. **Each phase produces a runnable, demonstrable result** — no big-bang integration.
5. **Stay in scope** — features outside the verification goals are deferred to V1.

## Communication Protocol

**Request** (Client → Agent via Named Pipe):
```json
{ "Path": "C:\\Temp\\ChromeSetup.exe" }
```

**Response** (Agent → Client via Named Pipe):
```json
{ "Result": "Allow", "Message": "Elevation Successful" }
```

Result values: `Allow` | `Deny` | `Error`

## Whitelist Schema (whitelist.json)

Each entry is identified by SHA256 hash and Publisher (digital signature subject). Both must match for Allow.

## Verification Log Format

```
Verify Start
SHA256 OK
Publisher OK
Allow
```

## Acceptance Checklist

- [ ] Explorer context menu "Run with Enterprise Admin" appears for .exe files only
- [ ] Context menu is correctly registered and removable
- [ ] Named Pipe IPC between Client and Agent is stable (multiple connections, timeouts)
- [x] SHA256 verification matches whitelist; modified files are rejected (Phase 4 implemented)
- [x] Publisher (Authenticode digital signature) verification works (Phase 4 implemented)
- [x] Non-whitelisted programs are denied (Phase 4 implemented)
- [x] Elevation Engine integrated into Agent (Phase 5)
- [x] Elevated process appears on the user's desktop (not Session 0) (Phase 0 verified)
- [ ] Agent installs/runs as a Windows Service (manageable via services.msc)
- [ ] Agent logs key runtime events (INFO/WARN/ERROR) to `logs\agent.log`
- [ ] Demo can be deployed via install.cmd and cleaned up via uninstall.cmd

## Risk Verification

| Risk | Demo Validation | Status |
|------|----------------|--------|
| Windows Service creating elevated processes | CreateProcessAsUser from SYSTEM → user desktop | ✅ Verified (Win11 24H2) |
| Explorer ↔ Service communication | Named Pipe reliability | ⬜ Phase 3 |
| Authenticode digital signature reading | Publisher extraction via WinVerifyTrust + CryptQueryObject | ✅ Verified (Win11 24H2) |
| SHA256 consistency | Hash computation across different installers | ✅ Implemented |
| UAC compatibility | Elevation behavior under standard user | ✅ Verified (linked token works) |
| Session isolation | Process appears on user desktop, not Session 0 | ✅ Verified |
