package util

import (
	"testing"
)

func TestGetMemoryInfo(t *testing.T) {
	info, err := GetMemoryInfo()
	if err != nil {
		t.Fatalf("GetMemoryInfo failed: %v", err)
	}

	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0")
	}

	if info.AvailableBytes == 0 {
		t.Error("AvailableBytes should not be 0")
	}

	if info.AvailableBytes > info.TotalBytes {
		t.Error("AvailableBytes should not exceed TotalBytes")
	}
}

func TestGetAvailableMemory(t *testing.T) {
	available, err := GetAvailableMemory()
	if err != nil {
		t.Fatalf("GetAvailableMemory failed: %v", err)
	}

	if available == 0 {
		t.Error("Available memory should not be 0")
	}
}

func TestCheckMemoryForFile(t *testing.T) {
	// Small file should always be OK
	result := CheckMemoryForFile(1024) // 1KB
	if !result.OK {
		t.Error("Small file should be OK")
	}
	if result.Warning != "" {
		t.Errorf("Small file should not have warning: %s", result.Warning)
	}
	if result.AbortReason != "" {
		t.Errorf("Small file should not have abort reason: %s", result.AbortReason)
	}
}

func TestCheckMemoryForFileLarge(t *testing.T) {
	// Get available memory to calculate a large file size
	available, err := GetAvailableMemory()
	if err != nil {
		t.Skip("Could not get available memory")
	}

	// 30% of available memory should trigger warning
	largeSize := int64(float64(available) * 0.30)
	result := CheckMemoryForFile(largeSize)
	if !result.OK {
		t.Error("30% of available memory should still be OK")
	}
	if result.Warning == "" {
		t.Error("30% of available memory should have warning")
	}
}

func TestCheckMemoryForFileHuge(t *testing.T) {
	// Get available memory
	available, err := GetAvailableMemory()
	if err != nil {
		t.Skip("Could not get available memory")
	}

	// 85% of available memory should abort
	hugeSize := int64(float64(available) * 0.85)
	result := CheckMemoryForFile(hugeSize)
	if result.OK {
		t.Error("85% of available memory should abort")
	}
	if result.AbortReason == "" {
		t.Error("85% of available memory should have abort reason")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}
