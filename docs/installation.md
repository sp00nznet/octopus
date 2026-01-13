# Installation Guide

This guide provides detailed instructions for installing Octopus on various platforms.

## Table of Contents

- [System Requirements](#system-requirements)
- [Quick Installation](#quick-installation)
- [Linux Installation](#linux-installation)
- [macOS Installation](#macos-installation)
- [Windows Installation](#windows-installation)
- [Docker Installation](#docker-installation)
- [Manual Installation](#manual-installation)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## System Requirements

### Minimum Requirements

| Component | Requirement |
|-----------|-------------|
| CPU | 2 cores |
| RAM | 4 GB |
| Disk | 10 GB free space |
| Network | Outbound internet access |

### Software Requirements

| Software | Version | Required For |
|----------|---------|--------------|
| Go | 1.21+ | Building from source |
| Docker | 20.10+ | Container deployment |
| Git | 2.0+ | Cloning repository |
| GCC/MinGW | Latest | CGO compilation |
| SQLite3 | 3.0+ | Database |

### Supported Operating Systems

| OS | Version | Architecture |
|----|---------|--------------|
| Ubuntu | 20.04+ | amd64, arm64 |
| Debian | 11+ | amd64, arm64 |
| CentOS/RHEL | 8+ | amd64, arm64 |
| Fedora | 35+ | amd64, arm64 |
| Alpine | 3.15+ | amd64, arm64 |
| macOS | 12+ | amd64, arm64 |
| Windows | 10/11, Server 2019+ | amd64 |

---

## Quick Installation

### Linux/macOS (One Command)

```bash
git clone https://github.com/sp00nznet/octopus.git && cd octopus && ./scripts/install.sh
```

### Windows (PowerShell as Administrator)

```powershell
git clone https://github.com/sp00nznet/octopus.git; cd octopus; .\scripts\install.ps1
```

---

## Linux Installation

### Using the Installation Script

The installation script automatically detects your Linux distribution and installs all dependencies.

```bash
# Clone the repository
git clone https://github.com/sp00nznet/octopus.git
cd octopus

# Make the script executable
chmod +x scripts/install.sh

# Run the installer
./scripts/install.sh
```

### Installation Options

| Option | Description |
|--------|-------------|
| `--dev` | Install development tools (air, golangci-lint) |
| `--docker-only` | Only install Docker and build the image |
| `--skip-docker` | Skip Docker installation |
| `--help` | Show help message |

### Examples

```bash
# Full installation with development tools
./scripts/install.sh --dev

# Only Docker setup
./scripts/install.sh --docker-only

# Build without Docker
./scripts/install.sh --skip-docker
```

### Distribution-Specific Notes

#### Ubuntu/Debian

```bash
# The script will use apt-get to install:
# - build-essential
# - git, curl, wget
# - sqlite3, libsqlite3-dev
# - ca-certificates, gnupg
```

#### CentOS/RHEL

```bash
# The script will use yum to install:
# - Development Tools group
# - git, curl, wget
# - sqlite, sqlite-devel
```

#### Alpine

```bash
# The script will use apk to install:
# - build-base
# - git, curl, wget
# - sqlite, sqlite-dev
```

---

## macOS Installation

### Prerequisites

1. **Install Xcode Command Line Tools**
   ```bash
   xcode-select --install
   ```

2. **Install Homebrew** (if not installed)
   ```bash
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   ```

### Installation

```bash
# Clone the repository
git clone https://github.com/sp00nznet/octopus.git
cd octopus

# Run the installer
./scripts/install.sh
```

### Docker Desktop

For Docker support on macOS, install Docker Desktop:
1. Download from [docker.com](https://www.docker.com/products/docker-desktop)
2. Install and start Docker Desktop
3. Run the installation script again

---

## Windows Installation

### Prerequisites

1. **Enable PowerShell script execution**
   ```powershell
   Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
   ```

2. **Run PowerShell as Administrator**
   - Right-click PowerShell
   - Select "Run as Administrator"

### Installation

```powershell
# Clone the repository
git clone https://github.com/sp00nznet/octopus.git
cd octopus

# Run the installer
.\scripts\install.ps1
```

### Alternative: Batch File

For users unfamiliar with PowerShell:

```cmd
# Run the batch file
scripts\install.bat
```

### Installation Options

| Option | Description |
|--------|-------------|
| `-Dev` | Install development tools |
| `-DockerOnly` | Only install Docker components |
| `-SkipDocker` | Skip Docker installation |
| `-Help` | Show help message |

### What Gets Installed

The Windows installer uses Chocolatey to install:
- Go 1.21
- Git
- MinGW-w64 (GCC for Windows)
- SQLite
- Docker Desktop

---

## Docker Installation

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/sp00nznet/octopus.git
cd octopus

# Start with Docker Compose
docker-compose -f docker/docker-compose.yml up -d

# View logs
docker-compose -f docker/docker-compose.yml logs -f
```

### Using Docker Directly

```bash
# Build the image
docker build -t octopus:latest -f docker/Dockerfile.server .

# Run the container
docker run -d \
  --name octopus \
  -p 8080:8080 \
  -v octopus-data:/data \
  -e JWT_SECRET=your-secret-here \
  octopus:latest
```

### Environment Variables

Pass environment variables to configure the container:

```bash
docker run -d \
  --name octopus \
  -p 8080:8080 \
  -v octopus-data:/data \
  -e PORT=8080 \
  -e DATABASE_PATH=/data/octopus.db \
  -e JWT_SECRET=your-secure-secret \
  -e AD_SERVER=ldap.example.com \
  -e AD_BASE_DN="DC=example,DC=com" \
  octopus:latest
```

---

## Manual Installation

If you prefer manual installation:

### 1. Install Go

```bash
# Download Go
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz

# Extract to /usr/local
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2. Install Dependencies

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y build-essential git sqlite3 libsqlite3-dev

# CentOS/RHEL
sudo yum groupinstall -y "Development Tools"
sudo yum install -y git sqlite sqlite-devel
```

### 3. Clone and Build

```bash
# Clone repository
git clone https://github.com/sp00nznet/octopus.git
cd octopus

# Download Go dependencies
cd server
go mod download
go mod tidy

# Build
CGO_ENABLED=1 go build -o ../octopus ./cmd/main.go
cd ..
```

### 4. Configure

```bash
# Create config directory
mkdir -p docker/config

# Copy example config
cp docker/config/config.yaml.example docker/config/config.yaml

# Edit configuration
vim docker/config/config.yaml
```

### 5. Run

```bash
# Set environment variables
export DATABASE_PATH=./data/octopus.db
export JWT_SECRET=your-secret-here

# Run the server
./octopus
```

---

## Verification

After installation, verify everything is working:

### 1. Check the Binary

```bash
./octopus --version
# or on Windows
.\octopus.exe --version
```

### 2. Test the API

```bash
# Health check
curl http://localhost:8080/api/v1/health
# Response: {"status":"healthy"}
```

### 3. Access the Web UI

Open your browser and navigate to:
```
http://localhost:8080
```

### 4. Test Login

Use the default credentials:
- Username: `admin`
- Password: `admin`

---

## Troubleshooting

### Common Issues

#### "Permission denied" on Linux/macOS

```bash
chmod +x scripts/install.sh
chmod +x octopus
```

#### CGO errors during build

Ensure GCC is installed:
```bash
# Ubuntu/Debian
sudo apt-get install build-essential

# macOS
xcode-select --install

# Windows - install MinGW via Chocolatey
choco install mingw
```

#### Docker not starting

```bash
# Linux
sudo systemctl start docker
sudo usermod -aG docker $USER
# Log out and back in

# macOS/Windows
# Start Docker Desktop application
```

#### Port 8080 already in use

```bash
# Change the port
export PORT=8081
./octopus

# Or in Docker
docker run -p 8081:8080 octopus:latest
```

### Getting Help

If you encounter issues:
1. Check the [Troubleshooting Guide](troubleshooting.md)
2. Search [GitHub Issues](https://github.com/sp00nznet/octopus/issues)
3. Open a new issue with:
   - Operating system and version
   - Installation method used
   - Complete error message
   - Steps to reproduce
