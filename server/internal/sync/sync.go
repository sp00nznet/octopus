package sync

import (
	"fmt"
	"time"
)

// SyncManager handles VM synchronization between source and target
type SyncManager struct {
	jobID       int64
	sourceType  string
	targetType  string
	sourceConfig map[string]interface{}
	targetConfig map[string]interface{}
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success          bool      `json:"success"`
	BytesTransferred int64     `json:"bytes_transferred"`
	Duration         int64     `json:"duration_seconds"`
	Error            string    `json:"error,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

// NewSyncManager creates a new sync manager for a migration job
func NewSyncManager(jobID int64, sourceType, targetType string, sourceConfig, targetConfig map[string]interface{}) *SyncManager {
	return &SyncManager{
		jobID:        jobID,
		sourceType:   sourceType,
		targetType:   targetType,
		sourceConfig: sourceConfig,
		targetConfig: targetConfig,
	}
}

// PerformSync executes a sync operation using CBT (Changed Block Tracking)
func (s *SyncManager) PerformSync(vmName string, preserveMAC, preservePortGroups bool) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		Timestamp: startTime,
	}

	// Step 1: Create a snapshot on the source VM
	err := s.createSourceSnapshot(vmName)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create snapshot: %v", err)
		return result, err
	}

	// Step 2: Get changed blocks since last sync (using CBT)
	changedBlocks, err := s.getChangedBlocks(vmName)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get changed blocks: %v", err)
		return result, err
	}

	// Step 3: Transfer changed blocks to target
	bytesTransferred, err := s.transferBlocks(vmName, changedBlocks)
	if err != nil {
		result.Error = fmt.Sprintf("failed to transfer blocks: %v", err)
		return result, err
	}

	// Step 4: Update target VM configuration if needed
	if preserveMAC || preservePortGroups {
		err = s.updateTargetConfig(vmName, preserveMAC, preservePortGroups)
		if err != nil {
			result.Error = fmt.Sprintf("failed to update target config: %v", err)
			return result, err
		}
	}

	result.Success = true
	result.BytesTransferred = bytesTransferred
	result.Duration = int64(time.Since(startTime).Seconds())

	return result, nil
}

// createSourceSnapshot creates a quiesced snapshot for CBT
func (s *SyncManager) createSourceSnapshot(vmName string) error {
	// Implementation depends on source type
	// For VMware, use the VMware client to create a snapshot
	switch s.sourceType {
	case "vmware":
		// Would use vmware.Client to create snapshot
		// client.CreateSnapshot(vmName, "octopus-sync-"+time.Now().Format("20060102150405"), "", true, true)
		return nil
	default:
		return fmt.Errorf("unsupported source type: %s", s.sourceType)
	}
}

// getChangedBlocks retrieves changed blocks using CBT
func (s *SyncManager) getChangedBlocks(vmName string) ([]BlockChange, error) {
	// Implementation uses VMware CBT API
	// Returns list of changed disk extents
	return []BlockChange{}, nil
}

// BlockChange represents a changed disk block
type BlockChange struct {
	DiskKey     int32
	StartOffset int64
	Length      int64
}

// transferBlocks transfers changed blocks to the target
func (s *SyncManager) transferBlocks(vmName string, blocks []BlockChange) (int64, error) {
	var totalBytes int64

	for _, block := range blocks {
		// Read block from source
		// Write block to target
		totalBytes += block.Length
	}

	return totalBytes, nil
}

// updateTargetConfig updates the target VM configuration
func (s *SyncManager) updateTargetConfig(vmName string, preserveMAC, preservePortGroups bool) error {
	// Update MAC addresses and port groups on target VM
	return nil
}

// PerformCutover executes the final cutover
func (s *SyncManager) PerformCutover(vmName string) error {
	// Step 1: Perform final sync
	_, err := s.PerformSync(vmName, true, true)
	if err != nil {
		return fmt.Errorf("final sync failed: %w", err)
	}

	// Step 2: Power off source VM
	err = s.powerOffSource(vmName)
	if err != nil {
		return fmt.Errorf("failed to power off source: %w", err)
	}

	// Step 3: Do one more sync to capture any final changes
	_, err = s.PerformSync(vmName, true, true)
	if err != nil {
		return fmt.Errorf("post-poweroff sync failed: %w", err)
	}

	// Step 4: Power on target VM
	err = s.powerOnTarget(vmName)
	if err != nil {
		return fmt.Errorf("failed to power on target: %w", err)
	}

	return nil
}

// powerOffSource powers off the source VM
func (s *SyncManager) powerOffSource(vmName string) error {
	switch s.sourceType {
	case "vmware":
		// Would use vmware.Client to power off
		return nil
	default:
		return fmt.Errorf("unsupported source type: %s", s.sourceType)
	}
}

// powerOnTarget powers on the target VM
func (s *SyncManager) powerOnTarget(vmName string) error {
	switch s.targetType {
	case "vmware":
		// Would use vmware.Client to power on
		return nil
	case "aws":
		// Would use aws.Client to start instance
		return nil
	case "gcp":
		// Would use gcp.Client to start instance
		return nil
	case "azure":
		// Would use azure.Client to start VM
		return nil
	default:
		return fmt.Errorf("unsupported target type: %s", s.targetType)
	}
}

// RAID overhead multipliers (physical space per unit of logical data)
// VXRail/vSAN uses these policies to determine how much raw space is consumed
var RAIDOverhead = map[string]float64{
	"raid1_ftt1": 2.0,  // RAID-1, FTT=1: Full mirror (2 copies)
	"raid1_ftt2": 3.0,  // RAID-1, FTT=2: Triple mirror (3 copies)
	"raid5_ftt1": 1.33, // RAID-5, FTT=1: 3+1 erasure coding
	"raid6_ftt2": 1.5,  // RAID-6, FTT=2: 4+2 erasure coding
	"none":       1.0,  // No RAID/FTT=0
}

// Organic adjustment factors based on VMware documentation
// These account for factors that cause reported size to differ from actual data
var OrganicFactors = struct {
	DedupExpansionLow    float64 // Low dedup ratio environments
	DedupExpansionMedium float64 // Medium dedup ratio
	DedupExpansionHigh   float64 // High dedup ratio
	CompressionExpansion float64 // Typical compression ratio
	VMSwapOverhead       float64 // Swap files don't migrate
	SnapshotOverhead     float64 // Snapshots consolidate on migration
}{
	DedupExpansionLow:    1.2,
	DedupExpansionMedium: 1.4,
	DedupExpansionHigh:   1.6,
	CompressionExpansion: 1.25,
	VMSwapOverhead:       0.95,
	SnapshotOverhead:     0.90,
}

// VXRailConfig holds VXRail-specific estimation parameters
type VXRailConfig struct {
	RAIDPolicy       string  `json:"raid_policy"`        // raid1_ftt1, raid5_ftt1, etc.
	DedupEnabled     bool    `json:"dedup_enabled"`      // Is deduplication enabled
	CompressionEnabled bool  `json:"compression_enabled"` // Is compression enabled
	DedupRatio       float64 `json:"dedup_ratio"`        // Actual dedup ratio (e.g., 1.5 for 1.5:1)
	CompressionRatio float64 `json:"compression_ratio"`  // Actual compression ratio
	HasSnapshots     bool    `json:"has_snapshots"`      // Does VM have snapshots
}

// SizeEstimation represents a size estimate for a target
type SizeEstimation struct {
	SourceSizeGB     float64 `json:"source_size_gb"`      // What vCenter reports (includes RAID overhead)
	LogicalSizeGB    float64 `json:"logical_size_gb"`     // Primary data only (RAID overhead removed)
	EstimatedSizeGB  float64 `json:"estimated_size_gb"`   // Final migration estimate
	SizeDifferenceGB float64 `json:"size_difference_gb"`  // Difference from source
	ChangePercent    float64 `json:"change_percent"`      // Percentage change
	Notes            string  `json:"notes"`               // Explanation of factors applied
}

// EstimateSize estimates the size of a VM on a target platform
// This is the simple version for backward compatibility
func EstimateSize(diskSizeGB, memoryGB float64, cpuCount int, targetType string, isVXRail bool) *SizeEstimation {
	// Use default VXRail config (RAID-1 FTT=1, no dedup/compression)
	config := VXRailConfig{
		RAIDPolicy: "raid1_ftt1",
	}
	return EstimateSizeWithConfig(diskSizeGB, memoryGB, cpuCount, targetType, isVXRail, config)
}

// EstimateSizeWithConfig estimates size with detailed VXRail configuration
func EstimateSizeWithConfig(diskSizeGB, memoryGB float64, cpuCount int, targetType string, isVXRail bool, config VXRailConfig) *SizeEstimation {
	estimation := &SizeEstimation{
		SourceSizeGB: diskSizeGB,
	}

	var notes []string
	logicalSize := diskSizeGB

	// Step 1: Remove RAID overhead to get logical/primary data
	if isVXRail {
		raidOverhead := RAIDOverhead[config.RAIDPolicy]
		if raidOverhead == 0 {
			raidOverhead = RAIDOverhead["raid1_ftt1"] // Default to RAID-1 FTT=1
		}
		logicalSize = diskSizeGB / raidOverhead
		notes = append(notes, fmt.Sprintf("Primary data (÷%.2f RAID)", raidOverhead))
	}

	estimation.LogicalSizeGB = logicalSize

	// Step 2: Calculate migration size with expansion factors
	estimatedSize := logicalSize

	// Dedup expansion: deduplicated data will expand on target
	if isVXRail && config.DedupEnabled {
		expansion := config.DedupRatio
		if expansion <= 1.0 {
			expansion = OrganicFactors.DedupExpansionMedium
		}
		estimatedSize *= expansion
		notes = append(notes, fmt.Sprintf("Dedup expansion ×%.2f", expansion))
	}

	// Compression expansion
	if isVXRail && config.CompressionEnabled {
		expansion := config.CompressionRatio
		if expansion <= 1.0 {
			expansion = OrganicFactors.CompressionExpansion
		}
		estimatedSize *= expansion
		notes = append(notes, fmt.Sprintf("Compression expansion ×%.2f", expansion))
	}

	// VM overhead: swap files don't need to migrate
	estimatedSize *= OrganicFactors.VMSwapOverhead

	// Snapshot consolidation
	if config.HasSnapshots {
		estimatedSize *= OrganicFactors.SnapshotOverhead
		notes = append(notes, "Snapshot consolidation")
	}

	// Step 3: Apply target-specific adjustments
	switch targetType {
	case "vmware":
		notes = append(notes, "VMware thin provisioning")

	case "aws":
		// AWS EBS rounds up to nearest GiB
		estimatedSize = float64(int(estimatedSize) + 1)
		notes = append(notes, "AWS EBS GP3 (rounded up)")

	case "gcp":
		// GCP minimum 10 GB, rounded up
		if estimatedSize < 10 {
			estimatedSize = 10
		}
		estimatedSize = float64(int(estimatedSize) + 1)
		notes = append(notes, "GCP Persistent Disk (min 10 GiB)")

	case "azure":
		// Azure aligns to standard disk tiers
		standardSizes := []float64{4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32767}
		for _, size := range standardSizes {
			if size >= estimatedSize {
				estimatedSize = size
				break
			}
		}
		notes = append(notes, "Azure Managed Disk tier")
	}

	estimation.EstimatedSizeGB = estimatedSize
	estimation.SizeDifferenceGB = estimatedSize - diskSizeGB

	if diskSizeGB > 0 {
		estimation.ChangePercent = ((estimatedSize - diskSizeGB) / diskSizeGB) * 100
	}

	// Build notes string
	notesStr := ""
	for i, note := range notes {
		if i > 0 {
			notesStr += "; "
		}
		notesStr += note
	}
	estimation.Notes = notesStr

	return estimation
}

// EstimateCost estimates the monthly cost for running a VM on a target platform
func EstimateCost(cpuCount int, memoryGB, diskSizeGB float64, targetType, region string) map[string]float64 {
	costs := make(map[string]float64)

	switch targetType {
	case "aws":
		// Rough AWS pricing (varies by region and instance type)
		// Using m5.xlarge as baseline ($0.192/hour in us-east-1)
		hourlyRate := 0.048 * float64(cpuCount) // Approximate
		costs["compute_monthly"] = hourlyRate * 24 * 30
		costs["storage_monthly"] = diskSizeGB * 0.10 // GP3 pricing
		costs["total_monthly"] = costs["compute_monthly"] + costs["storage_monthly"]

	case "gcp":
		// Rough GCP pricing
		hourlyRate := 0.0475 * float64(cpuCount) // n2-standard pricing
		costs["compute_monthly"] = hourlyRate * 24 * 30
		costs["storage_monthly"] = diskSizeGB * 0.17 // PD-SSD pricing
		costs["total_monthly"] = costs["compute_monthly"] + costs["storage_monthly"]

	case "azure":
		// Rough Azure pricing
		hourlyRate := 0.05 * float64(cpuCount) // D-series pricing
		costs["compute_monthly"] = hourlyRate * 24 * 30
		costs["storage_monthly"] = diskSizeGB * 0.15 // Premium SSD pricing
		costs["total_monthly"] = costs["compute_monthly"] + costs["storage_monthly"]

	default:
		costs["total_monthly"] = 0
	}

	return costs
}
