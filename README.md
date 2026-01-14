<p align="center">
  <img src="docs/images/octopus-logo.png" alt="Octopus Logo" width="200">
</p>

<h1 align="center">🐙 Octopus</h1>

<p align="center">
  <strong>Enterprise VM Migration & Disaster Recovery Platform</strong>
</p>

<p align="center">
  Seamlessly migrate virtual machines from VMware vCenter to AWS, GCP, Azure, or other vCenter instances
</p>

<p align="center">
  <a href="#-features">Features</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-documentation">Documentation</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="#-api">API</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0-blue.svg" alt="Version">
  <img src="https://img.shields.io/badge/go-%3E%3D1.21-00ADD8.svg" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey.svg" alt="Platform">
  <img src="https://img.shields.io/badge/docker-ready-2496ED.svg" alt="Docker">
</p>

---

## 📋 Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
- [Architecture](#-architecture)
- [Configuration](#-configuration)
- [Usage](#-usage)
- [API Reference](#-api-reference)
- [Documentation](#-documentation)
- [Contributing](#-contributing)
- [License](#-license)

---

## ✨ Features

<table>
<tr>
<td width="50%">

### 🌐 Multi-Cloud Migration
- **VMware to VMware** - Cross-datacenter migration
- **VMware to AWS** - EC2 instance creation
- **VMware to GCP** - Compute Engine migration
- **VMware to Azure** - Azure VM deployment

</td>
<td width="50%">

### 🛡️ Disaster Recovery
- **Continuous Sync** - CBT-based replication
- **Scheduled Cutover** - Planned failovers
- **Test Failover** - Non-destructive testing
- **Auto-Recovery** - Automated failback

</td>
</tr>
<tr>
<td width="50%">

### 🔧 Hardware Preservation
- **MAC Addresses** - Maintain network identity
- **Port Groups** - Preserve network config
- **Hardware Settings** - Retain VM specs
- **Guest OS** - Full compatibility

</td>
<td width="50%">

### 📊 Size Estimation
- **Cross-Platform Sizing** - Compare target sizes across VMware, AWS, GCP, Azure
- **VXRail/vSAN Support** - Accurate RAID overhead calculation (RAID-1, RAID-5, RAID-6)
- **Organic Factors** - Dedup/compression expansion, snapshot consolidation
- **Standalone Script** - `vxrail_size_estimator.py` for direct vCenter queries
- **Cost Estimation** - Cloud pricing estimates

</td>
</tr>
<tr>
<td width="50%">

### 🔐 Enterprise Security
- **Active Directory** - LDAP authentication
- **JWT Tokens** - Secure API access
- **Role-Based Access** - Admin/user separation
- **Audit Logging** - Full activity trail

</td>
<td width="50%">

### 🖥️ Modern Web Interface
- **Responsive Design** - Desktop & mobile
- **Real-time Updates** - Live progress tracking
- **Admin Portal** - Configuration management
- **Dashboard** - At-a-glance status

</td>
</tr>
</table>

---

## 🚀 Quick Start

### One-Line Installation

**Linux / macOS:**
```bash
git clone https://github.com/sp00nznet/octopus.git && cd octopus && ./scripts/install.sh
```

**Windows (PowerShell as Administrator):**
```powershell
git clone https://github.com/sp00nznet/octopus.git; cd octopus; .\scripts\install.ps1
```

### Using Docker (Fastest)

```bash
# Clone and start
git clone https://github.com/sp00nznet/octopus.git
cd octopus
make docker-run

# Access the web UI
open http://localhost:8080
```

### Default Credentials

| Username | Password | Role |
|----------|----------|------|
| `admin` | `admin` | Administrator |

> ⚠️ **Change these credentials in production!**

---

## 📦 Installation

### Prerequisites

| Requirement | Minimum Version | Notes |
|------------|-----------------|-------|
| Go | 1.21+ | For building from source |
| Docker | 20.10+ | For containerized deployment |
| Git | 2.0+ | For cloning the repository |

### Installation Scripts

The installation scripts automatically download and install all dependencies.

#### Linux / macOS

```bash
# Basic installation
./scripts/install.sh

# With development tools (air, golangci-lint)
./scripts/install.sh --dev

# Docker only (skip local build)
./scripts/install.sh --docker-only

# Skip Docker installation
./scripts/install.sh --skip-docker

# Show help
./scripts/install.sh --help
```

#### Windows

```powershell
# Basic installation (run as Administrator)
.\scripts\install.ps1

# With development tools
.\scripts\install.ps1 -Dev

# Docker only
.\scripts\install.ps1 -DockerOnly

# Skip Docker
.\scripts\install.ps1 -SkipDocker

# Or use the batch file
.\scripts\install.bat
```

### What Gets Installed

| Component | Linux | macOS | Windows |
|-----------|-------|-------|---------|
| Go 1.21 | ✅ | ✅ | ✅ |
| Docker | ✅ | ✅ | ✅ |
| Docker Compose | ✅ | ✅ | ✅ |
| Git | ✅ | ✅ | ✅ |
| SQLite | ✅ | ✅ | ✅ |
| GCC/MinGW | ✅ | ✅ | ✅ |
| air (dev) | Optional | Optional | Optional |
| golangci-lint (dev) | Optional | Optional | Optional |

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Web Browser (HTML5)                            │
│                    Dashboard • Migrations • Admin Portal                │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Octopus Server (Go)                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │
│  │    REST API     │  │    Scheduler    │  │   Sync Engine (CBT)     │  │
│  │  Authentication │  │  Task Queue     │  │  Block-Level Replication│  │
│  │  Authorization  │  │  Cron Jobs      │  │  Incremental Sync       │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │
│  │   AD/LDAP Auth  │  │  SQLite DB      │  │   Provider Modules      │  │
│  │   JWT Tokens    │  │  Migrations     │  │  ┌─────┐ ┌─────┐        │  │
│  │   Sessions      │  │  Activity Logs  │  │  │VMware│ │ AWS │        │  │
│  └─────────────────┘  └─────────────────┘  │  ├─────┤ ├─────┤        │  │
│                                            │  │ GCP │ │Azure│        │  │
│                                            │  └─────┘ └─────┘        │  │
│                                            └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
              │                                           │
              ▼                                           ▼
┌───────────────────────────┐           ┌─────────────────────────────────┐
│    Source Environments    │           │      Target Environments        │
│  ┌─────────────────────┐  │           │  ┌───────┐ ┌───────┐ ┌───────┐  │
│  │  VMware vCenter     │  │           │  │VMware │ │  AWS  │ │  GCP  │  │
│  │  • ESXi Clusters    │  │    ───►   │  │vCenter│ │  EC2  │ │Compute│  │
│  │  • VXRail           │  │           │  └───────┘ └───────┘ └───────┘  │
│  │  • vSAN             │  │           │            ┌───────┐            │
│  └─────────────────────┘  │           │            │ Azure │            │
└───────────────────────────┘           │            │  VMs  │            │
                                        │            └───────┘            │
                                        └─────────────────────────────────┘
```

---

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server listen port | `8080` |
| `DATABASE_PATH` | SQLite database location | `/data/octopus.db` |
| `JWT_SECRET` | Secret for JWT signing | `change-me-in-production` |
| `SESSION_KEY` | Secret for session cookies | `change-me-in-production-too` |
| `AD_SERVER` | Active Directory server | *(empty = local auth)* |
| `AD_BASE_DN` | AD search base DN | - |
| `AD_BIND_USER` | AD service account | - |
| `AD_BIND_PASS` | AD service account password | - |
| `AD_DOMAIN` | AD domain name | - |

### Configuration File

Create `docker/config/config.yaml`:

```yaml
database_path: /data/octopus.db

# Active Directory (leave empty for local auth)
ad_server: "ldap.example.com"
ad_base_dn: "DC=example,DC=com"
ad_bind_user: "CN=octopus,OU=Service,DC=example,DC=com"
ad_bind_pass: "secure-password"
ad_domain: "EXAMPLE"

# Security
jwt_secret: "your-secure-random-string-here"
jwt_expiration_hours: 24

# Provider defaults
vmware_defaults:
  insecure: false

aws_defaults:
  region: "us-east-1"

gcp_defaults:
  zone: "us-central1-a"

azure_defaults:
  location: "eastus"
```

---

## 📖 Usage

### Migration Workflow

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Add Source  │───►│   Sync VMs   │───►│  Add Target  │───►│ Estimate Size│
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
                                                                    │
┌──────────────┐    ┌──────────────┐    ┌──────────────┐            │
│   Cutover    │◄───│Schedule Task │◄───│ Continuous   │◄───────────┘
│  Execution   │    │              │    │    Sync      │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Step-by-Step Guide

1. **Add Source Environment** - Configure your VMware vCenter connection
2. **Sync Virtual Machines** - Discover and inventory VMs
3. **Add Target Environment** - Configure AWS, GCP, Azure, or another vCenter
4. **Estimate Size** - Compare storage requirements across targets
5. **Create Migration Job** - Select VM and configure migration options
6. **Monitor Sync** - Watch continuous synchronization progress
7. **Schedule Cutover** - Set a time for final migration
8. **Execute Cutover** - Final sync, power off source, power on target

---

## 📐 VXRail/vSAN Size Estimation

When migrating from VXRail/vSAN to plain ESXi or cloud targets, storage sizes change due to several factors:

### Estimation Factors

| Factor | Impact | Description |
|--------|--------|-------------|
| **RAID Overhead** | 25-67% reduction | RAID-1 (2x), RAID-5 (1.33x), RAID-6 (1.5x) overhead removed |
| **Deduplication** | 20-60% expansion | Deduplicated data expands when copied |
| **Compression** | 20-30% expansion | Compressed data expands when copied |
| **Snapshots** | 10% reduction | Snapshots consolidate during migration |
| **VM Swap** | 5% reduction | Swap files regenerated on target |

### Using the Web Interface

The web interface provides size estimation with organic factor adjustments:
1. Navigate to **VMs** section
2. Click **Estimate Size** on any VM
3. View breakdown: vSAN Reported → Logical Data → Migration Estimate

### Using the Standalone Script

For accurate vSAN-aware estimates, use the standalone Python script:

```bash
# Install dependency
pip install pyvmomi

# Run estimation (prompts for password)
python vxrail_size_estimator.py --vcenter vcenter.example.com --username admin@vsphere.local

# Output saved to: vxrail_estimate_<vcenter>_<timestamp>.csv
```

The script connects directly to vCenter and:
- Auto-detects vSAN cluster configuration
- Queries actual RAID policy, dedup/compression settings
- Calculates accurate migration estimates
- Exports CSV with full breakdown

See [Size Estimation Guide](docs/size-estimation.md) for detailed methodology.

---

## 🔌 API Reference

### Authentication

```bash
# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'
```

### Key Endpoints

| Category | Endpoint | Methods |
|----------|----------|---------|
| **Auth** | `/api/v1/auth/login` | POST |
| **Sources** | `/api/v1/sources` | GET, POST |
| **Sources** | `/api/v1/sources/:id` | GET, PUT, DELETE |
| **Sources** | `/api/v1/sources/:id/sync` | POST |
| **Targets** | `/api/v1/targets` | GET, POST |
| **Targets** | `/api/v1/targets/:id` | GET, PUT, DELETE |
| **VMs** | `/api/v1/vms` | GET |
| **VMs** | `/api/v1/vms/:id` | GET |
| **VMs** | `/api/v1/vms/:id/estimate` | POST |
| **Migrations** | `/api/v1/migrations` | GET, POST |
| **Migrations** | `/api/v1/migrations/:id` | GET, PUT |
| **Migrations** | `/api/v1/migrations/:id/sync` | POST |
| **Migrations** | `/api/v1/migrations/:id/cutover` | POST |
| **Schedules** | `/api/v1/schedules` | GET, POST |
| **Admin** | `/api/v1/admin/env` | GET, POST |
| **Admin** | `/api/v1/admin/logs` | GET |
| **Admin** | `/api/v1/admin/users` | GET |

---

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [Installation Guide](docs/installation.md) | Detailed installation instructions |
| [Configuration Guide](docs/configuration.md) | Configuration options reference |
| [User Guide](docs/user-guide.md) | How to use Octopus |
| [API Reference](docs/api-reference.md) | Complete API documentation |
| [Architecture](docs/architecture.md) | System architecture details |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and solutions |

---

## 🗂️ Project Structure

```
octopus/
├── 📁 server/                    # Go backend
│   ├── 📁 cmd/                   # Entry point
│   └── 📁 internal/
│       ├── 📁 api/               # REST API handlers
│       ├── 📁 auth/              # Authentication (AD/LDAP)
│       ├── 📁 config/            # Configuration management
│       ├── 📁 db/                # Database layer
│       ├── 📁 providers/         # Cloud providers
│       │   ├── 📁 vmware/        # VMware vCenter
│       │   ├── 📁 aws/           # Amazon Web Services
│       │   ├── 📁 gcp/           # Google Cloud Platform
│       │   └── 📁 azure/         # Microsoft Azure
│       ├── 📁 scheduler/         # Task scheduler
│       └── 📁 sync/              # VM sync engine
├── 📁 client/                    # HTML5 web client
│   ├── 📁 templates/             # HTML templates
│   └── 📁 static/                # CSS & JavaScript
├── 📁 docker/                    # Docker configuration
├── 📁 scripts/                   # Installation scripts
│   ├── 📄 install.sh             # Linux/macOS installer
│   ├── 📄 install.ps1            # Windows installer
│   └── 📄 install.bat            # Windows batch wrapper
├── 📁 docs/                      # Documentation
├── 📄 vxrail_size_estimator.py   # Standalone VXRail size estimator
├── 📄 Makefile                   # Build automation
└── 📄 README.md                  # This file
```

---

## 🛠️ Development

### Make Commands

| Command | Description |
|---------|-------------|
| `make build` | Build the server binary |
| `make run` | Build and run locally |
| `make test` | Run tests |
| `make deps` | Download dependencies |
| `make docker-build` | Build Docker image |
| `make docker-run` | Start with Docker Compose |
| `make docker-stop` | Stop containers |
| `make docker-logs` | View container logs |
| `make dev` | Run with hot-reload |
| `make fmt` | Format code |
| `make lint` | Run linter |
| `make help` | Show all commands |

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made with ❤️ for the VMware community
</p>

<p align="center">
  <a href="https://github.com/sp00nznet/octopus/issues">Report Bug</a> •
  <a href="https://github.com/sp00nznet/octopus/issues">Request Feature</a>
</p>
