package bms_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
}

func TestParseWAVFiles_DstorvEgo(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	result, err := bms.ParseWAVFiles(path)
	if err != nil {
		t.Fatalf("ParseWAVFiles failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("WAV files should not be empty")
	}
	// Dstorv [Ego] は631件のWAV定義を持つ
	if len(result) != 631 {
		t.Errorf("expected 631 WAV files, got %d", len(result))
	}
}

func TestParseWAVFiles_DstorvFalseFix(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_single4_fix.bme")
	result, err := bms.ParseWAVFiles(path)
	if err != nil {
		t.Fatalf("ParseWAVFiles failed: %v", err)
	}
	if len(result) != 630 {
		t.Errorf("expected 630 WAV files, got %d", len(result))
	}
}

func TestParseWAVFiles_RandomSPAnother(t *testing.T) {
	// RANDOMブロック内は#IF 1のみ処理。#IF 1ルートで定義されるWAV数を検証。
	path := filepath.Join(testdataDir(t), "[Clue]Random", "_random_s4.bms")
	result, err := bms.ParseWAVFiles(path)
	if err != nil {
		t.Fatalf("ParseWAVFiles failed: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("WAV files should not be empty")
	}
	// RANDOM内の#IF 1のみを処理した場合のWAV定義数: 1063件
	if len(result) != 1063 {
		t.Errorf("expected 1063 WAV files, got %d", len(result))
	}
}

func TestParseWAVFiles_ExtensionNormalization(t *testing.T) {
	// WAV定義のファイル名は拡張子除去されたベース名であること
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	result, err := bms.ParseWAVFiles(path)
	if err != nil {
		t.Fatalf("ParseWAVFiles failed: %v", err)
	}
	for _, f := range result {
		if filepath.Ext(f) != "" {
			t.Errorf("expected no extension, got %q", f)
			break
		}
	}
}
