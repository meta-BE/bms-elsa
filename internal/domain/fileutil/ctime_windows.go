//go:build windows

package fileutil

import (
	"os"
	"syscall"
	"time"
)

// fileCreationTime はWindowsのCreationTimeを返す
func fileCreationTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	sys := info.Sys().(*syscall.Win32FileAttributeData)
	return time.Unix(0, sys.CreationTime.Nanoseconds()), nil
}
