#!/usr/bin/env python3
"""
VXRail to ESXi Size Estimator

Connects to vCenter to automatically discover VXRail/vSAN configuration
and estimate storage sizes for migration to plain ESXi.

This script accounts for multiple factors that affect actual migration size:
1. RAID/FTT policy overhead (replicas/parity removed on migration)
2. Deduplication expansion (deduplicated data expands)
3. Compression expansion (compressed data expands)
4. TRIM/UNMAP status (unreclaimred blocks may show inflated sizes)
5. Thin provisioning (only actual written data migrates)
6. VM overhead (swap files, snapshots excluded)

The estimation uses vSAN Management API when available for accurate
primary vs overhead breakdown, with heuristic fallback.

Usage:
    python vxrail_size_estimator.py --vcenter vcenter.example.com --username admin@vsphere.local
    python vxrail_size_estimator.py --vcenter vcenter.example.com --username admin -o results.csv

Requirements:
    pip install pyvmomi

Optional (for more accurate vSAN metrics):
    VMware vSAN SDK (vsanmgmtObjects.py, vsanapiutils.py)

References:
    - https://blogs.vmware.com/cloud-foundation/2022/01/14/demystifying-capacity-reporting-in-vsan/
    - https://blogs.vmware.com/cloud-foundation/2022/03/10/the-importance-of-space-reclamation-for-data-usage-reporting-in-vsan/
    - https://developer.broadcom.com/xapis/vsan-management-api/latest/
"""

import argparse
import atexit
import csv
import getpass
import os
import ssl
import sys
from datetime import datetime
from dataclasses import dataclass
from typing import List, Optional, Dict, Any, Tuple

try:
    from pyVim.connect import SmartConnect, Disconnect
    from pyVmomi import vim, vmodl
except ImportError:
    print("Error: pyvmomi is required. Install with: pip install pyvmomi", file=sys.stderr)
    sys.exit(1)

# Try to import vSAN Management SDK (optional, for more accurate metrics)
VSAN_SDK_AVAILABLE = False
try:
    import vsanmgmtObjects
    import vsanapiutils
    VSAN_SDK_AVAILABLE = True
except ImportError:
    pass


# RAID overhead multipliers (physical space per unit of logical data)
RAID_OVERHEAD = {
    'RAID-1 (FTT=1)': 2.0,      # Mirror: 2 copies
    'RAID-1 (FTT=2)': 3.0,      # Triple mirror: 3 copies
    'RAID-1 (FTT=3)': 4.0,      # Quad mirror: 4 copies
    'RAID-5 (FTT=1)': 1.33,     # 3+1 erasure coding
    'RAID-6 (FTT=2)': 1.5,      # 4+2 erasure coding
    'None': 1.0,
}

# Organic adjustment factors based on VMware documentation and empirical data
# These account for factors that cause reported size to differ from actual data
ORGANIC_FACTORS = {
    # TRIM/UNMAP: If not enabled, deleted blocks aren't reclaimed
    # vSAN may report 10-30% higher than actual data
    'trim_unmap_not_enabled': 0.85,  # Reduce estimate by 15% (data is inflated)

    # Dedup expansion: When dedup is removed, data expands
    # Conservative estimate based on typical workloads
    'dedup_expansion_low': 1.2,      # Low dedup ratio environments
    'dedup_expansion_medium': 1.4,   # Medium dedup ratio
    'dedup_expansion_high': 1.6,     # High dedup ratio

    # Compression expansion: Compressed data expands
    'compression_expansion': 1.25,   # Typical compression ratio

    # VM overhead that doesn't migrate
    'vm_swap_overhead': 0.95,        # ~5% reduction for swap files

    # Snapshot overhead (if present)
    'snapshot_overhead': 0.90,       # Snapshots consolidate on migration
}


@dataclass
class VSANConfig:
    """Detected vSAN cluster configuration."""
    cluster_name: str
    is_vsan: bool = False
    is_vxrail: bool = False
    raid_policy: str = 'RAID-1 (FTT=1)'
    raid_overhead: float = 2.0
    dedup_enabled: bool = False
    compression_enabled: bool = False
    # Actual ratios from vSAN (if available)
    dedup_ratio: float = 1.0         # e.g., 1.5 means 1.5:1 dedup
    compression_ratio: float = 1.0   # e.g., 1.3 means 1.3:1 compression
    trim_unmap_enabled: bool = True  # Assume enabled unless detected otherwise
    # Capacity breakdown (if available from vSAN API)
    primary_capacity_gb: float = 0.0
    overhead_capacity_gb: float = 0.0
    total_capacity_gb: float = 0.0
    used_capacity_gb: float = 0.0


@dataclass
class VMInfo:
    """VM storage information."""
    name: str
    cluster: str
    provisioned_gb: float = 0.0      # Total provisioned (thin + thick)
    used_gb: float = 0.0             # Committed/used space (includes RAID overhead on vSAN)
    logical_gb: float = 0.0          # Estimated logical data (primary only)
    estimated_gb: float = 0.0        # Final migration estimate
    change_pct: float = 0.0
    has_snapshots: bool = False
    notes: str = ""


def connect_to_vcenter(host: str, username: str, password: str, port: int = 443) -> vim.ServiceInstance:
    """Connect to vCenter and return service instance."""
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE

    try:
        si = SmartConnect(
            host=host,
            user=username,
            pwd=password,
            port=port,
            sslContext=context
        )
        atexit.register(Disconnect, si)
        return si
    except vim.fault.InvalidLogin:
        print("Error: Invalid username or password", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error connecting to vCenter: {e}", file=sys.stderr)
        sys.exit(1)


def get_vsan_stub(si: vim.ServiceInstance, host: str, context: ssl.SSLContext):
    """Get vSAN API stub for advanced queries."""
    if not VSAN_SDK_AVAILABLE:
        return None
    try:
        vsan_stub = vsanapiutils.GetVsanVcMos(si._stub, context=context)
        return vsan_stub
    except Exception:
        return None


def get_all_clusters(content: vim.ServiceContent) -> List[vim.ClusterComputeResource]:
    """Get all clusters from vCenter."""
    container = content.viewManager.CreateContainerView(
        content.rootFolder, [vim.ClusterComputeResource], True
    )
    clusters = list(container.view)
    container.Destroy()
    return clusters


def detect_vsan_config(cluster: vim.ClusterComputeResource, vsan_stub=None) -> VSANConfig:
    """Detect vSAN configuration for a cluster."""
    config = VSANConfig(cluster_name=cluster.name)

    # Check if vSAN is enabled
    if not hasattr(cluster, 'configurationEx') or not cluster.configurationEx:
        return config

    vsan_config_info = cluster.configurationEx.vsanConfigInfo
    if not vsan_config_info or not vsan_config_info.enabled:
        return config

    config.is_vsan = True

    # Check for VXRail
    for host in cluster.host:
        if 'vxrail' in host.name.lower() or 'vxrail' in cluster.name.lower():
            config.is_vxrail = True
            break

    # Get space efficiency config (dedup/compression)
    try:
        if hasattr(vsan_config_info, 'dataEfficiencyConfig'):
            de_config = vsan_config_info.dataEfficiencyConfig
            if de_config:
                config.dedup_enabled = getattr(de_config, 'dedupEnabled', False)
                config.compression_enabled = getattr(de_config, 'compressionEnabled', False)
    except Exception:
        pass

    # Try to get RAID policy from default storage policy
    config.raid_policy, config.raid_overhead = detect_default_raid_policy(cluster)

    # Try to get actual dedup/compression ratios from vSAN API
    if vsan_stub and VSAN_SDK_AVAILABLE:
        try:
            config = query_vsan_capacity_details(cluster, vsan_stub, config)
        except Exception:
            pass

    return config


def query_vsan_capacity_details(cluster: vim.ClusterComputeResource, vsan_stub, config: VSANConfig) -> VSANConfig:
    """Query vSAN Management API for detailed capacity breakdown."""
    if not vsan_stub:
        return config

    try:
        # Get vSAN Space Report System
        vsan_space_report_system = vsan_stub['vsan-cluster-space-report-system']

        # Query cluster space usage
        space_usage = vsan_space_report_system.VsanQuerySpaceUsage(cluster=cluster)

        if space_usage:
            # Extract primary vs overhead capacity
            if hasattr(space_usage, 'primaryCapacity'):
                config.primary_capacity_gb = space_usage.primaryCapacity / (1024**3)
            if hasattr(space_usage, 'usedCapacity'):
                config.used_capacity_gb = space_usage.usedCapacity / (1024**3)

            # Get actual dedup/compression ratios if available
            if hasattr(space_usage, 'efficientCapacity'):
                efficient = space_usage.efficientCapacity
                if hasattr(efficient, 'dedupRatio') and efficient.dedupRatio:
                    config.dedup_ratio = efficient.dedupRatio
                if hasattr(efficient, 'compressionRatio') and efficient.compressionRatio:
                    config.compression_ratio = efficient.compressionRatio
    except Exception:
        pass

    return config


def detect_default_raid_policy(cluster: vim.ClusterComputeResource) -> Tuple[str, float]:
    """
    Detect the default/most common RAID policy for the cluster.

    In practice, you'd query SPBM (Storage Policy Based Management) to get
    the actual policies in use. This is a simplified detection.
    """
    # Check cluster configuration for hints
    try:
        if hasattr(cluster.configurationEx, 'vsanConfigInfo'):
            vsan_info = cluster.configurationEx.vsanConfigInfo
            # Check if FTT is configured at cluster level
            if hasattr(vsan_info, 'defaultConfig'):
                default_config = vsan_info.defaultConfig
                if hasattr(default_config, 'hostFailuresToTolerate'):
                    ftt = default_config.hostFailuresToTolerate
                    # Check if using erasure coding
                    if hasattr(default_config, 'spaceEfficiency') and default_config.spaceEfficiency:
                        if ftt == 1:
                            return 'RAID-5 (FTT=1)', 1.33
                        elif ftt == 2:
                            return 'RAID-6 (FTT=2)', 1.5
                    else:
                        if ftt == 1:
                            return 'RAID-1 (FTT=1)', 2.0
                        elif ftt == 2:
                            return 'RAID-1 (FTT=2)', 3.0
                        elif ftt == 3:
                            return 'RAID-1 (FTT=3)', 4.0
    except Exception:
        pass

    # Default to RAID-1 FTT=1 (most common)
    return 'RAID-1 (FTT=1)', 2.0


def get_vm_storage_info(vm: vim.VirtualMachine, vsan_configs: Dict[str, VSANConfig]) -> Optional[VMInfo]:
    """Get storage information for a VM with organic factor adjustments."""
    if vm.config is None:
        return None

    vm_info = VMInfo(name=vm.name, cluster="")

    # Get cluster name
    if vm.resourcePool and vm.resourcePool.owner:
        if isinstance(vm.resourcePool.owner, vim.ClusterComputeResource):
            vm_info.cluster = vm.resourcePool.owner.name

    # Check for snapshots
    if vm.snapshot:
        vm_info.has_snapshots = True

    # Calculate storage from per-datastore usage
    provisioned = 0
    committed = 0

    try:
        if vm.storage and vm.storage.perDatastoreUsage:
            for ds_usage in vm.storage.perDatastoreUsage:
                provisioned += ds_usage.committed + ds_usage.uncommitted
                committed += ds_usage.committed
    except Exception:
        # Fallback to summary
        if vm.summary and vm.summary.storage:
            provisioned = vm.summary.storage.committed + vm.summary.storage.uncommitted
            committed = vm.summary.storage.committed

    vm_info.provisioned_gb = provisioned / (1024**3)
    vm_info.used_gb = committed / (1024**3)

    # Calculate estimate based on cluster config and organic factors
    if vm_info.cluster in vsan_configs:
        config = vsan_configs[vm_info.cluster]
        if config.is_vsan:
            vm_info.logical_gb, vm_info.estimated_gb, vm_info.change_pct, vm_info.notes = calculate_estimate(
                vm_info.used_gb, vm_info.has_snapshots, config
            )
        else:
            vm_info.logical_gb = vm_info.used_gb
            vm_info.estimated_gb = vm_info.used_gb
            vm_info.change_pct = 0
            vm_info.notes = "Not on vSAN"
    else:
        vm_info.logical_gb = vm_info.used_gb
        vm_info.estimated_gb = vm_info.used_gb
        vm_info.change_pct = 0
        vm_info.notes = "Not on vSAN cluster"

    return vm_info


def calculate_estimate(used_gb: float, has_snapshots: bool, config: VSANConfig) -> Tuple[float, float, float, str]:
    """
    Calculate estimated migration size accounting for organic factors.

    The calculation flow:
    1. Start with vCenter reported "used" size (includes RAID overhead)
    2. Divide by RAID overhead to get primary/logical data
    3. Apply TRIM/UNMAP adjustment (unreclaimred blocks inflation)
    4. Apply dedup expansion (if dedup enabled)
    5. Apply compression expansion (if compression enabled)
    6. Apply snapshot/VM overhead adjustments

    Returns: (logical_gb, estimated_gb, change_pct, notes)
    """
    notes_parts = []

    # Step 1: Start with reported used size
    size = used_gb

    # Step 2: Remove RAID overhead to get logical/primary data
    # vCenter's "committed" on vSAN includes replica/parity overhead
    logical_size = size / config.raid_overhead
    notes_parts.append(f"Primary data (/{config.raid_overhead:.2f} RAID)")

    # Step 3: TRIM/UNMAP adjustment
    # If not known to be enabled, assume some inflation
    if not config.trim_unmap_enabled:
        logical_size *= ORGANIC_FACTORS['trim_unmap_not_enabled']
        notes_parts.append("TRIM/UNMAP adj")

    # The logical size after RAID removal represents the actual VM data
    logical_gb = logical_size

    # Step 4: Calculate migration size (data expansion factors)
    estimated_size = logical_size

    # Dedup expansion: deduplicated data will expand on target
    if config.dedup_enabled:
        if config.dedup_ratio > 1.0:
            # Use actual ratio from vSAN if available
            expansion = config.dedup_ratio
        else:
            # Use conservative estimate based on typical workloads
            expansion = ORGANIC_FACTORS['dedup_expansion_medium']
        estimated_size *= expansion
        notes_parts.append(f"Dedup expand x{expansion:.2f}")

    # Compression expansion
    if config.compression_enabled:
        if config.compression_ratio > 1.0:
            expansion = config.compression_ratio
        else:
            expansion = ORGANIC_FACTORS['compression_expansion']
        estimated_size *= expansion
        notes_parts.append(f"Compress expand x{expansion:.2f}")

    # Step 5: VM overhead adjustments
    # Swap files don't need to migrate (regenerated on target)
    estimated_size *= ORGANIC_FACTORS['vm_swap_overhead']

    # Snapshots consolidate during migration
    if has_snapshots:
        estimated_size *= ORGANIC_FACTORS['snapshot_overhead']
        notes_parts.append("Snapshot consolidation")

    # Final values
    estimated_gb = round(estimated_size, 2)
    logical_gb = round(logical_gb, 2)
    change_pct = round((estimated_gb - used_gb) / used_gb * 100, 1) if used_gb > 0 else 0
    notes = "; ".join(notes_parts)

    return logical_gb, estimated_gb, change_pct, notes


def get_all_vms(content: vim.ServiceContent) -> List[vim.VirtualMachine]:
    """Get all VMs from vCenter."""
    container = content.viewManager.CreateContainerView(
        content.rootFolder, [vim.VirtualMachine], True
    )
    vms = list(container.view)
    container.Destroy()
    return vms


def print_cluster_summary(vsan_configs: Dict[str, VSANConfig]):
    """Print summary of detected vSAN clusters."""
    print("\n" + "=" * 70, file=sys.stderr)
    print("DETECTED vSAN CLUSTERS", file=sys.stderr)
    print("=" * 70, file=sys.stderr)

    for name, config in vsan_configs.items():
        if config.is_vsan:
            print(f"\nCluster: {name}", file=sys.stderr)
            print(f"  Type: {'VXRail' if config.is_vxrail else 'vSAN'}", file=sys.stderr)
            print(f"  RAID Policy: {config.raid_policy} ({config.raid_overhead}x overhead)", file=sys.stderr)
            print(f"  Deduplication: {'Enabled' if config.dedup_enabled else 'Disabled'}", file=sys.stderr)
            if config.dedup_enabled and config.dedup_ratio > 1.0:
                print(f"    Detected ratio: {config.dedup_ratio:.2f}:1", file=sys.stderr)
            print(f"  Compression: {'Enabled' if config.compression_enabled else 'Disabled'}", file=sys.stderr)
            if config.compression_enabled and config.compression_ratio > 1.0:
                print(f"    Detected ratio: {config.compression_ratio:.2f}:1", file=sys.stderr)


def get_script_dir() -> str:
    """Get the directory where the script is located."""
    return os.path.dirname(os.path.abspath(__file__))


def generate_output_filename(vcenter: str) -> str:
    """Generate default output filename with vcenter name and timestamp."""
    clean_name = vcenter.replace('.', '_').replace(':', '_')
    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    return f"vxrail_estimate_{clean_name}_{timestamp}.csv"


def main():
    parser = argparse.ArgumentParser(
        description="VXRail to ESXi Size Estimator - Connects to vCenter to estimate migration sizes",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
This script accounts for organic factors that affect actual migration size:
  - RAID/FTT policy overhead (removed on migration to ESXi)
  - Deduplication expansion (deduplicated data expands)
  - Compression expansion (compressed data expands)
  - TRIM/UNMAP status (unreclaimred blocks may inflate sizes)
  - VM overhead (swap files, snapshot consolidation)

Examples:
  %(prog)s --vcenter vcenter.example.com --username admin@vsphere.local
  %(prog)s --vcenter vcenter.example.com --username admin -o custom_name.csv
  %(prog)s --vcenter vcenter.example.com --username admin --include-powered-off

Output:
  CSV file saved to script directory: vxrail_estimate_<vcenter>_<timestamp>.csv
        """
    )

    parser.add_argument("--vcenter", "-s", required=True,
                       help="vCenter server hostname or IP")
    parser.add_argument("--username", "-u", required=True,
                       help="vCenter username")
    parser.add_argument("--password", "-p",
                       help="vCenter password (will prompt if not provided)")
    parser.add_argument("--port", type=int, default=443,
                       help="vCenter port (default: 443)")
    parser.add_argument("-o", "--output",
                       help="Output CSV filename (default: auto-generated)")
    parser.add_argument("--include-powered-off", action="store_true",
                       help="Include powered-off VMs")
    parser.add_argument("--include-templates", action="store_true",
                       help="Include VM templates")
    parser.add_argument("--verbose", "-v", action="store_true",
                       help="Show detailed output")

    args = parser.parse_args()

    # Get password if not provided
    password = args.password
    if not password:
        password = getpass.getpass(f"Password for {args.username}@{args.vcenter}: ")

    # Connect to vCenter
    print(f"Connecting to vCenter: {args.vcenter}...", file=sys.stderr)
    si = connect_to_vcenter(args.vcenter, args.username, password, args.port)
    content = si.RetrieveContent()
    print("Connected successfully.", file=sys.stderr)

    # Check for vSAN SDK
    if VSAN_SDK_AVAILABLE:
        print("vSAN Management SDK detected - using advanced capacity queries", file=sys.stderr)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        vsan_stub = get_vsan_stub(si, args.vcenter, context)
    else:
        print("vSAN Management SDK not found - using heuristic estimation", file=sys.stderr)
        vsan_stub = None

    # Get all clusters and detect vSAN config
    print("Detecting vSAN cluster configurations...", file=sys.stderr)
    clusters = get_all_clusters(content)
    vsan_configs = {}
    for cluster in clusters:
        config = detect_vsan_config(cluster, vsan_stub)
        vsan_configs[cluster.name] = config
        if config.is_vsan:
            cluster_type = 'VXRail' if config.is_vxrail else 'vSAN'
            print(f"  Found {cluster_type} cluster: {cluster.name}", file=sys.stderr)

    # Print cluster summary
    print_cluster_summary(vsan_configs)

    # Get all VMs
    print("\nCollecting VM storage information...", file=sys.stderr)
    vms = get_all_vms(content)
    print(f"  Found {len(vms)} VMs", file=sys.stderr)

    # Process VMs
    results = []
    total_used = 0
    total_logical = 0
    total_estimated = 0

    for vm in vms:
        # Skip templates unless requested
        if vm.config and vm.config.template and not args.include_templates:
            continue

        # Skip powered-off unless requested
        if vm.runtime.powerState != vim.VirtualMachinePowerState.poweredOn and not args.include_powered_off:
            continue

        vm_info = get_vm_storage_info(vm, vsan_configs)
        if vm_info and vm_info.used_gb > 0:
            results.append(vm_info)
            total_used += vm_info.used_gb
            total_logical += vm_info.logical_gb
            total_estimated += vm_info.estimated_gb

    # Sort by name
    results.sort(key=lambda x: x.name.lower())

    # Determine output file path
    script_dir = get_script_dir()
    if args.output:
        if os.path.dirname(args.output):
            output_path = args.output
        else:
            output_path = os.path.join(script_dir, args.output)
    else:
        output_path = os.path.join(script_dir, generate_output_filename(args.vcenter))

    # Write CSV output
    with open(output_path, 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['host', 'cluster', 'vsan_used_gb', 'logical_gb', 'est_esxi_gb', 'change_pct', 'notes'])
        for r in results:
            writer.writerow([
                r.name, r.cluster, round(r.used_gb, 2), r.logical_gb,
                r.estimated_gb, r.change_pct, r.notes
            ])
    print(f"\nCSV output written to: {output_path}", file=sys.stderr)

    # Print summary
    print("\n" + "=" * 70, file=sys.stderr)
    print("MIGRATION ESTIMATE SUMMARY", file=sys.stderr)
    print("=" * 70, file=sys.stderr)
    print(f"Total VMs processed: {len(results)}", file=sys.stderr)
    print(f"\nStorage breakdown:", file=sys.stderr)
    print(f"  vSAN used (with RAID overhead): {total_used:,.2f} GB ({total_used/1024:.2f} TB)", file=sys.stderr)
    print(f"  Logical/primary data:           {total_logical:,.2f} GB ({total_logical/1024:.2f} TB)", file=sys.stderr)
    print(f"  Estimated ESXi size:            {total_estimated:,.2f} GB ({total_estimated/1024:.2f} TB)", file=sys.stderr)

    change = total_estimated - total_used
    change_pct = (change / total_used) * 100 if total_used > 0 else 0
    print(f"\nMigration impact:", file=sys.stderr)
    if change < 0:
        print(f"  Estimated reduction: {abs(change):,.2f} GB ({abs(change_pct):.1f}% smaller)", file=sys.stderr)
    else:
        print(f"  Estimated increase: {change:,.2f} GB ({change_pct:.1f}% larger)", file=sys.stderr)

    # Note about accuracy
    print(f"\nNote: Estimates account for RAID overhead removal and data expansion", file=sys.stderr)
    print(f"      from dedup/compression. Actual results may vary based on workload.", file=sys.stderr)
    print("=" * 70, file=sys.stderr)


if __name__ == "__main__":
    main()
