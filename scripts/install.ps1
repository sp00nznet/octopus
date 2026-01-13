#Requires -Version 5.1
<#
.SYNOPSIS
    Octopus Installation Script for Windows

.DESCRIPTION
    This script installs all dependencies and builds Octopus VM Migration Tool.
    It will install Go, Docker Desktop, and all required dependencies.

.PARAMETER Dev
    Install development dependencies (air, golangci-lint)

.PARAMETER DockerOnly
    Only install Docker dependencies and build Docker image

.PARAMETER SkipDocker
    Skip Docker installation

.PARAMETER Help
    Show this help message

.EXAMPLE
    .\install.ps1
    Basic installation with all components

.EXAMPLE
    .\install.ps1 -Dev
    Installation with development tools

.EXAMPLE
    .\install.ps1 -SkipDocker
    Installation without Docker

.NOTES
    Author: Octopus Team
    Version: 1.0
#>

param(
    [switch]$Dev,
    [switch]$DockerOnly,
    [switch]$SkipDocker,
    [switch]$Help
)

# Configuration
$GoVersion = "1.21.6"
$MinGoVersion = "1.21"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$InstallDir = Split-Path -Parent $ScriptDir

# Colors
$Colors = @{
    Red     = "Red"
    Green   = "Green"
    Yellow  = "Yellow"
    Blue    = "Cyan"
    Purple  = "Magenta"
    Cyan    = "Cyan"
    White   = "White"
}

# Logging functions
function Write-LogInfo {
    param([string]$Message)
    Write-Host "[INFO] " -ForegroundColor $Colors.Blue -NoNewline
    Write-Host $Message
}

function Write-LogSuccess {
    param([string]$Message)
    Write-Host "[SUCCESS] " -ForegroundColor $Colors.Green -NoNewline
    Write-Host $Message
}

function Write-LogWarning {
    param([string]$Message)
    Write-Host "[WARNING] " -ForegroundColor $Colors.Yellow -NoNewline
    Write-Host $Message
}

function Write-LogError {
    param([string]$Message)
    Write-Host "[ERROR] " -ForegroundColor $Colors.Red -NoNewline
    Write-Host $Message
}

function Write-LogStep {
    param([string]$Message)
    Write-Host ""
    Write-Host ("=" * 60) -ForegroundColor $Colors.Purple
    Write-Host "  $Message" -ForegroundColor $Colors.Cyan
    Write-Host ("=" * 60) -ForegroundColor $Colors.Purple
    Write-Host ""
}

# Print banner
function Show-Banner {
    $banner = @"

    ____        __
   / __ \______/ /_____  ____  __  _______
  / / / / ___/ __/ __ \/ __ \/ / / / ___/
 / /_/ / /__/ /_/ /_/ / /_/ / /_/ (__  )
 \____/\___/\__/\____/ .___/\__,_/____/
                    /_/

    VM Migration & Disaster Recovery Tool

"@
    Write-Host $banner -ForegroundColor $Colors.Cyan
    Write-Host "Installation Script v1.0 for Windows" -ForegroundColor White
    Write-Host ""
}

# Check if running as administrator
function Test-Administrator {
    $currentUser = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentUser.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Check if command exists
function Test-Command {
    param([string]$Command)
    return $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

# Install Chocolatey
function Install-Chocolatey {
    Write-LogStep "Installing Chocolatey Package Manager"

    if (Test-Command "choco") {
        $chocoVersion = (choco --version)
        Write-LogInfo "Chocolatey is already installed: v$chocoVersion"
        return
    }

    Write-LogInfo "Installing Chocolatey..."
    Set-ExecutionPolicy Bypass -Scope Process -Force
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
    Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

    # Refresh environment
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

    Write-LogSuccess "Chocolatey installed successfully"
}

# Install Git
function Install-Git {
    Write-LogStep "Installing Git"

    if (Test-Command "git") {
        $gitVersion = (git --version) -replace "git version ", ""
        Write-LogInfo "Git is already installed: v$gitVersion"
        return
    }

    Write-LogInfo "Installing Git via Chocolatey..."
    choco install git -y --no-progress

    # Refresh environment
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

    Write-LogSuccess "Git installed successfully"
}

# Install Go
function Install-Go {
    Write-LogStep "Installing Go"

    if (Test-Command "go") {
        $currentVersion = (go version) -replace "go version go", "" -replace " windows/.*", ""
        Write-LogInfo "Go is already installed: v$currentVersion"

        # Check version
        if ([version]$currentVersion -ge [version]$MinGoVersion) {
            Write-LogSuccess "Go version is sufficient (>= $MinGoVersion)"
            return
        }
        Write-LogWarning "Go version is too old. Installing newer version..."
    }

    Write-LogInfo "Installing Go $GoVersion..."

    # Determine architecture
    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    $goMsi = "go$GoVersion.windows-$arch.msi"
    $goUrl = "https://go.dev/dl/$goMsi"
    $downloadPath = "$env:TEMP\$goMsi"

    Write-LogInfo "Downloading Go from $goUrl..."
    Invoke-WebRequest -Uri $goUrl -OutFile $downloadPath -UseBasicParsing

    Write-LogInfo "Installing Go..."
    Start-Process msiexec.exe -Wait -ArgumentList "/i `"$downloadPath`" /quiet /norestart"

    # Clean up
    Remove-Item $downloadPath -Force -ErrorAction SilentlyContinue

    # Update PATH
    $goPath = "C:\Program Files\Go\bin"
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*$goPath*") {
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$goPath", "Machine")
    }

    # Refresh environment
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

    Write-LogSuccess "Go $GoVersion installed successfully"
}

# Install Docker Desktop
function Install-Docker {
    Write-LogStep "Installing Docker Desktop"

    if (Test-Command "docker") {
        try {
            $dockerVersion = (docker --version) -replace "Docker version ", "" -replace ",.*", ""
            Write-LogInfo "Docker is already installed: v$dockerVersion"

            # Check if Docker is running
            $dockerInfo = docker info 2>&1
            if ($LASTEXITCODE -eq 0) {
                Write-LogSuccess "Docker is running"
                return
            }
            else {
                Write-LogWarning "Docker is installed but not running. Please start Docker Desktop."
            }
        }
        catch {
            Write-LogWarning "Docker command found but not working properly."
        }
    }

    # Check for Docker Desktop installation
    $dockerDesktopPath = "${env:ProgramFiles}\Docker\Docker\Docker Desktop.exe"
    if (Test-Path $dockerDesktopPath) {
        Write-LogInfo "Docker Desktop is installed. Starting..."
        Start-Process $dockerDesktopPath
        Write-LogWarning "Please wait for Docker Desktop to start, then run this script again."
        return
    }

    Write-LogInfo "Installing Docker Desktop via Chocolatey..."
    choco install docker-desktop -y --no-progress

    Write-LogSuccess "Docker Desktop installed"
    Write-LogWarning "Please start Docker Desktop and run this script again to complete setup."
    Write-LogWarning "You may need to log out and back in for changes to take effect."
}

# Install SQLite
function Install-SQLite {
    Write-LogStep "Installing SQLite"

    if (Test-Command "sqlite3") {
        Write-LogInfo "SQLite is already installed"
        return
    }

    Write-LogInfo "Installing SQLite via Chocolatey..."
    choco install sqlite -y --no-progress

    # Refresh environment
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

    Write-LogSuccess "SQLite installed successfully"
}

# Install GCC (for CGO)
function Install-GCC {
    Write-LogStep "Installing GCC (MinGW-w64)"

    if (Test-Command "gcc") {
        $gccVersion = (gcc --version | Select-Object -First 1) -replace ".*\) ", ""
        Write-LogInfo "GCC is already installed: $gccVersion"
        return
    }

    Write-LogInfo "Installing MinGW-w64 via Chocolatey..."
    choco install mingw -y --no-progress

    # Refresh environment
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

    Write-LogSuccess "GCC installed successfully"
}

# Install development tools
function Install-DevTools {
    Write-LogStep "Installing Development Tools"

    # Set GOPATH if not set
    if (-not $env:GOPATH) {
        $env:GOPATH = "$env:USERPROFILE\go"
    }

    # Add GOPATH/bin to PATH
    $goBin = "$env:GOPATH\bin"
    if ($env:Path -notlike "*$goBin*") {
        $env:Path = "$env:Path;$goBin"
    }

    # Install air
    Write-LogInfo "Installing air (hot-reload)..."
    go install github.com/cosmtrek/air@latest

    # Install golangci-lint
    Write-LogInfo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

    Write-LogSuccess "Development tools installed"
}

# Download Go dependencies
function Get-GoDependencies {
    Write-LogStep "Downloading Go Dependencies"

    Push-Location "$InstallDir\server"

    Write-LogInfo "Running go mod download..."
    go mod download

    Write-LogInfo "Running go mod tidy..."
    go mod tidy

    Pop-Location

    Write-LogSuccess "Go dependencies downloaded"
}

# Build the application
function Build-Application {
    Write-LogStep "Building Octopus"

    Push-Location $InstallDir

    Write-LogInfo "Building server binary..."
    Push-Location "server"

    $env:CGO_ENABLED = "1"
    go build -o "..\octopus.exe" .\cmd\main.go

    Pop-Location

    if (Test-Path "octopus.exe") {
        Write-LogSuccess "Build successful! Binary: $InstallDir\octopus.exe"
    }
    else {
        Write-LogError "Build failed"
        exit 1
    }

    Pop-Location
}

# Setup configuration
function Initialize-Configuration {
    Write-LogStep "Setting Up Configuration"

    $configDir = "$InstallDir\docker\config"
    $configFile = "$configDir\config.yaml"
    $configTemplate = "$configDir\config.yaml.example"

    if (-not (Test-Path $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }

    if (-not (Test-Path $configFile)) {
        if (Test-Path $configTemplate) {
            Write-LogInfo "Creating configuration file from template..."
            Copy-Item $configTemplate $configFile
            Write-LogSuccess "Configuration file created: docker\config\config.yaml"
            Write-LogWarning "Please edit this file with your settings before running Octopus"
        }
    }
    else {
        Write-LogInfo "Configuration file already exists"
    }
}

# Build Docker image
function Build-DockerImage {
    Write-LogStep "Building Docker Image"

    # Check if Docker is running
    $dockerInfo = docker info 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-LogWarning "Docker is not running. Please start Docker Desktop and run: docker build -t octopus:latest -f docker/Dockerfile.server ."
        return
    }

    Push-Location $InstallDir

    Write-LogInfo "Building Octopus Docker image..."
    docker build -t octopus:latest -f docker/Dockerfile.server .

    Pop-Location

    Write-LogSuccess "Docker image built: octopus:latest"
}

# Print completion message
function Show-Completion {
    Write-Host ""
    Write-Host ("=" * 60) -ForegroundColor Green
    Write-Host "  Installation Complete!" -ForegroundColor Green
    Write-Host ("=" * 60) -ForegroundColor Green
    Write-Host ""

    Write-Host "Quick Start:" -ForegroundColor White
    Write-Host "  1. Edit configuration: " -NoNewline
    Write-Host "notepad docker\config\config.yaml" -ForegroundColor Cyan
    Write-Host "  2. Run with Docker:    " -NoNewline
    Write-Host "docker-compose -f docker\docker-compose.yml up -d" -ForegroundColor Cyan
    Write-Host "  3. Or run locally:     " -NoNewline
    Write-Host ".\octopus.exe" -ForegroundColor Cyan
    Write-Host "  4. Access web UI:      " -NoNewline
    Write-Host "http://localhost:8080" -ForegroundColor Cyan
    Write-Host ""

    Write-Host "Default Credentials (development mode):" -ForegroundColor White
    Write-Host "  Username: " -NoNewline
    Write-Host "admin" -ForegroundColor Cyan
    Write-Host "  Password: " -NoNewline
    Write-Host "admin" -ForegroundColor Cyan
    Write-Host ""

    Write-Host "Useful Commands:" -ForegroundColor White
    Write-Host "  " -NoNewline
    Write-Host ".\octopus.exe" -ForegroundColor Cyan -NoNewline
    Write-Host "                    - Run the server"
    Write-Host "  " -NoNewline
    Write-Host "docker-compose up -d" -ForegroundColor Cyan -NoNewline
    Write-Host "            - Start with Docker"
    Write-Host "  " -NoNewline
    Write-Host "docker-compose logs -f" -ForegroundColor Cyan -NoNewline
    Write-Host "          - View logs"
    Write-Host ""

    if ($Dev) {
        Write-Host "Development Commands:" -ForegroundColor White
        Write-Host "  " -NoNewline
        Write-Host "air" -ForegroundColor Cyan -NoNewline
        Write-Host "                            - Run with hot-reload"
        Write-Host "  " -NoNewline
        Write-Host "go test ./..." -ForegroundColor Cyan -NoNewline
        Write-Host "                  - Run tests"
        Write-Host "  " -NoNewline
        Write-Host "golangci-lint run" -ForegroundColor Cyan -NoNewline
        Write-Host "              - Run linter"
        Write-Host ""
    }

    Write-Host "Documentation: " -NoNewline -ForegroundColor Magenta
    Write-Host "docs\README.md" -ForegroundColor White
    Write-Host "Issues: " -NoNewline -ForegroundColor Magenta
    Write-Host "https://github.com/sp00nznet/octopus/issues" -ForegroundColor White
    Write-Host ""
}

# Main installation flow
function Main {
    Show-Banner

    # Check if running as administrator
    if (-not (Test-Administrator)) {
        Write-LogWarning "This script requires administrator privileges for some operations."
        Write-LogWarning "Some features may not install correctly."
        Write-Host ""
        $continue = Read-Host "Continue anyway? (y/N)"
        if ($continue -ne "y" -and $continue -ne "Y") {
            Write-LogInfo "Please run this script as Administrator."
            Write-LogInfo "Right-click PowerShell and select 'Run as Administrator'"
            exit 0
        }
    }

    if ($Help) {
        Get-Help $MyInvocation.MyCommand.Path -Detailed
        exit 0
    }

    if ($DockerOnly) {
        Install-Chocolatey
        Install-Docker
        Build-DockerImage
        Show-Completion
        exit 0
    }

    Install-Chocolatey
    Install-Git
    Install-GCC
    Install-SQLite
    Install-Go

    if (-not $SkipDocker) {
        Install-Docker
    }

    if ($Dev) {
        Install-DevTools
    }

    Get-GoDependencies
    Build-Application
    Initialize-Configuration

    if (-not $SkipDocker) {
        Build-DockerImage
    }

    Show-Completion
}

# Run main
Main
