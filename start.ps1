#Requires -Version 5.1
<#
.SYNOPSIS
    Octopus Startup Script for Windows

.DESCRIPTION
    Builds (if needed) and starts the Octopus server on port 5005
    Automatically installs Go 1.21+ if not present or outdated

.EXAMPLE
    .\start.ps1
    Start the Octopus server on port 5005
#>

# Configuration
$Port = 5005
$GoVersion = "1.21.13"
$GoMinVersion = [version]"1.21"
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

# Get current Go version
function Get-GoVersion {
    $goCmd = Get-Command "go" -ErrorAction SilentlyContinue
    if ($null -eq $goCmd) {
        return $null
    }

    $versionOutput = & go version 2>$null
    if ($versionOutput -match 'go(\d+\.\d+(\.\d+)?)') {
        return [version]$Matches[1]
    }
    return $null
}

# Install Go for Windows
function Install-Go {
    $currentVersion = Get-GoVersion

    if ($null -ne $currentVersion -and $currentVersion -ge $GoMinVersion) {
        Write-Host "[OK] Go $currentVersion is installed (>= $GoMinVersion required)" -ForegroundColor Green
        return
    }

    if ($null -eq $currentVersion) {
        Write-Host "[INFO] Go is not installed. Installing Go $GoVersion..." -ForegroundColor Yellow
    } else {
        Write-Host "[INFO] Go $currentVersion is too old. Installing Go $GoVersion..." -ForegroundColor Yellow
    }

    # Detect architecture
    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

    $goZip = "go$GoVersion.windows-$arch.zip"
    $goUrl = "https://go.dev/dl/$goZip"
    $goInstallPath = "C:\Go"

    Write-Host "[INFO] Downloading Go $GoVersion for windows/$arch..." -ForegroundColor Cyan

    # Create temp directory
    $tempDir = Join-Path $env:TEMP "go-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        $zipPath = Join-Path $tempDir $goZip

        # Download Go
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $webClient = New-Object System.Net.WebClient
        $webClient.DownloadFile($goUrl, $zipPath)

        Write-Host "[INFO] Installing Go to $goInstallPath..." -ForegroundColor Cyan

        # Remove existing installation
        if (Test-Path $goInstallPath) {
            Remove-Item -Path $goInstallPath -Recurse -Force
        }

        # Extract to C:\
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, "C:\")

        # Add to PATH for current session
        $env:PATH = "$goInstallPath\bin;$env:PATH"

        # Add to system PATH permanently
        $currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
        if ($currentPath -notlike "*$goInstallPath\bin*") {
            Write-Host "[INFO] Adding Go to system PATH..." -ForegroundColor Cyan
            [Environment]::SetEnvironmentVariable("PATH", "$goInstallPath\bin;$currentPath", "Machine")
        }

        # Verify installation
        $newVersion = Get-GoVersion
        if ($null -ne $newVersion) {
            Write-Host "[SUCCESS] Go $newVersion installed successfully" -ForegroundColor Green
        } else {
            Write-Host "[ERROR] Go installation verification failed" -ForegroundColor Red
            exit 1
        }
    }
    catch {
        Write-Host "[ERROR] Failed to install Go: $_" -ForegroundColor Red
        exit 1
    }
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
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
Install-Go

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
