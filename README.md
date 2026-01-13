# Octopus

Octopus is a VM migration and disaster recovery tool that enables seamless migration of virtual machines from VMware vCenter to multiple cloud platforms including AWS, GCP, Azure, and other VMware vCenter instances.

## Features

- **Multi-Cloud Migration**: Migrate VMs from VMware vCenter to:
  - VMware vCenter (another instance)
  - Amazon Web Services (AWS)
  - Google Cloud Platform (GCP)
  - Microsoft Azure

- **Disaster Recovery**:
  - Continuous VM synchronization using Changed Block Tracking (CBT)
  - Scheduled cutover/failover operations
  - Test failover capability (non-destructive)

- **Hardware Preservation**:
  - Preserve MAC addresses during migration
  - Maintain distributed port group configurations
  - Retain VM hardware configuration

- **Size Estimation**:
  - Compare storage requirements across platforms
  - Handle VXRail vs plain ESXi size differences
  - Cost estimation for cloud targets

- **Web Interface**:
  - HTML5 web client accessible from any browser
  - Active Directory authentication
  - Admin portal for environment variable management
  - Activity logging and audit trail

- **Containerized Architecture**:
  - Docker-ready server and client
  - Persistent storage for tracking migrations
  - Easy deployment with Docker Compose

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Web Browser (HTML5)                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Octopus Server (Go)                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   REST API  │  │ Scheduler   │  │  Sync Engine (CBT)  │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  AD Auth    │  │  Database   │  │  Provider Modules   │  │
│  │  (LDAP)     │  │  (SQLite)   │  │  VMware/AWS/GCP/Az  │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
         │                                      │
         ▼                                      ▼
┌─────────────────┐              ┌─────────────────────────────┐
│  Source vCenter │              │     Target Environments     │
│    Clusters     │              │  vCenter/AWS/GCP/Azure      │
└─────────────────┘              └─────────────────────────────┘
```

## Quick Start

### Using Docker Compose (Recommended)

1. Clone the repository:
   ```bash
   git clone https://github.com/sp00nznet/octopus.git
   cd octopus
   ```

2. Initialize configuration:
   ```bash
   make init
   ```

3. Edit the configuration file:
   ```bash
   vim docker/config/config.yaml
   ```

4. Start the application:
   ```bash
   make docker-run
   ```

5. Access the web interface at http://localhost:8080

### Development Setup

1. Install Go 1.21 or later

2. Install dependencies:
   ```bash
   make deps
   ```

3. Run the server:
   ```bash
   make run
   ```

4. For development with hot-reload (requires [air](https://github.com/cosmtrek/air)):
   ```bash
   make dev
   ```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DATABASE_PATH` | SQLite database path | `/data/octopus.db` |
| `JWT_SECRET` | Secret for JWT tokens | `change-me-in-production` |
| `SESSION_KEY` | Secret for sessions | `change-me-in-production-too` |
| `AD_SERVER` | Active Directory server | (empty - local auth) |
| `AD_BASE_DN` | AD base DN for searches | - |
| `AD_BIND_USER` | AD service account DN | - |
| `AD_BIND_PASS` | AD service account password | - |
| `AD_DOMAIN` | AD domain name | - |

### Authentication

**Development Mode**: When `AD_SERVER` is not configured, local authentication is used:
- Username: `admin`, Password: `admin` (admin user)
- Any username with password matching the username (regular user)

**Production Mode**: Configure AD settings for LDAP authentication against Active Directory.

## API Endpoints

### Authentication
- `POST /api/v1/auth/login` - Login with credentials

### Source Environments
- `GET /api/v1/sources` - List all source environments
- `POST /api/v1/sources` - Create a source environment
- `GET /api/v1/sources/:id` - Get source environment details
- `PUT /api/v1/sources/:id` - Update source environment
- `DELETE /api/v1/sources/:id` - Delete source environment
- `POST /api/v1/sources/:id/sync` - Sync VMs from source

### Target Environments
- `GET /api/v1/targets` - List all target environments
- `POST /api/v1/targets` - Create a target environment
- `GET /api/v1/targets/:id` - Get target environment details
- `PUT /api/v1/targets/:id` - Update target environment
- `DELETE /api/v1/targets/:id` - Delete target environment

### Virtual Machines
- `GET /api/v1/vms` - List all VMs
- `GET /api/v1/vms/:id` - Get VM details
- `POST /api/v1/vms/:id/estimate` - Estimate VM size for target

### Migrations
- `GET /api/v1/migrations` - List all migrations
- `POST /api/v1/migrations` - Create a migration job
- `GET /api/v1/migrations/:id` - Get migration details
- `PUT /api/v1/migrations/:id` - Update migration
- `POST /api/v1/migrations/:id/cancel` - Cancel migration
- `POST /api/v1/migrations/:id/sync` - Trigger sync
- `POST /api/v1/migrations/:id/cutover` - Trigger cutover

### Scheduled Tasks
- `GET /api/v1/schedules` - List scheduled tasks
- `POST /api/v1/schedules` - Create scheduled task
- `GET /api/v1/schedules/:id` - Get task details
- `POST /api/v1/schedules/:id/cancel` - Cancel task

### Admin (requires admin role)
- `GET /api/v1/admin/env` - List environment variables
- `POST /api/v1/admin/env` - Create environment variable
- `PUT /api/v1/admin/env/:id` - Update environment variable
- `DELETE /api/v1/admin/env/:id` - Delete environment variable
- `GET /api/v1/admin/logs` - Get activity logs
- `GET /api/v1/admin/users` - List users
- `PUT /api/v1/admin/users/:id/admin` - Toggle user admin status

## Project Structure

```
octopus/
├── server/                    # Go backend
│   ├── cmd/                   # Main entry point
│   │   └── main.go
│   └── internal/
│       ├── api/               # REST API handlers
│       ├── auth/              # AD/LDAP authentication
│       ├── config/            # Configuration management
│       ├── db/                # Database and models
│       ├── providers/         # Cloud provider clients
│       │   ├── vmware/        # VMware vCenter client
│       │   ├── aws/           # AWS EC2 client
│       │   ├── gcp/           # GCP Compute client
│       │   └── azure/         # Azure VM client
│       ├── scheduler/         # Task scheduler
│       └── sync/              # VM sync engine
├── client/                    # HTML5 web client
│   ├── templates/             # HTML templates
│   └── static/
│       ├── css/               # Stylesheets
│       └── js/                # JavaScript
├── docker/                    # Docker configuration
│   ├── Dockerfile.server
│   ├── docker-compose.yml
│   └── config/                # Configuration files
├── Makefile                   # Build automation
└── README.md
```

## Migration Workflow

1. **Add Source Environment**: Configure your VMware vCenter as a source
2. **Sync VMs**: Discover VMs from the source environment
3. **Add Target Environment**: Configure AWS, GCP, Azure, or another vCenter
4. **Size Estimation**: Compare VM sizes across targets before migration
5. **Create Migration**: Select VM, source, target, and migration options
6. **Continuous Sync**: Octopus keeps the target in sync using CBT
7. **Schedule Cutover**: Set a time for the final cutover
8. **Cutover Execution**: Final sync, source poweroff, target poweron

## License

MIT License
