//go:build windows

package windows

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentProcess        = modKernel32.NewProc("GetCurrentProcess")
	procSetProcessWorkingSetSize = modKernel32.NewProc("SetProcessWorkingSetSize")

	modPsapi                 = windows.NewLazySystemDLL("psapi.dll")
	procEmptyWorkingSet      = modPsapi.NewProc("EmptyWorkingSet")
	procGetProcessMemoryInfo = modPsapi.NewProc("GetProcessMemoryInfo")
)

// ProcessMemoryCountersEx mirrors PROCESS_MEMORY_COUNTERS_EX in winapi.
type ProcessMemoryCountersEx struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

// OSMemoryStats captures operating-system level physical and commit memory metrics.
type OSMemoryStats struct {
	WorkingSetBytes     uint64 `json:"workingSetBytes"`
	PeakWorkingSetBytes uint64 `json:"peakWorkingSetBytes"`
	PagefileUsageBytes  uint64 `json:"pagefileUsageBytes"`
	PrivateUsageBytes   uint64 `json:"privateUsageBytes"`
	PageFaultCount      uint32 `json:"pageFaultCount"`
}

// GetOSMemoryStats retrieves live physical and commit memory consumption via psapi.dll.
func GetOSMemoryStats() (OSMemoryStats, error) {
	hProcess, _, _ := procGetCurrentProcess.Call()
	if hProcess == 0 {
		return OSMemoryStats{}, fmt.Errorf("GetCurrentProcess returned invalid handle")
	}

	var pmc ProcessMemoryCountersEx
	pmc.Cb = uint32(unsafe.Sizeof(pmc))

	ret, _, err := procGetProcessMemoryInfo.Call(
		hProcess,
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(pmc.Cb),
	)
	if ret == 0 {
		return OSMemoryStats{}, fmt.Errorf("GetProcessMemoryInfo failed: %w", err)
	}

	return OSMemoryStats{
		WorkingSetBytes:     uint64(pmc.WorkingSetSize),
		PeakWorkingSetBytes: uint64(pmc.PeakWorkingSetSize),
		PagefileUsageBytes:  uint64(pmc.PagefileUsage),
		PrivateUsageBytes:   uint64(pmc.PrivateUsage),
		PageFaultCount:      pmc.PageFaultCount,
	}, nil
}

// TrimProcessWorkingSet requests the Windows Virtual Memory Manager to trim unreferenced
// physical pages from the process working set back to the OS pool.
func TrimProcessWorkingSet() error {
	hProcess, _, _ := procGetCurrentProcess.Call()
	if hProcess == 0 {
		return fmt.Errorf("GetCurrentProcess returned invalid handle")
	}

	// 1. Try EmptyWorkingSet from psapi.dll
	if procEmptyWorkingSet.Find() == nil {
		ret, _, _ := procEmptyWorkingSet.Call(hProcess)
		if ret != 0 {
			return nil
		}
	}

	// 2. Fall back to SetProcessWorkingSetSize with (-1, -1) which orders VMM to trim working set
	minusOne := ^uintptr(0)
	ret, _, err := procSetProcessWorkingSetSize.Call(hProcess, minusOne, minusOne)
	if ret == 0 {
		return fmt.Errorf("SetProcessWorkingSetSize trim failed: %w", err)
	}
	return nil
}
