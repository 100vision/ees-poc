# Elevation Pre-Research (Phase 0)

## Purpose

Independently verify the Windows elevation chain **before** integrating it into the Agent service. This separates "does the API work?" from "does the service framework work?".

## API Chain Under Test

```
WTSGetActiveConsoleSessionId   → Get active session (logged-in user)
WTSQueryUserToken              → Get impersonation token for that session
DuplicateTokenEx               → Convert to primary token (for CreateProcessAsUser)
  └─ GetTokenInformation (TokenLinkedToken, class 19)  → Get elevated (admin) token
CreateEnvironmentBlock         → Build user environment
CreateProcessAsUser            → Launch process as the user (lpDesktop="winsta0\default")
  └─ WaitForSingleObject       → Wait for exit
  └─ GetExitCodeProcess        → Capture exit code
```

## Build

```bash
# From project root:
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/ees-elevation.exe ./research/elevation/
```

## Usage

```cmd
REM List active terminal sessions
elevation.exe -list

REM Launch notepad.exe as the active console user (run from SYSTEM)
elevation.exe -session

REM Launch a specific program
elevation.exe -session -path C:\Temp\ChromeSetup.exe
```

## Testing Matrix

| Scenario | How | Expected | Result |
|----------|-----|----------|--------|
| SYSTEM context | `psexec -s -i cmd.exe` → `elevation.exe -session` | Process launches on user desktop | ✅ Pass (Win11 24H2) |
| UAC elevated admin token | Via `GetTokenInformation(LinkedToken=19)` | Elevated token obtained | ✅ Obtained |
| Bare executable name | `notepad.exe` (no path) | Not found from SYSTEM context | ❌ Needs full path |
| Launched process user | Via Task Manager | Runs as logged-in user, not SYSTEM | ✅ Verified |
| Other installers | `-path C:\Temp\ChromeSetup.exe` | Installer launches under user | ✅ Verified |

## Key Findings

Verified 2026-07-21 on Windows 11 24H2, launched from SYSTEM via `psexec -s -i`.

- [x] WTSGetActiveConsoleSessionId returns correct session ID (returns 1)
- [x] WTSQueryUserToken succeeds from SYSTEM context
- [x] DuplicateTokenEx produces a usable primary token
- [x] GetLinkedToken works (TokenLinkedToken=19 returns elevated admin token)
- [x] CreateEnvironmentBlock succeeds
- [x] CreateProcessAsUser launches process on user desktop (winsta0\default)
- [x] Process runs as the logged-in user (not SYSTEM, not Session 0)
- [x] Exit code captured correctly

### Critical Technical Notes (for Phase 5)

1. **WTSGetActiveConsoleSessionId** must be loaded from **kernel32.dll**, NOT wtsapi32.dll
   - Win11 24H2 removed this API from wtsapi32.dll
   - Use `golang.org/x/sys/windows.WTSGetActiveConsoleSessionId()` — it already loads from kernel32 ✅
2. **Always pass full executable paths** — bare names (e.g. `notepad.exe`) won't resolve because the environment block doesn't inherit SYSTEM's PATH
   - Use `C:\Windows\System32\notepad.exe`
   - Or resolve via the user's environment block
3. **Execution user identity**: The launched process runs as the active console user, NOT SYSTEM
   - This is the desired behavior — standard user can install approved software
4. **Elevated token**: `GetTokenInformation(LinkedToken=19)` successfully elevates from filtered UAC token to full admin token when the user is an administrator

## Integration Points (for Phase 5)

The final `elevation.go` in the `agent/` package should:

1. Wrap this chain into `type ElevationEngine struct { ... }`
2. Call `elevation.Launch(path string) (uint32, error)` from the Agent's Named Pipe handler
3. Log each step (using `common/log`)
4. Handle errors gracefully and return structured responses

See `research/elevation/elevate.go` for the reference implementation.
