#!/usr/bin/env python3
"""
VXRail to ESXi Size Estimator

Calculates estimated storage size when migrating from VXRail/vSAN to plain ESXi.

The size change depends on multiple factors:
1. RAID/FTT Policy - RAID-1 doubles data, RAID-5/6 adds parity overhead
2. Thin vs Thick - Provisioned size vs actual used space
3. Deduplication & Compression - Data may expand when migrated
4. Object Space Reservation - Whether space was reserved

When migrating to ESXi, RAID overhead is removed (single copy), but deduped
data may expand.

References:
- https://blogs.vmware.com/cloud-foundation/2022/01/14/demystifying-capacity-reporting-in-vsan/
- https://digitalthoughtdisruption.com/2019/03/26/vsan-capacity-math-made-easy/
- https://techdocs.broadcom.com/us/en/vmware-cis/vsan/vsan/8-0/vsan-administration/

Usage:
    python vxrail_size_estimator.py input.csv
    python vxrail_size_estimator.py input.csv --raid raid1 --dedup-ratio 1.5
    python vxrail_size_estimator.py input.csv --size-type used

Input CSV: host,size
Output CSV: host,size,estsize
"""

import argparse
import csv
import sys
from dataclasses import dataclass
from typing import Optional


# RAID overhead multipliers (how much raw space is consumed per unit of data)
# These represent how much vSAN consumes for each unit of actual VM data
RAID_OVERHEAD = {
    'raid1_ftt1': 2.0,      # RAID-1, FTT=1: Full mirror (2 copies)
    'raid1_ftt2': 3.0,      # RAID-1, FTT=2: Triple mirror (3 copies)
    'raid1_ftt3': 4.0,      # RAID-1, FTT=3: Quad mirror (4 copies)
    'raid5_ftt1': 1.33,     # RAID-5, FTT=1: 3+1 parity
    'raid6_ftt2': 1.5,      # RAID-6, FTT=2: 4+2 parity
    'none': 1.0,            # No RAID (FTT=0)
}

# Friendly aliases
RAID_ALIASES = {
    'raid1': 'raid1_ftt1',
    'raid5': 'raid5_ftt1',
    'raid6': 'raid6_ftt2',
    'mirror': 'raid1_ftt1',
    'ftt1': 'raid1_ftt1',
    'ftt2': 'raid1_ftt2',
}


@dataclass
class EstimationParams:
    """Parameters that affect size estimation."""
    raid_policy: str = 'raid1_ftt1'     # vSAN RAID policy
    size_type: str = 'provisioned'       # 'provisioned' or 'used'
    dedup_ratio: float = 1.0             # Dedup ratio (1.0 = none, 1.5 = 1.5:1)
    compression_ratio: float = 1.0       # Compression ratio (1.0 = none)
    thin_ratio: Optional[float] = None   # If size_type=provisioned, what % is used


def get_raid_overhead(policy: str) -> float:
    """Get the RAID overhead multiplier for a policy."""
    policy = policy.lower().replace('-', '_').replace(' ', '_')
    if policy in RAID_ALIASES:
        policy = RAID_ALIASES[policy]
    return RAID_OVERHEAD.get(policy, 2.0)  # Default to RAID-1


def estimate_esxi_size(vxrail_size: float, params: EstimationParams) -> dict:
    """
    Estimate the size a VM will be after migrating from VXRail to ESXi.

    The calculation logic:
    1. If size_type is 'provisioned' and thin_ratio given, calculate actual used
    2. The reported vSAN size includes RAID overhead - divide by raid multiplier
       to get actual unique data
    3. If dedup/compression was enabled, data will expand - multiply by ratios

    Args:
        vxrail_size: Size reported on VXRail (GB)
        params: Estimation parameters

    Returns:
        dict with estimated size and calculation details
    """
    details = {'input_size': vxrail_size}

    # Step 1: Determine actual used space if size is provisioned
    if params.size_type == 'provisioned' and params.thin_ratio:
        working_size = vxrail_size * params.thin_ratio
        details['after_thin'] = working_size
        details['thin_applied'] = True
    else:
        working_size = vxrail_size
        details['thin_applied'] = False

    # Step 2: Remove RAID overhead
    # vSAN size includes replicas/parity, divide by overhead to get unique data
    raid_overhead = get_raid_overhead(params.raid_policy)
    unique_data = working_size / raid_overhead
    details['raid_policy'] = params.raid_policy
    details['raid_overhead'] = raid_overhead
    details['after_raid_removal'] = unique_data

    # Step 3: Account for dedup/compression expansion
    # If data was deduped, it will expand when migrated
    if params.dedup_ratio > 1.0 or params.compression_ratio > 1.0:
        # Data expands because dedup/compression no longer applies
        expansion = params.dedup_ratio * params.compression_ratio
        estimated_size = unique_data * expansion
        details['dedup_expansion'] = params.dedup_ratio
        details['compression_expansion'] = params.compression_ratio
    else:
        estimated_size = unique_data

    details['estimated_size'] = round(estimated_size, 2)
    details['size_change_pct'] = round((estimated_size - vxrail_size) / vxrail_size * 100, 1)

    return details


def process_csv(input_file: str, output_file: Optional[str], params: EstimationParams,
                verbose: bool = False):
    """Process input CSV and write results."""

    results = []
    total_current = 0
    total_estimated = 0

    # Read input
    with open(input_file, 'r') as f:
        reader = csv.DictReader(f)

        if 'host' not in reader.fieldnames or 'size' not in reader.fieldnames:
            print("Error: CSV must have 'host' and 'size' columns", file=sys.stderr)
            sys.exit(1)

        for row in reader:
            host = row['host']
            size = float(row['size'])

            estimation = estimate_esxi_size(size, params)
            estsize = estimation['estimated_size']

            results.append({
                'host': host,
                'size': size,
                'estsize': estsize,
                'details': estimation
            })

            total_current += size
            total_estimated += estsize

    # Write output
    if output_file:
        with open(output_file, 'w', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=['host', 'size', 'estsize'])
            writer.writeheader()
            for r in results:
                writer.writerow({'host': r['host'], 'size': r['size'], 'estsize': r['estsize']})
        print(f"Output written to {output_file}")
    else:
        print("host,size,estsize")
        for r in results:
            print(f"{r['host']},{r['size']},{r['estsize']}")

    # Print summary to stderr
    print(f"\n{'='*60}", file=sys.stderr)
    print("ESTIMATION SUMMARY", file=sys.stderr)
    print(f"{'='*60}", file=sys.stderr)
    print(f"Parameters used:", file=sys.stderr)
    print(f"  RAID Policy: {params.raid_policy} ({get_raid_overhead(params.raid_policy)}x overhead)", file=sys.stderr)
    print(f"  Size Type: {params.size_type}", file=sys.stderr)
    if params.thin_ratio:
        print(f"  Thin Provisioning Ratio: {params.thin_ratio*100:.0f}% used", file=sys.stderr)
    if params.dedup_ratio > 1.0:
        print(f"  Dedup Ratio: {params.dedup_ratio}:1 (data will expand)", file=sys.stderr)
    if params.compression_ratio > 1.0:
        print(f"  Compression Ratio: {params.compression_ratio}:1 (data will expand)", file=sys.stderr)
    print(f"\nResults:", file=sys.stderr)
    print(f"  Total hosts: {len(results)}", file=sys.stderr)
    print(f"  Total VXRail size: {total_current:,.2f} GB", file=sys.stderr)
    print(f"  Total ESXi estimate: {total_estimated:,.2f} GB", file=sys.stderr)
    change = total_estimated - total_current
    change_pct = (change / total_current) * 100 if total_current > 0 else 0
    if change < 0:
        print(f"  Size reduction: {abs(change):,.2f} GB ({abs(change_pct):.1f}% smaller)", file=sys.stderr)
    else:
        print(f"  Size increase: {change:,.2f} GB ({change_pct:.1f}% larger)", file=sys.stderr)
    print(f"{'='*60}", file=sys.stderr)

    if verbose:
        print("\nDetailed breakdown:", file=sys.stderr)
        for r in results:
            print(f"\n  {r['host']}:", file=sys.stderr)
            d = r['details']
            print(f"    Input: {d['input_size']} GB", file=sys.stderr)
            print(f"    After RAID removal (/{d['raid_overhead']}): {d['after_raid_removal']:.2f} GB", file=sys.stderr)
            print(f"    Final estimate: {d['estimated_size']} GB ({d['size_change_pct']:+.1f}%)", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(
        description="Estimate storage size when migrating from VXRail to ESXi",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
RAID Policies:
  raid1, raid1_ftt1    RAID-1 with FTT=1 (2x overhead, 50%% reduction)
  raid1_ftt2           RAID-1 with FTT=2 (3x overhead, 67%% reduction)
  raid5, raid5_ftt1    RAID-5 with FTT=1 (1.33x overhead, 25%% reduction)
  raid6, raid6_ftt2    RAID-6 with FTT=2 (1.5x overhead, 33%% reduction)
  none                 No RAID/FTT=0 (no change)

Size Types:
  provisioned    The full VMDK size (what vCenter shows as "Provisioned")
  used           The actual consumed space (what vCenter shows as "Used")

Examples:
  # Default: RAID-1, provisioned size, no dedup
  %(prog)s servers.csv

  # RAID-5 cluster with 1.5:1 dedup ratio
  %(prog)s servers.csv --raid raid5 --dedup-ratio 1.5

  # Using actual "used" size (not provisioned), RAID-1
  %(prog)s servers.csv --size-type used --raid raid1

  # Provisioned size but only 60%% actually used (thin provisioned)
  %(prog)s servers.csv --thin-ratio 0.6

  # Output to file with verbose details
  %(prog)s servers.csv -o results.csv --verbose
        """
    )

    parser.add_argument("input", help="Input CSV file with host,size columns")
    parser.add_argument("-o", "--output", help="Output CSV file (default: stdout)")

    parser.add_argument("--raid", type=str, default="raid1_ftt1",
                       help="vSAN RAID policy (default: raid1_ftt1)")
    parser.add_argument("--size-type", type=str, default="provisioned",
                       choices=["provisioned", "used"],
                       help="Type of size in input (default: provisioned)")
    parser.add_argument("--thin-ratio", type=float,
                       help="If provisioned, what fraction is actually used (0.0-1.0)")
    parser.add_argument("--dedup-ratio", type=float, default=1.0,
                       help="Deduplication ratio, e.g., 1.5 for 1.5:1 (default: 1.0 = none)")
    parser.add_argument("--compression-ratio", type=float, default=1.0,
                       help="Compression ratio, e.g., 1.3 for 1.3:1 (default: 1.0 = none)")
    parser.add_argument("--verbose", "-v", action="store_true",
                       help="Show detailed breakdown for each host")

    args = parser.parse_args()

    params = EstimationParams(
        raid_policy=args.raid,
        size_type=args.size_type,
        dedup_ratio=args.dedup_ratio,
        compression_ratio=args.compression_ratio,
        thin_ratio=args.thin_ratio
    )

    try:
        process_csv(args.input, args.output, params, args.verbose)
    except FileNotFoundError:
        print(f"Error: File '{args.input}' not found", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
