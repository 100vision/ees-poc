@echo off
setlocal enabledelayedexpansion

title EES Demo — Uninstaller

set "INSTALL_DIR=%ProgramFiles%\EES"
set "SERVICE_NAME=EESAgent"

echo ============================================
echo  EES — Enterprise Elevation Service
echo  Demo Uninstaller
echo ============================================
echo.

REM Check admin rights
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo ERROR: This script must be run as Administrator.
    echo Right-click uninstall.cmd and select "Run as administrator".
    pause
    exit /b 1
)

REM Stop the service
echo Stopping service...
net stop %SERVICE_NAME% 2>nul

REM Uninstall the service
echo.
echo Removing Windows Service...
if exist "%INSTALL_DIR%\ees-agent.exe" (
    "%INSTALL_DIR%\ees-agent.exe" uninstall
)

REM Unregister context menu
echo.
echo Removing Explorer context menu...
if exist "%INSTALL_DIR%\ees-client.exe" (
    "%INSTALL_DIR%\ees-client.exe" uninstall
)

REM Clean up files
echo.
echo Cleaning up files...
if exist "%INSTALL_DIR%" (
    rmdir /S /Q "%INSTALL_DIR%"
    echo Removed: %INSTALL_DIR%
)

echo.
echo ============================================
echo  Uninstall Complete
echo ============================================
echo  EES Demo has been removed from this system.
echo ====================================
echo.
pause
