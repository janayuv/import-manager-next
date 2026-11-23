//go:build windows

package windows

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	procGetDiskFreeSpaceExW = modKernel32.NewProc("GetDiskFreeSpaceExW")
)

// DiskSpaceInfo captures disk capacity and available storage in bytes.
type DiskSpaceInfo struct {
	FreeBytesAvailable     uint64 `json:"freeBytesAvailable"`
	TotalNumberOfBytes     uint64 `json:"totalNumberOfBytes"`
	TotalNumberOfFreeBytes uint64 `json:"totalNumberOfFreeBytes"`
}

// GetDiskSpace retrieves storage capacity for the drive containing path.
func GetDiskSpace(path string) (DiskSpaceInfo, error) {
	vol := filepath.VolumeName(path)
	if vol == "" {
		vol = path
	}
	if len(vol) > 0 && vol[len(vol)-1] != '\\' {
		vol += "\\"
	}
	volPtr, err := syscall.UTF16PtrFromString(vol)
	if err != nil {
		return DiskSpaceInfo{}, err
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	r1, _, callErr := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(volPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return DiskSpaceInfo{}, fmt.Errorf("GetDiskFreeSpaceExW failed for %q: %w", vol, callErr)
	}

	return DiskSpaceInfo{
		FreeBytesAvailable:     freeBytesAvailable,
		TotalNumberOfBytes:     totalNumberOfBytes,
		TotalNumberOfFreeBytes: totalNumberOfFreeBytes,
	}, nil
}
