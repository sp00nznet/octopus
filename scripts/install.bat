@echo off
REM Octopus Installation Script for Windows
REM This batch file launches the PowerShell installation script

echo.
echo  ====================================================
echo    Octopus - VM Migration Tool - Windows Installer
echo  ====================================================
echo.

REM Check if PowerShell is available
where powershell >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] PowerShell is not available on this system.
    echo Please install PowerShell and try again.
    pause
    exit /b 1
)

REM Get script directory
set SCRIPT_DIR=%~dp0

REM Check for arguments
set ARGS=
if "%1"=="--dev" set ARGS=-Dev
if "%1"=="-dev" set ARGS=-Dev
if "%1"=="--docker-only" set ARGS=-DockerOnly
if "%1"=="--skip-docker" set ARGS=-SkipDocker
if "%1"=="--help" set ARGS=-Help
if "%1"=="-h" set ARGS=-Help

REM Run PowerShell script with appropriate execution policy
echo Starting PowerShell installation script...
echo.
powershell -ExecutionPolicy Bypass -File "%SCRIPT_DIR%install.ps1" %ARGS%

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Installation failed with error code: %ERRORLEVEL%
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo Installation complete!
pause
