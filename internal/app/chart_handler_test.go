package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteChartFile_RemovesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bms")
	if err := os.WriteFile(path, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	h := &ChartHandler{}
	if err := h.DeleteChartFile(path); err != nil {
		t.Fatalf("DeleteChartFile returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists after delete: stat err=%v", err)
	}
}

func TestDeleteChartFile_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.bms")

	h := &ChartHandler{}
	if err := h.DeleteChartFile(path); err != nil {
		t.Fatalf("DeleteChartFile on missing file returned error: %v", err)
	}
}
