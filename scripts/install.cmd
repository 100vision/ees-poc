@echo off
setlocal enabledelayedexpansion

title EES Demo - Installer

set "INSTALL_DIR=%ProgramFiles%\EES"
set "CONFIG_DIR=%INSTALL_DIR%\config"
set "SERVICE_NAME=EESAgent"

echo ============================================
echo  EES - Enterprise Elevation Service
echo  Demo Installer
echo ============================================
echo.

REM Check admin rights
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo ERROR: This script must be run as Administrator.
    echo Right-click install.cmd and select "Run as administrator".
    pause
    exit /b 1
)

REM Determine source directory
set "SCRIPT_DIR=%~dp0"

REM Try script directory first (dist package layout), fallback to ..\build\ (repo layout)
if exist "%SCRIPT_DIR%ees-agent.exe" (
    set "SRC_DIR=%SCRIPT_DIR%"
) else (
    if exist "%SCRIPT_DIR%..\build\ees-agent.exe" (
        set "SRC_DIR=%SCRIPT_DIR%..\"
    ) else (
        echo ERROR: Cannot find ees-agent.exe
        echo Looked in: %SCRIPT_DIR% and %SCRIPT_DIR%..\build\
        pause
        exit /b 1
    )
)

echo Installing to: %INSTALL_DIR%
echo Source: %SRC_DIR%
echo.

REM Create directories
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%CONFIG_DIR%" mkdir "%CONFIG_DIR%"
if not exist "%INSTALL_DIR%\logs" mkdir "%INSTALL_DIR%\logs"

REM Copy files
echo Copying files...
copy /Y "%SRC_DIR%ees-agent.exe"         "%INSTALL_DIR%\" >nul
copy /Y "%SRC_DIR%ees-client.exe"        "%INSTALL_DIR%\" >nul
copy /Y "%SRC_DIR%config\whitelist.json" "%CONFIG_DIR%\" >nul

if not exist "%INSTALL_DIR%\ees-agent.exe" (
    echo ERROR: Failed to copy ees-agent.exe
    pause
    exit /b 1
)
echo Files copied.

REM Install Windows Service
echo.
echo Installing Windows Service...
"%INSTALL_DIR%\ees-agent.exe" install
if %ERRORLEVEL% neq 0 (
    echo WARNING: Service may already exist. Attempting to continue...
)

REM Register Explorer context menu
echo.
echo Registering Explorer context menu...
"%INSTALL_DIR%\ees-client.exe" install
if %ERRORLEVEL% neq 0 (
    echo WARNING: Context menu registration failed.
)

REM Start the service
echo.
echo Starting service...
net start %SERVICE_NAME%
if %ERRORLEVEL% neq 0 (
    echo WARNING: Service did not start. Check the logs.
)

echo.
echo ============================================
echo  Installation Complete
echo ============================================
echo.
echo  Installed to: %INSTALL_DIR%
echo  Config:       %CONFIG_DIR%\whitelist.json
echo  Logs:         %INSTALL_DIR%\logs\agent.log
echo.
echo  Next steps:
echo   1. Right-click any .exe file in Explorer
echo   2. Select "Run with Enterprise Admin"
echo   3. If the program is in the whitelist, it will launch elevated
echo.
pause
