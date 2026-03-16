package fileutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileCreationTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	before := time.Now().Add(-time.Second)
	os.WriteFile(path, []byte("test"), 0644)

	ct, err := fileCreationTime(path)
	if err != nil {
		t.Fatalf("fileCreationTime failed: %v", err)
	}
	if ct.Before(before) {
		t.Errorf("creation time %v is before file creation %v", ct, before)
	}
}

func TestFileCreationTime_NotExist(t *testing.T) {
	_, err := fileCreationTime("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("should return error for nonexistent file")
	}
}
