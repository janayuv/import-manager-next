//go:build !windows

package windows

// OSMemoryStats captures operating-system level physical and commit memory metrics.
type OSMemoryStats struct {
	WorkingSetBytes     uint64 `json:"workingSetBytes"`
	PeakWorkingSetBytes uint64 `json:"peakWorkingSetBytes"`
	PagefileUsageBytes  uint64 `json:"pagefileUsageBytes"`
	PrivateUsageBytes   uint64 `json:"privateUsageBytes"`
	PageFaultCount      uint32 `json:"pageFaultCount"`
}

// GetOSMemoryStats retrieves stubbed OS memory stats on non-Windows platforms.
func GetOSMemoryStats() (OSMemoryStats, error) {
	return OSMemoryStats{}, nil
}

// TrimProcessWorkingSet is a no-op on non-Windows platforms.
func TrimProcessWorkingSet() error {
	return nil
}
