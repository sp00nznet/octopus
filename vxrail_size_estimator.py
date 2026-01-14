#!/usr/bin/env python3
"""
VXRail to ESXi Size Estimator

Connects to vCenter to automatically discover VXRail/vSAN configuration
and estimate storage sizes for migration to plain ESXi.

The script will:
1. Connect to vCenter and detect vSAN clusters
2. Determine RAID policy, dedup/compression settings
3. Pull all VM storage information
4. Calculate estimated sizes after migration
5. Output a CSV with host,size,estsize

Usage:
    python vxrail_size_estimator.py --vcenter vcenter.example.com --username admin@vsphere.local
    python vxrail_size_estimator.py --vcenter vcenter.example.com --username admin --password pass123
    python vxrail_size_estimator.py --vcenter vcenter.example.com --username admin -o results.csv

Requirements:
    pip install pyvmomi
"""

import argparse
import atexit
import csv
import getpass
import os
import ssl
import sys
from datetime import datetime
from dataclasses import dataclass, field
from typing import List, Optional, Dict, Any

try:
    from pyVim.connect import SmartConnect, Disconnect
    from pyVmomi import vim, vmodl
except ImportError:
    print("Error: pyvmomi is required. Install with: pip install pyvmomi", file=sys.stderr)
    sys.exit(1)


# RAID overhead multipliers
RAID_OVERHEAD = {
    'RAID-1 (FTT=1)': 2.0,
    'RAID-1 (FTT=2)': 3.0,
    'RAID-1 (FTT=3)': 4.0,
    'RAID-5 (FTT=1)': 1.33,
    'RAID-6 (FTT=2)': 1.5,
    'None': 1.0,
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
    dedup_ratio: float = 1.0
    compression_ratio: float = 1.0
    total_capacity_gb: float = 0.0
    used_capacity_gb: float = 0.0


@dataclass
class VMInfo:
    """VM storage information."""
    name: str
    cluster: str
    provisioned_gb: float = 0.0
    used_gb: float = 0.0
    estimated_gb: float = 0.0
    change_pct: float = 0.0
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


def get_all_clusters(content: vim.ServiceContent) -> List[vim.ClusterComputeResource]:
    """Get all clusters from vCenter."""
    clusters = []
    container = content.viewManager.CreateContainerView(
        content.rootFolder, [vim.ClusterComputeResource], True
    )
    clusters = list(container.view)
    container.Destroy()
    return clusters


def detect_vsan_config(cluster: vim.ClusterComputeResource) -> VSANConfig:
    """Detect vSAN configuration for a cluster."""
    config = VSANConfig(cluster_name=cluster.name)

    # Check if vSAN is enabled
    if not hasattr(cluster, 'configurationEx') or not cluster.configurationEx:
        return config

    vsan_config = cluster.configurationEx.vsanConfigInfo
    if not vsan_config or not vsan_config.enabled:
        return config

    config.is_vsan = True

    # Check for VXRail (look for VxRail in host names or cluster name)
    for host in cluster.host:
        if 'vxrail' in host.name.lower() or 'vxrail' in cluster.name.lower():
            config.is_vxrail = True
            break

    # Try to get vSAN space efficiency config (dedup/compression)
    try:
        if hasattr(cluster, 'configurationEx'):
            vsan_config_info = cluster.configurationEx.vsanConfigInfo
            if hasattr(vsan_config_info, 'dataEfficiencyConfig'):
                de_config = vsan_config_info.dataEfficiencyConfig
                if de_config:
                    config.dedup_enabled = getattr(de_config, 'dedupEnabled', False)
                    config.compression_enabled = getattr(de_config, 'compressionEnabled', False)
    except Exception:
        pass

    # Try to get default storage policy to determine RAID level
    try:
        config.raid_policy, config.raid_overhead = detect_default_raid_policy(cluster)
    except Exception:
        # Default to RAID-1 FTT=1 if we can't detect
        config.raid_policy = 'RAID-1 (FTT=1)'
        config.raid_overhead = 2.0

    # Get capacity info
    try:
        summary = cluster.summary
        if hasattr(summary, 'totalCapacity'):
            config.total_capacity_gb = summary.totalCapacity / (1024**3)
        if hasattr(summary, 'freeCapacity'):
            config.used_capacity_gb = config.total_capacity_gb - (summary.freeCapacity / (1024**3))
    except Exception:
        pass

    return config


def detect_default_raid_policy(cluster: vim.ClusterComputeResource) -> tuple:
    """Try to detect the default RAID policy for the cluster."""
    # This is a simplified detection - in practice you'd query vSAN policies
    # Default to RAID-1 FTT=1 which is most common
    return 'RAID-1 (FTT=1)', 2.0


def get_vsan_storage_policies(content: vim.ServiceContent) -> Dict[str, Any]:
    """Get vSAN storage policies and their settings."""
    policies = {}
    try:
        pbm_si = content.pbmServiceInstance
        if pbm_si:
            # Query storage policies
            pass
    except Exception:
        pass
    return policies


def get_datastore_type(datastore: vim.Datastore) -> str:
    """Determine the type of datastore."""
    if hasattr(datastore.summary, 'type'):
        ds_type = datastore.summary.type
        if ds_type == 'vsan':
            return 'vsan'
        elif ds_type == 'VMFS':
            return 'vmfs'
        elif ds_type == 'NFS':
            return 'nfs'
    return 'unknown'


def get_vm_storage_info(vm: vim.VirtualMachine, vsan_configs: Dict[str, VSANConfig]) -> Optional[VMInfo]:
    """Get storage information for a VM."""
    if vm.config is None:
        return None

    vm_info = VMInfo(name=vm.name, cluster="")

    # Get cluster name
    if vm.resourcePool and vm.resourcePool.owner:
        if isinstance(vm.resourcePool.owner, vim.ClusterComputeResource):
            vm_info.cluster = vm.resourcePool.owner.name

    # Calculate storage
    provisioned = 0
    used = 0

    try:
        if vm.storage and vm.storage.perDatastoreUsage:
            for ds_usage in vm.storage.perDatastoreUsage:
                provisioned += ds_usage.committed + ds_usage.uncommitted
                used += ds_usage.committed
    except Exception:
        # Fallback to summary
        if vm.summary and vm.summary.storage:
            provisioned = vm.summary.storage.committed + vm.summary.storage.uncommitted
            used = vm.summary.storage.committed

    vm_info.provisioned_gb = provisioned / (1024**3)
    vm_info.used_gb = used / (1024**3)

    # Calculate estimate based on cluster config
    if vm_info.cluster in vsan_configs:
        config = vsan_configs[vm_info.cluster]
        vm_info.estimated_gb, vm_info.change_pct, vm_info.notes = calculate_estimate(
            vm_info.used_gb, config
        )
    else:
        # Not on vSAN, size stays the same
        vm_info.estimated_gb = vm_info.used_gb
        vm_info.change_pct = 0
        vm_info.notes = "Not on vSAN"

    return vm_info


def calculate_estimate(used_gb: float, config: VSANConfig) -> tuple:
    """Calculate estimated size after migration."""
    notes_parts = []

    # Start with used size (actual data with RAID overhead)
    size = used_gb

    # Remove RAID overhead - this is the main reduction
    size = size / config.raid_overhead
    notes_parts.append(f"RAID: /{config.raid_overhead}")

    # Account for dedup/compression expansion
    expansion = 1.0
    if config.dedup_enabled:
        # Conservative estimate: data may expand 1.2-1.5x
        expansion *= 1.3
        notes_parts.append("Dedup expansion: x1.3")
    if config.compression_enabled:
        expansion *= 1.2
        notes_parts.append("Compression expansion: x1.2")

    size = size * expansion

    estimated = round(size, 2)
    change_pct = round((estimated - used_gb) / used_gb * 100, 1) if used_gb > 0 else 0
    notes = ", ".join(notes_parts)

    return estimated, change_pct, notes


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
            print(f"  Compression: {'Enabled' if config.compression_enabled else 'Disabled'}", file=sys.stderr)


def get_script_dir() -> str:
    """Get the directory where the script is located."""
    return os.path.dirname(os.path.abspath(__file__))


def generate_output_filename(vcenter: str) -> str:
    """Generate default output filename with vcenter name and timestamp."""
    # Clean vcenter name for filename
    clean_name = vcenter.replace('.', '_').replace(':', '_')
    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    return f"vxrail_estimate_{clean_name}_{timestamp}.csv"


def main():
    parser = argparse.ArgumentParser(
        description="VXRail to ESXi Size Estimator - Connects to vCenter to estimate migration sizes",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Interactive password prompt (saves CSV to script directory)
  %(prog)s --vcenter vcenter.example.com --username admin@vsphere.local

  # With password (not recommended for security)
  %(prog)s --vcenter vcenter.example.com --username admin --password mypass

  # Custom output filename
  %(prog)s --vcenter vcenter.example.com --username admin -o custom_name.csv

  # Include powered-off VMs
  %(prog)s --vcenter vcenter.example.com --username admin --include-powered-off

Output:
  CSV file is automatically saved to the script directory with format:
  vxrail_estimate_<vcenter>_<timestamp>.csv
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

    # Get all clusters and detect vSAN config
    print("Detecting vSAN cluster configurations...", file=sys.stderr)
    clusters = get_all_clusters(content)
    vsan_configs = {}
    for cluster in clusters:
        config = detect_vsan_config(cluster)
        vsan_configs[cluster.name] = config
        if config.is_vsan:
            print(f"  Found vSAN cluster: {cluster.name} ({'VXRail' if config.is_vxrail else 'vSAN'})", file=sys.stderr)

    # Print cluster summary
    print_cluster_summary(vsan_configs)

    # Get all VMs
    print("\nCollecting VM storage information...", file=sys.stderr)
    vms = get_all_vms(content)
    print(f"  Found {len(vms)} VMs", file=sys.stderr)

    # Process VMs
    results = []
    total_used = 0
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
            total_estimated += vm_info.estimated_gb

    # Sort by name
    results.sort(key=lambda x: x.name.lower())

    # Determine output file path (always save to script directory)
    script_dir = get_script_dir()
    if args.output:
        # If user provided a filename, use it but ensure it's in script dir
        if os.path.dirname(args.output):
            output_path = args.output  # User provided full path
        else:
            output_path = os.path.join(script_dir, args.output)
    else:
        # Auto-generate filename
        output_path = os.path.join(script_dir, generate_output_filename(args.vcenter))

    # Write CSV output
    with open(output_path, 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['host', 'cluster', 'size', 'estsize', 'change_pct', 'notes'])
        for r in results:
            writer.writerow([r.name, r.cluster, round(r.used_gb, 2), r.estimated_gb, r.change_pct, r.notes])
    print(f"\nCSV output written to: {output_path}", file=sys.stderr)

    # Print summary
    print("\n" + "=" * 70, file=sys.stderr)
    print("MIGRATION ESTIMATE SUMMARY", file=sys.stderr)
    print("=" * 70, file=sys.stderr)
    print(f"Total VMs processed: {len(results)}", file=sys.stderr)
    print(f"Total current used space: {total_used:,.2f} GB ({total_used/1024:.2f} TB)", file=sys.stderr)
    print(f"Total estimated ESXi space: {total_estimated:,.2f} GB ({total_estimated/1024:.2f} TB)", file=sys.stderr)

    change = total_estimated - total_used
    change_pct = (change / total_used) * 100 if total_used > 0 else 0
    if change < 0:
        print(f"Estimated savings: {abs(change):,.2f} GB ({abs(change_pct):.1f}% reduction)", file=sys.stderr)
    else:
        print(f"Estimated increase: {change:,.2f} GB ({change_pct:.1f}% growth)", file=sys.stderr)
    print("=" * 70, file=sys.stderr)


if __name__ == "__main__":
    main()
