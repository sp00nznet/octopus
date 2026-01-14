# VXRail/vSAN Size Estimation Guide

This guide explains how Octopus estimates storage sizes when migrating from VXRail/vSAN to plain ESXi or cloud targets.

## Overview

When migrating from VXRail/vSAN, the reported storage size in vCenter includes overhead that won't be present on the target. Understanding these factors is critical for accurate capacity planning.

## Why Sizes Change

### vSAN Reports Physical Space, Not Logical

vCenter's "used space" for VMs on vSAN includes:
- **RAID overhead** - Replicas (RAID-1) or parity (RAID-5/6)
- **vSAN metadata** - Object tracking and management
- **Deduplication effects** - Logical data may be larger than physical
- **Compression effects** - Logical data may be larger than physical

When you migrate to plain ESXi, only the **logical/primary data** is copied.

## Estimation Factors

### 1. RAID/FTT Policy Overhead

The biggest factor affecting size change. vSAN stores multiple copies or parity data.

| Policy | Overhead Multiplier | Reduction on Migration | Description |
|--------|--------------------|-----------------------|-------------|
| RAID-1 FTT=1 | 2.0x | 50% smaller | Full mirror (2 copies) |
| RAID-1 FTT=2 | 3.0x | 67% smaller | Triple mirror (3 copies) |
| RAID-1 FTT=3 | 4.0x | 75% smaller | Quad mirror (4 copies) |
| RAID-5 FTT=1 | 1.33x | 25% smaller | 3+1 erasure coding |
| RAID-6 FTT=2 | 1.5x | 33% smaller | 4+2 erasure coding |

**Example:** A VM showing 200 GB on RAID-1 FTT=1 has only 100 GB of actual data.

### 2. Deduplication Expansion

If deduplication is enabled on your vSAN cluster, the stored data is smaller than the logical data. When migrated, data expands back to its original size.

| Dedup Ratio | Expansion on Migration |
|-------------|------------------------|
| 1.2:1 | Data increases 20% |
| 1.4:1 | Data increases 40% |
| 1.6:1 | Data increases 60% |

**Example:** 100 GB of deduplicated data at 1.5:1 becomes 150 GB on the target.

### 3. Compression Expansion

Similar to deduplication, compressed data expands when migrated.

| Compression Ratio | Expansion on Migration |
|-------------------|------------------------|
| 1.25:1 | Data increases 25% |
| 1.5:1 | Data increases 50% |

### 4. VM Overhead Reduction

Some VM components don't migrate:

| Component | Reduction | Reason |
|-----------|-----------|--------|
| Swap files | ~5% | Regenerated on target |
| Snapshots | ~10% | Consolidated during migration |

### 5. TRIM/UNMAP Status

If TRIM/UNMAP is not enabled, vSAN may report deleted blocks as still in use. This inflates the reported size by 10-30%.

## Calculation Flow

```
vSAN Reported Size (e.g., 200 GB)
    │
    ├── ÷ RAID Overhead (e.g., ÷2.0 for RAID-1)
    │   = 100 GB (Logical Data)
    │
    ├── × Dedup Expansion (e.g., ×1.4)
    │   = 140 GB
    │
    ├── × Compression Expansion (e.g., ×1.25)
    │   = 175 GB
    │
    ├── × VM Swap Overhead (×0.95)
    │   = 166.25 GB
    │
    └── × Snapshot Consolidation (×0.90, if applicable)
        = 149.6 GB (Final Estimate)
```

## Using the Web Interface

1. Navigate to the **VMs** section
2. Click the **Estimate Size** button for any VM
3. The modal shows:
   - **vSAN Reported Size** - What vCenter reports
   - **Logical/Primary Data** - After RAID overhead removal
   - **Estimated Migration Size** - Final estimate
   - **Factors Applied** - Explanation of adjustments

### Batch Estimation

For VXRail environments with many VMs:

1. Click **Batch Estimate** in the VMs section
2. Select your VXRail source environment
3. Select VMs to estimate
4. Choose target environment/type
5. View results and export to CSV

## Using the Standalone Script

For the most accurate estimates, use the standalone Python script that connects directly to vCenter:

### Installation

```bash
pip install pyvmomi
```

### Basic Usage

```bash
# Interactive (prompts for password)
python vxrail_size_estimator.py \
    --vcenter vcenter.example.com \
    --username admin@vsphere.local

# With password (not recommended for scripts)
python vxrail_size_estimator.py \
    --vcenter vcenter.example.com \
    --username admin@vsphere.local \
    --password 'yourpassword'

# Include powered-off VMs and templates
python vxrail_size_estimator.py \
    --vcenter vcenter.example.com \
    --username admin@vsphere.local \
    --include-powered-off \
    --include-templates
```

### Output

The script generates a CSV file in the script directory:
```
vxrail_estimate_<vcenter>_<timestamp>.csv
```

Columns:
| Column | Description |
|--------|-------------|
| `host` | VM name |
| `cluster` | vSAN cluster name |
| `vsan_used_gb` | Size reported by vCenter |
| `logical_gb` | Logical/primary data size |
| `est_esxi_gb` | Estimated migration size |
| `change_pct` | Percentage change |
| `notes` | Factors applied |

### What the Script Detects

- vSAN cluster membership
- RAID policy (FTT level, mirroring vs erasure coding)
- Deduplication enabled/disabled (and actual ratio if available)
- Compression enabled/disabled (and actual ratio if available)
- VM snapshot status

### Optional: vSAN Management SDK

For more accurate ratios, install the VMware vSAN SDK:
1. Download from [VMware Developer](https://developer.vmware.com/web/sdk/6.7.0/vsan-python)
2. Place `vsanmgmtObjects.py` and `vsanapiutils.py` in the script directory
3. The script will automatically use the SDK for detailed capacity queries

## API Integration

To estimate sizes programmatically:

```bash
curl -X POST http://localhost:8080/api/v1/vms/{vm_id}/estimate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "target_type": "vmware",
    "is_vxrail": true,
    "raid_policy": "raid1_ftt1",
    "dedup_enabled": true,
    "compression_enabled": true,
    "dedup_ratio": 1.5,
    "compression_ratio": 1.25,
    "has_snapshots": false
  }'
```

See [API Reference](api-reference.md#estimate-vm-size) for full details.

## Best Practices

1. **Use the standalone script** for accurate vSAN-aware estimates before large migrations
2. **Know your RAID policy** - Check vSAN settings in vCenter
3. **Check dedup/compression** - These significantly affect estimates
4. **Include buffer space** - Add 10-20% to estimates for safety
5. **Validate with test migrations** - Run a few VMs first to verify estimates

## Troubleshooting

### Estimates Don't Match Actual Migration

- **Check RAID policy**: Different VMs may use different storage policies
- **Verify dedup/compression**: Settings may vary by cluster
- **Account for thick provisioning**: Some VMs may have reserved space

### Script Connection Errors

- Verify vCenter hostname and credentials
- Check network connectivity
- Ensure user has read permissions on VMs and clusters

## References

- [Demystifying Capacity Reporting in vSAN](https://blogs.vmware.com/cloud-foundation/2022/01/14/demystifying-capacity-reporting-in-vsan/)
- [Space Reclamation in vSAN](https://blogs.vmware.com/cloud-foundation/2022/03/10/the-importance-of-space-reclamation-for-data-usage-reporting-in-vsan/)
- [vSAN Management API Reference](https://developer.broadcom.com/xapis/vsan-management-api/latest/)
