# Architecture Overview

This document describes the technical architecture of Octopus.

## System Overview

Octopus is a VM migration and disaster recovery platform built with a modern microservices-inspired architecture. While deployed as a monolithic application for simplicity, it follows clean separation of concerns internally.

```
┌──────────────────────────────────────────────────────────────────┐
│                        Presentation Layer                         │
│                                                                    │
│   ┌────────────────┐    ┌─────────────────────────────────────┐  │
│   │  HTML5 Client  │    │           REST API                   │  │
│   │  (JavaScript)  │◄──►│    (JSON over HTTP/HTTPS)           │  │
│   └────────────────┘    └─────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────┐
│                       Application Layer                           │
│                                                                    │
│   ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│   │   API        │  │  Scheduler   │  │   Sync Engine        │   │
│   │   Handlers   │  │  Service     │  │   Service            │   │
│   └──────────────┘  └──────────────┘  └──────────────────────┘   │
│                                                                    │
│   ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│   │   Auth       │  │   Config     │  │   Size Estimation    │   │
│   │   Service    │  │   Service    │  │   Service            │   │
│   └──────────────┘  └──────────────┘  └──────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────┐
│                      Integration Layer                            │
│                                                                    │
│   ┌─────────────────────────────────────────────────────────┐    │
│   │                  Provider Abstraction                    │    │
│   └─────────────────────────────────────────────────────────┘    │
│         │              │              │              │            │
│   ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐    │
│   │  VMware   │  │   AWS     │  │   GCP     │  │  Azure    │    │
│   │  Provider │  │  Provider │  │  Provider │  │  Provider │    │
│   └───────────┘  └───────────┘  └───────────┘  └───────────┘    │
└──────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────┐
│                       Data Layer                                  │
│                                                                    │
│   ┌─────────────────────────────────────────────────────────┐    │
│   │                    SQLite Database                       │    │
│   │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│   │  │  Users   │ │   VMs    │ │Migrations│ │  Logs    │   │    │
│   │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│   └─────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

## Components

### Web Client

The HTML5 web client is a single-page application (SPA) built with vanilla JavaScript.

**Technologies:**
- HTML5 / CSS3
- Vanilla JavaScript (ES6+)
- Fetch API for HTTP requests
- CSS Grid/Flexbox for layout

**Key Features:**
- Responsive design
- Real-time updates
- No framework dependencies

### REST API Server

The Go server provides a RESTful API for all operations.

**Technologies:**
- Go 1.21+
- Gorilla Mux (routing)
- Standard library for HTTP

**Key Features:**
- JWT-based authentication
- Middleware support
- JSON request/response

### Authentication Service

Handles user authentication via Active Directory or local auth.

**Technologies:**
- go-ldap/ldap (LDAP client)
- golang-jwt/jwt (JWT tokens)

**Flow:**
```
User → Login → LDAP Bind → Generate JWT → Return Token
```

### Scheduler Service

Manages scheduled tasks and background operations.

**Features:**
- Periodic sync execution
- Scheduled cutovers
- Task queue management

**Implementation:**
- Go ticker for periodic tasks
- Database-backed task storage
- Concurrent execution with goroutines

### Sync Engine

Handles VM synchronization using Changed Block Tracking (CBT).

**VMware CBT Process:**
1. Create source snapshot
2. Query changed blocks since last sync
3. Transfer changed blocks
4. Update target disk
5. Remove snapshot

**Features:**
- Incremental sync
- Block-level replication
- Bandwidth optimization

### Provider Modules

Abstract cloud provider operations behind a common interface.

#### VMware Provider

**Library:** govmomi

**Capabilities:**
- VM inventory
- Snapshot management
- Disk operations
- Power management
- CBT queries

#### AWS Provider

**Library:** aws-sdk-go-v2

**Capabilities:**
- Image import from S3
- EC2 instance management
- EBS snapshot operations
- Instance type mapping

#### GCP Provider

**Library:** cloud.google.com/go/compute

**Capabilities:**
- Image creation from GCS
- Compute Engine instances
- Disk management
- Machine type mapping

#### Azure Provider

**Library:** azure-sdk-for-go

**Capabilities:**
- VHD image import
- VM deployment
- Managed disk operations
- VM size mapping

### Database

SQLite database for persistent storage.

**Tables:**
- `users` - User accounts
- `source_environments` - vCenter connections
- `target_environments` - Migration destinations
- `vms` - VM inventory
- `migration_jobs` - Migration tracking
- `sync_history` - Sync logs
- `scheduled_tasks` - Scheduled operations
- `activity_logs` - Audit trail
- `env_variables` - Configuration storage
- `size_estimations` - Size calculations

## Data Flow

### Migration Sync Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Source    │     │   Octopus   │     │   Target    │
│   vCenter   │     │   Server    │     │   Cloud     │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │   1. Create       │                   │
       │◄──Snapshot────────│                   │
       │                   │                   │
       │   2. Query CBT    │                   │
       │◄──Changed Blocks──│                   │
       │                   │                   │
       │   3. Read Blocks  │                   │
       │──────────────────►│                   │
       │                   │                   │
       │                   │   4. Write Blocks │
       │                   │──────────────────►│
       │                   │                   │
       │   5. Delete       │                   │
       │◄──Snapshot────────│                   │
       │                   │                   │
```

### Authentication Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │     │   Server    │     │   AD/LDAP   │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │   1. Login        │                   │
       │   (user/pass)     │                   │
       │──────────────────►│                   │
       │                   │                   │
       │                   │   2. LDAP Bind    │
       │                   │──────────────────►│
       │                   │                   │
       │                   │   3. Verify       │
       │                   │◄──────────────────│
       │                   │                   │
       │   4. JWT Token    │                   │
       │◄──────────────────│                   │
       │                   │                   │
       │   5. API Request  │                   │
       │   + Bearer Token  │                   │
       │──────────────────►│                   │
       │                   │                   │
```

## Security Model

### Authentication

- **Active Directory**: LDAP bind for corporate environments
- **Local Auth**: Username/password for development
- **JWT Tokens**: Stateless authentication for API

### Authorization

- **Role-Based**: Admin and regular user roles
- **Resource-Based**: Actions restricted to owned resources

### Data Protection

- **Secrets**: Passwords stored encrypted
- **Transport**: HTTPS recommended for production
- **Database**: File-level permissions on SQLite

## Scalability Considerations

### Current Limitations

- Single instance deployment
- SQLite database (not horizontally scalable)
- Synchronous sync operations

### Future Improvements

1. **PostgreSQL Support**
   - For multi-instance deployments
   - Better concurrent access

2. **Worker Queues**
   - Distributed sync operations
   - Better resource utilization

3. **Caching Layer**
   - Redis for session storage
   - API response caching

4. **High Availability**
   - Active-passive failover
   - Database replication

## Deployment Options

### Single Server

```
┌─────────────────────────────────────┐
│           Single Host               │
│  ┌─────────────────────────────┐   │
│  │      Octopus Server         │   │
│  │  ┌─────────┐ ┌──────────┐  │   │
│  │  │ Web UI  │ │  API     │  │   │
│  │  └─────────┘ └──────────┘  │   │
│  │  ┌─────────────────────┐   │   │
│  │  │  SQLite Database    │   │   │
│  │  └─────────────────────┘   │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

### Docker Compose

```
┌─────────────────────────────────────┐
│         Docker Host                 │
│  ┌─────────────────────────────┐   │
│  │   octopus-server container  │   │
│  │  ┌───────────────────────┐ │   │
│  │  │    Octopus Server     │ │   │
│  │  └───────────────────────┘ │   │
│  │  ┌───────────────────────┐ │   │
│  │  │   Volume: /data       │ │   │
│  │  │   (SQLite DB)         │ │   │
│  │  └───────────────────────┘ │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

### Reverse Proxy Setup

```
┌─────────────────────────────────────────────────────┐
│                   Production Setup                   │
│                                                      │
│  ┌────────────────┐      ┌──────────────────────┐  │
│  │  Nginx/Traefik │      │   Octopus Server     │  │
│  │  (TLS, Cache)  │─────►│   (Port 8080)        │  │
│  │  Port 443      │      │                      │  │
│  └────────────────┘      └──────────────────────┘  │
│                                                      │
└─────────────────────────────────────────────────────┘
```

## Technology Stack Summary

| Layer | Technology |
|-------|------------|
| Frontend | HTML5, CSS3, JavaScript |
| Backend | Go 1.21+ |
| Routing | Gorilla Mux |
| Auth | go-ldap, golang-jwt |
| Database | SQLite 3 |
| VMware | govmomi |
| AWS | aws-sdk-go-v2 |
| GCP | google-cloud-go |
| Azure | azure-sdk-for-go |
| Container | Docker, Docker Compose |
