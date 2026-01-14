#!/usr/bin/env python3
"""
VXRail to VMware/Cloud Size Estimation Script

This script estimates VM disk sizes when migrating from VXRail to different
target platforms (VMware, AWS, GCP, Azure). It accounts for vSAN overhead
present in VXRail deployments.

Usage:
    python vxrail_size_estimator.py --disk-size 500 --target vmware
    python vxrail_size_estimator.py --disk-size 500 --memory 16 --cpu 4 --target all
    python vxrail_size_estimator.py --file vms.csv --target aws
"""

import argparse
import csv
import json
import math
import sys
from dataclasses import dataclass, asdict
from typing import List, Optional


# VXRail vSAN overhead factor (10%)
VXRAIL_VSAN_OVERHEAD = 0.10

# Azure standard managed disk sizes in GB
AZURE_DISK_TIERS = [4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32767]


@dataclass
class SizeEstimation:
    """Represents a size estimation result for a target platform."""
    target_type: str
    source_size_gb: float
    estimated_size_gb: float
    size_difference_gb: float
    notes: str


@dataclass
class VMSpec:
    """Represents a VM specification."""
    name: str
    disk_size_gb: float
    memory_gb: float = 0.0
    cpu_count: int = 0


def estimate_vmware_size(disk_size_gb: float, vxrail_overhead: float) -> SizeEstimation:
    """
    Estimate size for VMware target.
    VMware to VMware migration has minimal transformation, uses thin provisioning.
    """
    estimated_size = disk_size_gb - vxrail_overhead
    notes = "VMware target uses thin provisioning by default."

    if vxrail_overhead > 0:
        notes = f"VXRail source detected - accounting for vSAN overhead ({VXRAIL_VSAN_OVERHEAD*100:.0f}%). " + notes

    return SizeEstimation(
        target_type="vmware",
        source_size_gb=disk_size_gb,
        estimated_size_gb=estimated_size,
        size_difference_gb=estimated_size - disk_size_gb,
        notes=notes
    )


def estimate_aws_size(disk_size_gb: float, vxrail_overhead: float) -> SizeEstimation:
    """
    Estimate size for AWS EBS GP3 volumes.
    AWS EBS volumes: 1 GiB - 16 TiB, rounded up to nearest GiB.
    """
    aws_size = disk_size_gb - vxrail_overhead
    # Round up to nearest GiB
    aws_size = math.floor(aws_size) + 1

    notes = "AWS EBS GP3 volumes. Size rounded up to nearest GiB."
    if vxrail_overhead > 0:
        notes = f"VXRail source detected - accounting for vSAN overhead ({VXRAIL_VSAN_OVERHEAD*100:.0f}%). " + notes

    return SizeEstimation(
        target_type="aws",
        source_size_gb=disk_size_gb,
        estimated_size_gb=aws_size,
        size_difference_gb=aws_size - disk_size_gb,
        notes=notes
    )


def estimate_gcp_size(disk_size_gb: float, vxrail_overhead: float) -> SizeEstimation:
    """
    Estimate size for GCP Persistent Disk.
    GCP has a minimum of 10 GB, rounded up to nearest GB.
    """
    gcp_size = disk_size_gb - vxrail_overhead

    # Minimum 10 GB
    if gcp_size < 10:
        gcp_size = 10

    # Round up to nearest GB
    gcp_size = math.floor(gcp_size) + 1

    notes = "GCP Persistent Disk. Minimum 10 GiB."
    if vxrail_overhead > 0:
        notes = f"VXRail source detected - accounting for vSAN overhead ({VXRAIL_VSAN_OVERHEAD*100:.0f}%). " + notes

    return SizeEstimation(
        target_type="gcp",
        source_size_gb=disk_size_gb,
        estimated_size_gb=gcp_size,
        size_difference_gb=gcp_size - disk_size_gb,
        notes=notes
    )


def estimate_azure_size(disk_size_gb: float, vxrail_overhead: float) -> SizeEstimation:
    """
    Estimate size for Azure Managed Disk.
    Azure has standard disk tiers that sizes must align to.
    """
    azure_size = disk_size_gb - vxrail_overhead

    # Find the smallest standard size >= calculated size
    estimated_size = azure_size
    for tier_size in AZURE_DISK_TIERS:
        if tier_size >= azure_size:
            estimated_size = tier_size
            break
    else:
        # If larger than all tiers, use the largest
        estimated_size = AZURE_DISK_TIERS[-1]

    notes = "Azure Managed Disk. Size aligned to standard disk tiers."
    if vxrail_overhead > 0:
        notes = f"VXRail source detected - accounting for vSAN overhead ({VXRAIL_VSAN_OVERHEAD*100:.0f}%). " + notes

    return SizeEstimation(
        target_type="azure",
        source_size_gb=disk_size_gb,
        estimated_size_gb=estimated_size,
        size_difference_gb=estimated_size - disk_size_gb,
        notes=notes
    )


def estimate_size(disk_size_gb: float, target_type: str, is_vxrail: bool = True) -> SizeEstimation:
    """
    Main estimation function that routes to platform-specific estimators.

    Args:
        disk_size_gb: Source disk size in GB
        target_type: Target platform (vmware, aws, gcp, azure)
        is_vxrail: Whether the source is VXRail (accounts for vSAN overhead)

    Returns:
        SizeEstimation object with results
    """
    # Calculate VXRail vSAN overhead
    vxrail_overhead = disk_size_gb * VXRAIL_VSAN_OVERHEAD if is_vxrail else 0.0

    estimators = {
        "vmware": estimate_vmware_size,
        "aws": estimate_aws_size,
        "gcp": estimate_gcp_size,
        "azure": estimate_azure_size,
    }

    if target_type not in estimators:
        return SizeEstimation(
            target_type=target_type,
            source_size_gb=disk_size_gb,
            estimated_size_gb=disk_size_gb,
            size_difference_gb=0,
            notes=f"Unknown target type '{target_type}' - using source size."
        )

    return estimators[target_type](disk_size_gb, vxrail_overhead)


def estimate_all_targets(disk_size_gb: float, is_vxrail: bool = True) -> List[SizeEstimation]:
    """Estimate sizes for all supported target platforms."""
    targets = ["vmware", "aws", "gcp", "azure"]
    return [estimate_size(disk_size_gb, target, is_vxrail) for target in targets]


def estimate_cost(cpu_count: int, memory_gb: float, disk_size_gb: float,
                  target_type: str) -> dict:
    """
    Estimate monthly costs for running a VM on a target platform.
    These are rough estimates and actual costs vary by region and instance type.
    """
    costs = {}

    if target_type == "aws":
        # Rough AWS pricing (m5.xlarge baseline ~$0.192/hour in us-east-1)
        hourly_rate = 0.048 * cpu_count
        costs["compute_monthly"] = hourly_rate * 24 * 30
        costs["storage_monthly"] = disk_size_gb * 0.10  # GP3 pricing
        costs["total_monthly"] = costs["compute_monthly"] + costs["storage_monthly"]

    elif target_type == "gcp":
        # Rough GCP pricing (n2-standard)
        hourly_rate = 0.0475 * cpu_count
        costs["compute_monthly"] = hourly_rate * 24 * 30
        costs["storage_monthly"] = disk_size_gb * 0.17  # PD-SSD pricing
        costs["total_monthly"] = costs["compute_monthly"] + costs["storage_monthly"]

    elif target_type == "azure":
        # Rough Azure pricing (D-series)
        hourly_rate = 0.05 * cpu_count
        costs["compute_monthly"] = hourly_rate * 24 * 30
        costs["storage_monthly"] = disk_size_gb * 0.15  # Premium SSD pricing
        costs["total_monthly"] = costs["compute_monthly"] + costs["storage_monthly"]

    else:
        costs["total_monthly"] = 0

    return costs


def load_vms_from_csv(filepath: str) -> List[VMSpec]:
    """
    Load VM specifications from a CSV file.
    Expected columns: name, disk_size_gb, memory_gb (optional), cpu_count (optional)
    """
    vms = []
    with open(filepath, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            vms.append(VMSpec(
                name=row.get('name', 'unnamed'),
                disk_size_gb=float(row['disk_size_gb']),
                memory_gb=float(row.get('memory_gb', 0)),
                cpu_count=int(row.get('cpu_count', 0))
            ))
    return vms


def format_table(estimations: List[SizeEstimation], vm_name: str = None) -> str:
    """Format estimations as an ASCII table."""
    lines = []

    header = "=" * 80
    lines.append(header)
    if vm_name:
        lines.append(f"VM: {vm_name}")
    lines.append(f"{'Target':<10} {'Source (GB)':<15} {'Estimated (GB)':<18} {'Difference (GB)':<18}")
    lines.append("-" * 80)

    for est in estimations:
        diff_str = f"{est.size_difference_gb:+.2f}"
        lines.append(f"{est.target_type:<10} {est.source_size_gb:<15.2f} {est.estimated_size_gb:<18.2f} {diff_str:<18}")

    lines.append(header)

    # Add notes
    lines.append("\nNotes:")
    for est in estimations:
        lines.append(f"  [{est.target_type}] {est.notes}")

    return "\n".join(lines)


def format_json(estimations: List[SizeEstimation], vm_name: str = None) -> str:
    """Format estimations as JSON."""
    data = {
        "vm_name": vm_name,
        "estimations": [asdict(e) for e in estimations]
    }
    return json.dumps(data, indent=2)


def main():
    parser = argparse.ArgumentParser(
        description="VXRail to VMware/Cloud Size Estimation Tool",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Estimate for a single VM to VMware
  %(prog)s --disk-size 500 --target vmware

  # Estimate for all targets with cost estimation
  %(prog)s --disk-size 500 --memory 16 --cpu 4 --target all --show-costs

  # Estimate from a CSV file
  %(prog)s --file vms.csv --target aws --output json

  # Estimate without VXRail overhead (plain ESXi source)
  %(prog)s --disk-size 500 --target vmware --no-vxrail
        """
    )

    # Input options
    input_group = parser.add_mutually_exclusive_group(required=True)
    input_group.add_argument("--disk-size", "-d", type=float,
                            help="Source disk size in GB")
    input_group.add_argument("--file", "-f", type=str,
                            help="CSV file with VM specifications")

    # VM specifications
    parser.add_argument("--memory", "-m", type=float, default=0,
                       help="Memory in GB (for cost estimation)")
    parser.add_argument("--cpu", "-c", type=int, default=0,
                       help="CPU count (for cost estimation)")
    parser.add_argument("--vm-name", "-n", type=str, default="VM",
                       help="VM name for display")

    # Target options
    parser.add_argument("--target", "-t", type=str, default="all",
                       choices=["vmware", "aws", "gcp", "azure", "all"],
                       help="Target platform (default: all)")

    # Source options
    parser.add_argument("--no-vxrail", action="store_true",
                       help="Source is not VXRail (skip vSAN overhead)")

    # Output options
    parser.add_argument("--output", "-o", type=str, default="table",
                       choices=["table", "json"],
                       help="Output format (default: table)")
    parser.add_argument("--show-costs", action="store_true",
                       help="Show cost estimates (requires --cpu and --memory)")

    args = parser.parse_args()

    is_vxrail = not args.no_vxrail

    # Process input
    if args.file:
        # Process CSV file
        try:
            vms = load_vms_from_csv(args.file)
        except FileNotFoundError:
            print(f"Error: File '{args.file}' not found.", file=sys.stderr)
            sys.exit(1)
        except Exception as e:
            print(f"Error reading CSV file: {e}", file=sys.stderr)
            sys.exit(1)

        all_results = []
        for vm in vms:
            if args.target == "all":
                estimations = estimate_all_targets(vm.disk_size_gb, is_vxrail)
            else:
                estimations = [estimate_size(vm.disk_size_gb, args.target, is_vxrail)]

            if args.output == "table":
                print(format_table(estimations, vm.name))
                print()
            else:
                all_results.append({
                    "vm_name": vm.name,
                    "estimations": [asdict(e) for e in estimations]
                })

        if args.output == "json":
            print(json.dumps(all_results, indent=2))

    else:
        # Process single VM
        disk_size = args.disk_size

        if args.target == "all":
            estimations = estimate_all_targets(disk_size, is_vxrail)
        else:
            estimations = [estimate_size(disk_size, args.target, is_vxrail)]

        if args.output == "table":
            print(format_table(estimations, args.vm_name))

            # Show cost estimates if requested
            if args.show_costs and args.cpu > 0:
                print("\n" + "=" * 80)
                print("COST ESTIMATES (Monthly)")
                print("-" * 80)

                for est in estimations:
                    if est.target_type in ["aws", "gcp", "azure"]:
                        costs = estimate_cost(args.cpu, args.memory, est.estimated_size_gb, est.target_type)
                        print(f"\n{est.target_type.upper()}:")
                        print(f"  Compute: ${costs.get('compute_monthly', 0):.2f}/month")
                        print(f"  Storage: ${costs.get('storage_monthly', 0):.2f}/month")
                        print(f"  Total:   ${costs.get('total_monthly', 0):.2f}/month")

                print("\n" + "=" * 80)
                print("Note: Cost estimates are approximate and vary by region/instance type.")
        else:
            data = {
                "vm_name": args.vm_name,
                "estimations": [asdict(e) for e in estimations]
            }

            if args.show_costs and args.cpu > 0:
                data["cost_estimates"] = {}
                for est in estimations:
                    if est.target_type in ["aws", "gcp", "azure"]:
                        data["cost_estimates"][est.target_type] = estimate_cost(
                            args.cpu, args.memory, est.estimated_size_gb, est.target_type
                        )

            print(json.dumps(data, indent=2))


if __name__ == "__main__":
    main()
