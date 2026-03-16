//go:build !windows

package fileutil

import (
	"os"
	"time"
)

// fileCreationTime は非WindowsではModTimeにフォールバックする
func fileCreationTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
