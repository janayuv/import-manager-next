//go:build !windows

package windows

// DiskSpaceInfo captures disk capacity and available storage in bytes.
type DiskSpaceInfo struct {
	FreeBytesAvailable     uint64 `json:"freeBytesAvailable"`
	TotalNumberOfBytes     uint64 `json:"totalNumberOfBytes"`
	TotalNumberOfFreeBytes uint64 `json:"totalNumberOfFreeBytes"`
}

// GetDiskSpace provides a portable stub for non-Windows test environments.
func GetDiskSpace(path string) (DiskSpaceInfo, error) {
	return DiskSpaceInfo{
		FreeBytesAvailable:     50 * 1024 * 1024 * 1024,
		TotalNumberOfBytes:     250 * 1024 * 1024 * 1024,
		TotalNumberOfFreeBytes: 50 * 1024 * 1024 * 1024,
	}, nil
}
