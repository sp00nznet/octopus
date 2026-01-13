#Requires -Version 5.1
<#
.SYNOPSIS
    Octopus Startup Script for Windows

.DESCRIPTION
    Builds (if needed) and starts the Octopus server on port 5005

.EXAMPLE
    .\start.ps1
    Start the Octopus server on port 5005
#>

# Configuration
$Port = 5005
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Binary = Join-Path $ScriptDir "octopus.exe"
$ServerDir = Join-Path $ScriptDir "server"
$DataDir = Join-Path $ScriptDir "data"

# Print banner
$banner = @"

    ____        __
   / __ \______/ /_____  ____  __  _______
  / / / / ___/ __/ __ \/ __ \/ / / / ___/
 / /_/ / /__/ /_/ /_/ / /_/ / /_/ (__  )
 \____/\___/\__/\____/ .___/\__,_/____/
                    /_/

"@
Write-Host $banner -ForegroundColor Cyan

# Check if Go is installed
function Test-GoInstalled {
    return $null -ne (Get-Command "go" -ErrorAction SilentlyContinue)
}

# Check if binary needs to be built
function Test-NeedsBuild {
    if (-not (Test-Path $Binary)) {
        return $true
    }

    # Check if any Go source files are newer than the binary
    $binaryTime = (Get-Item $Binary).LastWriteTime
    $newerFiles = Get-ChildItem -Path $ServerDir -Filter "*.go" -Recurse |
        Where-Object { $_.LastWriteTime -gt $binaryTime }

    return $null -ne $newerFiles -and $newerFiles.Count -gt 0
}

# Build the application
function Build-Application {
    if (-not (Test-GoInstalled)) {
        Write-Host "[ERROR] Go is not installed. Please run .\scripts\install.ps1 first." -ForegroundColor Red
        exit 1
    }

    Push-Location $ServerDir

    Write-Host "[INFO] Downloading dependencies..." -ForegroundColor Cyan
    go mod tidy

    if ($LASTEXITCODE -ne 0) {
        Write-Host "[ERROR] Failed to download dependencies." -ForegroundColor Red
        Pop-Location
        exit 1
    }

    Write-Host "[INFO] Building Octopus..." -ForegroundColor Cyan
    $env:CGO_ENABLED = "1"
    go build -o $Binary .\cmd\main.go

    if ($LASTEXITCODE -ne 0) {
        Write-Host "[ERROR] Build failed." -ForegroundColor Red
        Pop-Location
        exit 1
    }

    Pop-Location
    Write-Host "[SUCCESS] Build complete." -ForegroundColor Green
}

# Create data directory
function Initialize-DataDir {
    if (-not (Test-Path $DataDir)) {
        New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
    }
}

# Start the server
function Start-Server {
    Write-Host "[INFO] Starting Octopus on port $Port..." -ForegroundColor Green
    Write-Host "[INFO] Web UI: http://localhost:$Port" -ForegroundColor Cyan
    Write-Host "[INFO] Default credentials: admin / admin" -ForegroundColor Cyan
    Write-Host ""

    $env:PORT = $Port
    if (-not $env:DATABASE_PATH) {
        $env:DATABASE_PATH = Join-Path $DataDir "octopus.db"
    }

    & $Binary
}

# Main
if (Test-NeedsBuild) {
    if (-not (Test-Path $Binary)) {
        Write-Host "[INFO] Binary not found. Building..." -ForegroundColor Cyan
    } else {
        Write-Host "[INFO] Source files changed. Rebuilding..." -ForegroundColor Cyan
    }
    Build-Application
}

Initialize-DataDir
Start-Server
