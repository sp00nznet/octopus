#!/usr/bin/env python3
"""
VXRail to ESXi Size Estimator

Calculates the estimated storage size when migrating from VXRail to plain ESXi.
VXRail uses vSAN which has ~10% overhead for metadata. When migrating to ESXi,
this overhead is removed, resulting in smaller storage requirements.

Usage:
    python vxrail_size_estimator.py input.csv
    python vxrail_size_estimator.py input.csv -o output.csv
    python vxrail_size_estimator.py input.csv --overhead 0.15

Input CSV format:
    host,size
    server1,500
    server2,1000

Output CSV format:
    host,size,estsize
    server1,500,450.0
    server2,1000,900.0
"""

import argparse
import csv
import sys

# Default vSAN overhead (10%)
DEFAULT_VSAN_OVERHEAD = 0.10


def estimate_size(size: float, overhead: float) -> float:
    """
    Calculate estimated size after removing vSAN overhead.

    Args:
        size: Current size on VXRail (GB)
        overhead: vSAN overhead factor (default 0.10 = 10%)

    Returns:
        Estimated size on ESXi (GB)
    """
    return size * (1 - overhead)


def process_csv(input_file: str, output_file: str, overhead: float):
    """Process input CSV and write results."""

    results = []
    total_current = 0
    total_estimated = 0

    # Read input
    with open(input_file, 'r') as f:
        reader = csv.DictReader(f)

        # Check for required columns
        if 'host' not in reader.fieldnames or 'size' not in reader.fieldnames:
            print("Error: CSV must have 'host' and 'size' columns", file=sys.stderr)
            sys.exit(1)

        for row in reader:
            host = row['host']
            size = float(row['size'])
            estsize = estimate_size(size, overhead)

            results.append({
                'host': host,
                'size': size,
                'estsize': round(estsize, 2)
            })

            total_current += size
            total_estimated += estsize

    # Write output
    if output_file:
        with open(output_file, 'w', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=['host', 'size', 'estsize'])
            writer.writeheader()
            writer.writerows(results)
        print(f"Output written to {output_file}")
    else:
        # Print to stdout
        print("host,size,estsize")
        for r in results:
            print(f"{r['host']},{r['size']},{r['estsize']}")

    # Print summary
    print(f"\n--- Summary ---", file=sys.stderr)
    print(f"Total hosts: {len(results)}", file=sys.stderr)
    print(f"Total current size: {total_current:.2f} GB", file=sys.stderr)
    print(f"Total estimated size: {total_estimated:.2f} GB", file=sys.stderr)
    print(f"Total savings: {total_current - total_estimated:.2f} GB ({overhead*100:.0f}%)", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(
        description="Estimate storage size when migrating from VXRail to ESXi",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s servers.csv                    # Output to stdout
  %(prog)s servers.csv -o results.csv     # Output to file
  %(prog)s servers.csv --overhead 0.15    # Use 15%% overhead instead of 10%%
        """
    )

    parser.add_argument("input", help="Input CSV file with host,size columns")
    parser.add_argument("-o", "--output", help="Output CSV file (default: stdout)")
    parser.add_argument("--overhead", type=float, default=DEFAULT_VSAN_OVERHEAD,
                       help=f"vSAN overhead factor (default: {DEFAULT_VSAN_OVERHEAD})")

    args = parser.parse_args()

    try:
        process_csv(args.input, args.output, args.overhead)
    except FileNotFoundError:
        print(f"Error: File '{args.input}' not found", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
