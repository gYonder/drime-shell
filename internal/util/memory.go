// Package util provides general utility functions.
package util

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/mem"
)

const (
	// WarnThresholdPercent is the percentage of available RAM above which we warn.
	WarnThresholdPercent = 25
	// AbortThresholdPercent is the percentage of available RAM above which we abort.
	AbortThresholdPercent = 80
)

// MemoryInfo contains information about system memory.
type MemoryInfo struct {
	TotalBytes     uint64
	AvailableBytes uint64
	UsedPercent    float64
}

// GetMemoryInfo returns information about system memory.
func GetMemoryInfo() (*MemoryInfo, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("get memory info: %w", err)
	}

	return &MemoryInfo{
		TotalBytes:     v.Total,
		AvailableBytes: v.Available,
		UsedPercent:    v.UsedPercent,
	}, nil
}

// GetAvailableMemory returns the available system memory in bytes.
func GetAvailableMemory() (uint64, error) {
	info, err := GetMemoryInfo()
	if err != nil {
		return 0, err
	}
	return info.AvailableBytes, nil
}

// CheckResult contains the result of a memory check for a file operation.
type CheckResult struct {
	// OK is true if the operation can proceed.
	OK bool
	// Warning message if the operation should proceed with caution.
	Warning string
	// AbortReason is set if the operation should not proceed.
	AbortReason string
	// AvailableBytes is the amount of available memory.
	AvailableBytes uint64
	// RequiredBytes is the amount of memory required for the operation.
	RequiredBytes uint64
	// RequiredPercent is the percentage of available memory required.
	RequiredPercent float64
}

// CheckMemoryForFile checks if there's enough memory for a file operation.
// fileSize is the size of the file in bytes.
// Returns a CheckResult indicating whether to proceed, warn, or abort.
func CheckMemoryForFile(fileSize int64) *CheckResult {
	result := &CheckResult{
		OK:            true,
		RequiredBytes: uint64(fileSize),
	}

	available, err := GetAvailableMemory()
	if err != nil {
		// If we can't get memory info, proceed with a warning
		result.Warning = "Could not determine available memory; proceeding anyway"
		return result
	}

	result.AvailableBytes = available

	if available == 0 {
		result.Warning = "Could not determine available memory; proceeding anyway"
		return result
	}

	// Calculate percentage of available memory required
	result.RequiredPercent = (float64(fileSize) / float64(available)) * 100

	if result.RequiredPercent >= AbortThresholdPercent {
		result.OK = false
		result.AbortReason = fmt.Sprintf(
			"File size (%s) requires %.0f%% of available memory (%s). "+
				"This operation would likely cause system instability. "+
				"Consider processing the file in smaller chunks or freeing memory.",
			FormatBytes(fileSize),
			result.RequiredPercent,
			FormatBytes(int64(available)),
		)
		return result
	}

	if result.RequiredPercent >= WarnThresholdPercent {
		result.Warning = fmt.Sprintf(
			"Large file: %s requires %.0f%% of available memory (%s). "+
				"Encryption will use significant memory.",
			FormatBytes(fileSize),
			result.RequiredPercent,
			FormatBytes(int64(available)),
		)
	}

	return result
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
