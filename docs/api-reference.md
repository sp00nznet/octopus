# API Reference

Complete reference for the Octopus REST API.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

All endpoints except `/auth/login` and `/health` require authentication.

### Login

```http
POST /auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "username": "admin",
    "display_name": "Administrator",
    "email": "admin@localhost",
    "is_admin": true
  }
}
```

### Using the Token

Include the token in the Authorization header:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

---

## Health Check

### Get Health Status

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy"
}
```

---

## Source Environments

### List Sources

```http
GET /sources
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "Production vCenter",
    "type": "vmware",
    "host": "vcenter.example.com",
    "username": "administrator@vsphere.local",
    "datacenter": "DC1",
    "cluster": "Cluster-01",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

### Create Source

```http
POST /sources
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Production vCenter",
  "type": "vmware",
  "host": "vcenter.example.com",
  "username": "administrator@vsphere.local",
  "password": "secure-password",
  "datacenter": "DC1",
  "cluster": "Cluster-01"
}
```

**Response:**
```json
{
  "id": 1
}
```

### Get Source

```http
GET /sources/:id
Authorization: Bearer <token>
```

### Update Source

```http
PUT /sources/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Production vCenter Updated",
  "host": "vcenter.example.com",
  "username": "administrator@vsphere.local",
  "password": "new-password",
  "datacenter": "DC1",
  "cluster": "Cluster-02"
}
```

### Delete Source

```http
DELETE /sources/:id
Authorization: Bearer <token>
```

### Sync Source

Discover VMs from the source environment.

```http
POST /sources/:id/sync
Authorization: Bearer <token>
```

**Response:**
```json
{
  "status": "synced",
  "vm_count": 42
}
```

---

## Target Environments

### List Targets

```http
GET /targets
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "AWS Production",
    "type": "aws",
    "config_json": "{\"region\":\"us-east-1\"}",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

### Create Target

#### VMware Target

```http
POST /targets
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "DR vCenter",
  "type": "vmware",
  "config": {
    "host": "vcenter-dr.example.com",
    "username": "administrator@vsphere.local",
    "password": "secure-password",
    "datacenter": "DR-DC1"
  }
}
```

#### AWS Target

```http
POST /targets
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "AWS Production",
  "type": "aws",
  "config": {
    "region": "us-east-1",
    "access_key_id": "AKIA...",
    "secret_access_key": "..."
  }
}
```

#### GCP Target

```http
POST /targets
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "GCP Project",
  "type": "gcp",
  "config": {
    "project_id": "my-project-123",
    "zone": "us-central1-a",
    "credentials": "{ service account json }"
  }
}
```

#### Azure Target

```http
POST /targets
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Azure Subscription",
  "type": "azure",
  "config": {
    "subscription_id": "12345678-...",
    "resource_group": "octopus-rg",
    "tenant_id": "abcdef12-...",
    "client_id": "98765432-...",
    "client_secret": "..."
  }
}
```

### Get Target

```http
GET /targets/:id
Authorization: Bearer <token>
```

### Update Target

```http
PUT /targets/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "AWS Production Updated",
  "type": "aws",
  "config": {
    "region": "us-west-2",
    "access_key_id": "AKIA...",
    "secret_access_key": "..."
  }
}
```

### Delete Target

```http
DELETE /targets/:id
Authorization: Bearer <token>
```

---

## Virtual Machines

### List VMs

```http
GET /vms
Authorization: Bearer <token>
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| source_id | int | Filter by source environment ID |

**Response:**
```json
[
  {
    "id": 1,
    "source_env_id": 1,
    "name": "web-server-01",
    "uuid": "421a3c8a-...",
    "cpu_count": 4,
    "memory_mb": 8192,
    "disk_size_gb": 100.5,
    "guest_os": "Ubuntu 22.04 (64-bit)",
    "power_state": "poweredOn",
    "ip_addresses": "10.0.1.50,192.168.1.50",
    "mac_addresses": "00:50:56:a1:b2:c3",
    "port_groups": "Production-Network",
    "hardware_version": "vmx-19",
    "vmware_tools_status": "guestToolsRunning",
    "last_synced": "2024-01-15T12:00:00Z",
    "created_at": "2024-01-15T10:30:00Z"
  }
]
```

### Get VM

```http
GET /vms/:id
Authorization: Bearer <token>
```

### Estimate VM Size

Get size estimates for migrating to different targets.

```http
POST /vms/:id/estimate
Authorization: Bearer <token>
Content-Type: application/json

{
  "target_type": "aws",
  "is_vxrail": false
}
```

**Response:**
```json
{
  "source_size_gb": 100.5,
  "estimated_size_gb": 101.0,
  "size_difference_gb": 0.5,
  "notes": "AWS EBS GP3 volumes. Size rounded up to nearest GiB."
}
```

---

## Migrations

### List Migrations

```http
GET /migrations
Authorization: Bearer <token>
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| status | string | Filter by status |

**Status Values:** `pending`, `syncing`, `ready`, `cutting_over`, `completed`, `failed`, `cancelled`

**Response:**
```json
[
  {
    "id": 1,
    "vm_id": 1,
    "source_env_id": 1,
    "target_env_id": 1,
    "status": "syncing",
    "progress": 45,
    "preserve_mac": true,
    "preserve_port_groups": true,
    "sync_interval_minutes": 60,
    "scheduled_cutover": "2024-01-20T02:00:00Z",
    "error_message": null,
    "created_by": "admin",
    "created_at": "2024-01-15T10:30:00Z",
    "started_at": "2024-01-15T10:35:00Z",
    "completed_at": null,
    "vm_name": "web-server-01",
    "source_name": "Production vCenter",
    "target_name": "AWS Production"
  }
]
```

### Create Migration

```http
POST /migrations
Authorization: Bearer <token>
Content-Type: application/json

{
  "vm_id": 1,
  "source_env_id": 1,
  "target_env_id": 1,
  "preserve_mac": true,
  "preserve_port_groups": true,
  "sync_interval_minutes": 60,
  "scheduled_cutover": "2024-01-20T02:00:00Z"
}
```

**Response:**
```json
{
  "id": 1
}
```

### Get Migration

```http
GET /migrations/:id
Authorization: Bearer <token>
```

### Update Migration

```http
PUT /migrations/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "sync_interval_minutes": 30,
  "scheduled_cutover": "2024-01-21T02:00:00Z"
}
```

### Cancel Migration

```http
POST /migrations/:id/cancel
Authorization: Bearer <token>
```

### Trigger Sync

Manually trigger a sync operation.

```http
POST /migrations/:id/sync
Authorization: Bearer <token>
```

**Response:**
```json
{
  "status": "sync_started"
}
```

### Trigger Cutover

Execute the final cutover.

```http
POST /migrations/:id/cutover
Authorization: Bearer <token>
```

**Response:**
```json
{
  "status": "cutover_started"
}
```

---

## Scheduled Tasks

### List Scheduled Tasks

```http
GET /schedules
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": 1,
    "job_id": 1,
    "task_type": "cutover",
    "scheduled_time": "2024-01-20T02:00:00Z",
    "status": "pending",
    "result": null,
    "created_by": "admin",
    "created_at": "2024-01-15T10:30:00Z",
    "executed_at": null
  }
]
```

### Create Scheduled Task

```http
POST /schedules
Authorization: Bearer <token>
Content-Type: application/json

{
  "job_id": 1,
  "task_type": "cutover",
  "scheduled_time": "2024-01-20T02:00:00Z"
}
```

**Task Types:** `cutover`, `failover`, `sync`, `test_failover`

### Get Scheduled Task

```http
GET /schedules/:id
Authorization: Bearer <token>
```

### Cancel Scheduled Task

```http
POST /schedules/:id/cancel
Authorization: Bearer <token>
```

---

## Admin Endpoints

These endpoints require administrator privileges.

### Environment Variables

#### List Environment Variables

```http
GET /admin/env
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "VCENTER_DEFAULT_DATACENTER",
    "value": "DC1",
    "description": "Default datacenter for new connections",
    "is_secret": false,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

#### Create Environment Variable

```http
POST /admin/env
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "AWS_DEFAULT_REGION",
  "value": "us-east-1",
  "description": "Default AWS region",
  "is_secret": false
}
```

#### Update Environment Variable

```http
PUT /admin/env/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "AWS_DEFAULT_REGION",
  "value": "us-west-2",
  "description": "Default AWS region (updated)",
  "is_secret": false
}
```

#### Delete Environment Variable

```http
DELETE /admin/env/:id
Authorization: Bearer <token>
```

### Activity Logs

#### List Activity Logs

```http
GET /admin/logs
Authorization: Bearer <token>
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| limit | int | Maximum number of logs (default: 100) |

**Response:**
```json
[
  {
    "id": 1,
    "user_id": 1,
    "action": "create_migration",
    "entity_type": "migration",
    "entity_id": 1,
    "details": "Created migration for web-server-01",
    "ip_address": "192.168.1.100",
    "created_at": "2024-01-15T10:30:00Z",
    "username": "admin"
  }
]
```

### Users

#### List Users

```http
GET /admin/users
Authorization: Bearer <token>
```

**Response:**
```json
[
  {
    "id": 1,
    "username": "admin",
    "email": "admin@localhost",
    "display_name": "Administrator",
    "is_admin": true,
    "created_at": "2024-01-15T10:00:00Z",
    "last_login": "2024-01-15T10:30:00Z"
  }
]
```

#### Get User

```http
GET /admin/users/:id
Authorization: Bearer <token>
```

#### Toggle Admin Status

```http
PUT /admin/users/:id/admin
Authorization: Bearer <token>
Content-Type: application/json

{
  "is_admin": true
}
```

---

## Error Responses

All errors return a JSON object with an `error` field:

```json
{
  "error": "Error message describing what went wrong"
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Invalid or missing token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |

---

## Rate Limiting

Currently, no rate limiting is implemented. For production deployments, consider using a reverse proxy (nginx, traefik) with rate limiting configured.
