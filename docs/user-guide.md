# User Guide

This guide explains how to use Octopus to migrate virtual machines from VMware vCenter to various cloud platforms.

## Table of Contents

- [Getting Started](#getting-started)
- [Dashboard Overview](#dashboard-overview)
- [Managing Source Environments](#managing-source-environments)
- [Managing Target Environments](#managing-target-environments)
- [Working with Virtual Machines](#working-with-virtual-machines)
- [Creating Migrations](#creating-migrations)
- [Scheduling Tasks](#scheduling-tasks)
- [Admin Portal](#admin-portal)
- [Best Practices](#best-practices)

---

## Getting Started

### Logging In

1. Navigate to `http://localhost:8080` (or your server's address)
2. Enter your credentials:
   - **Development Mode**: Use `admin` / `admin`
   - **Production**: Use your Active Directory credentials
3. Click **Login**

### First-Time Setup

1. **Add a Source Environment** - Connect to your VMware vCenter
2. **Sync VMs** - Discover available virtual machines
3. **Add Target Environments** - Configure migration destinations
4. **Create a Migration** - Start migrating VMs

---

## Dashboard Overview

The dashboard provides an at-a-glance view of your migration status.

### Statistics Cards

| Card | Description |
|------|-------------|
| Source Environments | Number of connected vCenter instances |
| Target Environments | Number of configured migration targets |
| Total VMs | Number of discovered virtual machines |
| Active Migrations | Migrations currently in progress |

### Recent Migrations

Shows the last 5 migration jobs with:
- VM name
- Source and target environments
- Current status
- Progress percentage

### Upcoming Tasks

Lists scheduled tasks pending execution:
- Task type (cutover, sync, failover)
- Associated job ID
- Scheduled time
- Status

---

## Managing Source Environments

Source environments are VMware vCenter instances from which you migrate VMs.

### Adding a Source Environment

1. Navigate to **Sources**
2. Click **Add Source**
3. Fill in the form:

| Field | Description | Example |
|-------|-------------|---------|
| Name | Friendly name | `Production vCenter` |
| Type | Environment type | `VMware vCenter` |
| Host | vCenter hostname/IP | `vcenter.example.com` |
| Username | vCenter username | `administrator@vsphere.local` |
| Password | vCenter password | `********` |
| Datacenter | Target datacenter | `DC1` |
| Cluster | Optional cluster filter | `Cluster-01` |

4. Click **Add Source**

### Syncing VMs

After adding a source:
1. Click **Sync** next to the source
2. Wait for the sync to complete
3. Check the **VMs** page for discovered machines

**Note**: Sync pulls VM inventory, configurations, and current state from vCenter.

### Editing a Source

1. Click on the source name
2. Update the desired fields
3. Click **Save**

**Note**: Changing credentials requires re-authentication.

### Deleting a Source

1. Ensure no active migrations use this source
2. Click **Delete** next to the source
3. Confirm the deletion

**Warning**: Deleting a source removes all associated VM records.

---

## Managing Target Environments

Target environments are destinations for migrated VMs.

### Target Types

| Type | Description | Use Case |
|------|-------------|----------|
| VMware | Another vCenter | Datacenter migration |
| AWS | Amazon EC2 | Cloud migration |
| GCP | Google Compute Engine | Cloud migration |
| Azure | Microsoft Azure VMs | Cloud migration |

### Adding a VMware Target

```
Name: DR vCenter
Type: VMware vCenter
Host: vcenter-dr.example.com
Username: administrator@vsphere.local
Password: ********
Datacenter: DR-DC1
```

### Adding an AWS Target

```
Name: AWS Production
Type: AWS
Region: us-east-1
Access Key ID: AKIA...
Secret Access Key: ********
```

### Adding a GCP Target

```
Name: GCP Project
Type: Google Cloud Platform
Project ID: my-project-123
Zone: us-central1-a
Service Account JSON: { ... }
```

### Adding an Azure Target

```
Name: Azure Subscription
Type: Microsoft Azure
Subscription ID: 12345678-...
Resource Group: octopus-rg
Tenant ID: abcdef12-...
Client ID: 98765432-...
Client Secret: ********
```

---

## Working with Virtual Machines

### VM Inventory

The VMs page shows all discovered virtual machines:

| Column | Description |
|--------|-------------|
| Name | VM display name |
| CPUs | Number of vCPUs |
| Memory | RAM in GB |
| Disk | Total disk size in GB |
| Power State | Current power state |
| Last Synced | Last inventory update |

### Filtering VMs

Use the dropdown to filter by source environment.

### Size Estimation

Before migrating, estimate the target size:

1. Click **Estimate Size** on a VM
2. View size comparisons across targets:
   - Source size
   - Estimated target size
   - Size difference
   - Notes about the estimation

**VXRail Consideration**: VXRail deployments include vSAN overhead (approximately 10%) that may not apply to other targets.

---

## Creating Migrations

### Migration Workflow

```
Source VM → Initial Sync → Continuous Sync → Cutover → Target VM
```

### Creating a New Migration

1. Navigate to **Migrations**
2. Click **New Migration**
3. Configure the migration:

| Field | Description |
|-------|-------------|
| Virtual Machine | Select the VM to migrate |
| Source Environment | Source vCenter |
| Target Environment | Destination platform |
| Preserve MAC Addresses | Keep original MAC addresses |
| Preserve Port Groups | Maintain network configuration |
| Sync Interval | Minutes between syncs |
| Scheduled Cutover | Optional cutover time |

4. Click **Create Migration**

### Migration Statuses

| Status | Description |
|--------|-------------|
| Pending | Migration created, not started |
| Syncing | Initial or incremental sync in progress |
| Ready | Synced and ready for cutover |
| Cutting Over | Cutover in progress |
| Completed | Migration finished successfully |
| Failed | Migration encountered an error |
| Cancelled | Migration was cancelled |

### Manual Sync

To trigger an immediate sync:
1. Find the migration in the list
2. Click **Sync**
3. Monitor progress on the dashboard

### Executing Cutover

When ready to complete the migration:

1. Ensure the VM is in **Ready** status
2. Click **Cutover**
3. Confirm the action

**Cutover Process**:
1. Final sync to capture latest changes
2. Source VM powered off
3. Last incremental sync
4. Target VM powered on
5. Status changed to **Completed**

### Cancelling a Migration

To cancel an in-progress migration:
1. Click **Cancel** on the migration
2. Confirm the cancellation

**Note**: Cancelling stops syncs but doesn't affect the source VM.

---

## Scheduling Tasks

Schedule migrations for specific times.

### Task Types

| Type | Description |
|------|-------------|
| Cutover | Complete the migration |
| Failover | Emergency cutover |
| Sync | One-time sync |
| Test Failover | Non-destructive test |

### Creating a Scheduled Task

1. Navigate to **Scheduled Tasks**
2. Click **Schedule Task**
3. Configure:
   - Job ID (migration to act on)
   - Task type
   - Scheduled time
4. Click **Schedule**

### Managing Schedules

- View all scheduled tasks
- Cancel pending tasks
- Review completed task results

---

## Admin Portal

Admin features are available to users with administrator privileges.

### Environment Variables

Store configuration values securely:

1. Navigate to **Admin** → **Environment Variables**
2. Click **Add Variable**
3. Enter:
   - Name (uppercase, underscores)
   - Value
   - Description
   - Is Secret (masks the value)
4. Click **Save**

### User Management

View and manage users:

| Action | Description |
|--------|-------------|
| View Users | List all users who have logged in |
| Grant Admin | Promote user to administrator |
| Revoke Admin | Remove administrator privileges |

### Activity Logs

Monitor system activity:
- User actions
- Migration events
- System operations
- Timestamps and details

---

## Best Practices

### Planning Migrations

1. **Assess VMs First**
   - Check compatibility
   - Review size estimates
   - Identify dependencies

2. **Test with Non-Critical VMs**
   - Start with test/dev environments
   - Validate the process
   - Document issues

3. **Schedule During Maintenance Windows**
   - Plan cutovers for low-usage periods
   - Communicate with stakeholders
   - Have rollback plans

### Optimizing Sync Performance

1. **Initial Sync**
   - Schedule during off-hours
   - Consider VM size

2. **Incremental Syncs**
   - Adjust interval based on change rate
   - More frequent = smaller syncs
   - Less frequent = larger syncs

3. **Network Considerations**
   - Ensure adequate bandwidth
   - Consider WAN optimization

### Cutover Checklist

Before cutover:
- [ ] Final sync completed
- [ ] Stakeholders notified
- [ ] Rollback plan ready
- [ ] DNS/IP changes planned
- [ ] Application teams on standby

During cutover:
- [ ] Monitor progress
- [ ] Verify target VM starts
- [ ] Test application functionality
- [ ] Update DNS if needed

After cutover:
- [ ] Confirm services operational
- [ ] Monitor for 24-48 hours
- [ ] Document the migration
- [ ] Decommission source (when confident)

### Security Recommendations

1. **Change Default Credentials**
   - Use strong passwords
   - Enable AD authentication

2. **Secure API Access**
   - Use HTTPS in production
   - Rotate JWT secrets

3. **Network Security**
   - Restrict access to management ports
   - Use firewalls appropriately

4. **Audit Regularly**
   - Review activity logs
   - Monitor for unusual activity
