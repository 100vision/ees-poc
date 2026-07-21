# EES Demo Guide

## Overview

Enterprise Elevation Service (EES) Demo — a technical prototype that allows standard Windows users to run approved software installers **without needing an administrator account or password**.

### Core Flow

```
User right-clicks .exe → "Run with Enterprise Admin"
  → Named Pipe IPC → EES Agent Service (SYSTEM)
  → SHA256 hash + Authenticode Publisher check
  → Whitelist match → CreateProcessAsUser
  → Installer launches on user's desktop (elevated)
```

## System Requirements

| Component | Requirement |
|-----------|-------------|
| OS | Windows 10 / 11 / Server 2019+ |
| Architecture | x64 (amd64) |
| Admin rights | Required for service installation |
| User account | Any (admin or standard) for launching installers |

## Files

| File | Purpose |
|------|---------|
| `ees-agent.exe` | Windows Service — Named Pipe server, verification, elevation |
| `ees-client.exe` | Explorer Context Menu — registry, pipe client, user prompts |
| `config/whitelist.json` | Allow-list of approved software |
| `install.cmd` | Automated installation script |
| `uninstall.cmd` | Automated removal script |

## Installation

### Quick Install (Recommended)

```cmd
REM 1. Run as Administrator
Right-click install.cmd → "Run as administrator"

REM 2. The script will:
REM    - Copy files to %ProgramFiles%\EES\
REM    - Install the Windows Service
REM    - Register Explorer context menu
REM    - Start the service
```

### Manual Install

```cmd
REM 1. Copy files to a permanent location (e.g. Desktop or Program Files)
REM 2. Install the service (Admin required)
ees-agent.exe install

REM 3. Start the service
net start EESAgent

REM 4. Register context menu (Admin required)
ees-client.exe install

REM 5. Verify service is running
sc query EESAgent
```

## Configuration

### Whitelist (`config/whitelist.json`)

The whitelist determines which programs can be elevated. Each entry has:

```json
{
  "SHA256": "6745fa76...",
  "Publisher": "Google LLC",
  "Description": "Google Chrome Installer",
  "Enabled": true
}
```

Matching logic:
- **SHA256** — exact match (if specified). Empty = skip hash check.
- **Publisher** — exact match of Authenticode signing certificate (if specified). Empty = skip publisher check.
- Both checks must pass for Allow (unless a field is empty).

### Pre-Configured Entries

| Program | Match By |
|---------|----------|
| Google Chrome Installer | Publisher: `Google LLC` |
| Visual Studio Code Installer | Publisher: `Microsoft Corporation` |
| 7-Zip (signed version) | Publisher: `Igor Pavlov` |
| 7-Zip (unsigned, v26.02 x64) | SHA256 hash only |

### Adding New Programs

```cmd
REM 1. Get the file's SHA256 hash (via PowerShell)
Get-FileHash "C:\path\to\setup.exe" -Algorithm SHA256

REM 2. Check if it has a digital signature (via PowerShell)
Get-AuthenticodeSignature "C:\path\to\setup.exe"

REM 3. Add an entry to whitelist.json
REM No need to restart — whitelist is hot-reloaded on each request
```

## Demo Walkthrough

### Scenario 1: Approved Program (Should Succeed)

```cmd
1. Right-click ChromeSetup.exe in Explorer
2. Select "Run with Enterprise Admin"
3. Expected: ✅ "Elevation Successful"
4. Chrome installer launches on your desktop
```

### Scenario 2: Unapproved Program (Should Be Denied)

```cmd
1. Right-click an unsigned .exe not in whitelist (e.g., random_setup.exe)
2. Select "Run with Enterprise Admin"
3. Expected: 🚫 "Application Not Approved"
```

### Scenario 3: Service Not Running (Error Handling)

```cmd
1. Stop the agent service: net stop EESAgent
2. Right-click any .exe
3. Select "Run with Enterprise Admin"
4. Expected: ❌ "Connection Error" dialog
```

## Logs

The agent writes detailed logs to:

```
%ProgramFiles%\EES\logs\agent.log
```

Log format:

```
2026-07-21 14:57:31.813 [INFO] Verify Start: D:\setup.exe
2026-07-21 14:57:31.817 [INFO] SHA256: 6745fa76dc2ea031596d8678f6f6b99c...
2026-07-21 14:57:31.818 [INFO] Publisher: Google LLC
2026-07-21 14:57:31.818 [INFO] Allow: D:\setup.exe
2026-07-21 14:57:31.818 [INFO] Elevation start: D:\setup.exe
2026-07-21 14:57:31.818 [INFO]   Session ID: 1
2026-07-21 14:57:31.818 [INFO]   User token obtained
2026-07-21 14:57:31.818 [INFO]   Elevated (linked) token obtained
2026-07-21 14:57:31.920 [INFO] Elevation complete (exit code: 0)
```

## Uninstallation

```cmd
REM Run as Administrator
Right-click uninstall.cmd → "Run as administrator"
```

The uninstaller will:
1. Stop the EESAgent service
2. Uninstall the service
3. Remove the Explorer context menu
4. Delete all EES files from `%ProgramFiles%\EES\`

### Manual Uninstall

```cmd
net stop EESAgent
ees-agent.exe uninstall
ees-client.exe uninstall
```

## Troubleshooting

| Problem | Likely Cause | Solution |
|---------|-------------|----------|
| "Access is denied" | Named Pipe security | Reinstall latest ees-agent.exe |
| Service won't start | Missing config/whitelist.json | Ensure config\ directory exists alongside ees-agent.exe |
| "WinVerifyTrust" warning | Expired certificate | Publisher still extracted (check logs for publisher name) |
| Elevation fails silently | Session 0 isolation | Verify user is logged in at the console |
| Exit code non-zero | Installer itself errored | The installer ran successfully; exit code is from the installer |

## Technical Architecture

```
┌─────────────────┐    ┌──────────────────────────────┐
│  Explorer        │    │  Windows Agent (SYSTEM)       │
│  Context Menu    │    │                              │
│  "Run with EA"   │    │  ┌────────────────────────┐  │
│       │          │    │  │ Named Pipe Server      │  │
│       ▼          │    │  │ \\.\pipe\ees            │  │
│  ees-client.exe  │───▶│  │                        │  │
│  - Pipe Client   │    │  ├────────────────────────┤  │
│  - MessageBox    │    │  │ Verification Engine    │  │
└─────────────────┘    │  │  - SHA256              │  │
                       │  │  - Authenticode        │  │
                       │  │  - Whitelist Match     │  │
                       │  ├────────────────────────┤  │
                       │  │ Elevation Engine       │  │
                       │  │  - WTSQueryUserToken   │  │
                       │  │  - CreateProcessAsUser │  │
                       │  │  - User Desktop        │  │
                       │  └────────────────────────┘  │
                       └──────────────────────────────┘
```
