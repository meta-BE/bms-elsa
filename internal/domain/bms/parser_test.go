package bms_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
}

func TestParseBMSFile_DstorvEgo(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	if len(parsed.WAVFiles) == 0 {
		t.Fatal("WAV files should not be empty")
	}
	// Dstorv [Ego] は631件のWAV定義を持つ
	if len(parsed.WAVFiles) != 631 {
		t.Errorf("expected 631 WAV files, got %d", len(parsed.WAVFiles))
	}
}

func TestParseBMSFile_DstorvFalseFix(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_single4_fix.bme")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	if len(parsed.WAVFiles) != 630 {
		t.Errorf("expected 630 WAV files, got %d", len(parsed.WAVFiles))
	}
}

func TestParseBMSFile_RandomSPAnother(t *testing.T) {
	// RANDOMブロック内は#IF 1のみ処理。#IF 1ルートで定義されるWAV数を検証。
	path := filepath.Join(testdataDir(t), "[Clue]Random", "_random_s4.bms")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	if len(parsed.WAVFiles) == 0 {
		t.Fatal("WAV files should not be empty")
	}
	// RANDOM内の#IF 1のみを処理した場合のWAV定義数: 1063件
	if len(parsed.WAVFiles) != 1063 {
		t.Errorf("expected 1063 WAV files, got %d", len(parsed.WAVFiles))
	}
}

func TestMinHash_SameSongHighSimilarity(t *testing.T) {
	egoPath := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	fixPath := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_single4_fix.bme")

	egoParsed, err := bms.ParseBMSFile(egoPath)
	if err != nil {
		t.Fatal(err)
	}
	fixParsed, err := bms.ParseBMSFile(fixPath)
	if err != nil {
		t.Fatal(err)
	}

	egoSig := bms.ComputeMinHash(egoParsed.WAVFiles)
	fixSig := bms.ComputeMinHash(fixParsed.WAVFiles)
	sim := egoSig.Similarity(fixSig)

	t.Logf("Dstorv [Ego] vs [false_fix] similarity: %.4f", sim)
	if sim < 0.9 {
		t.Errorf("same song similarity should be >= 0.9, got %.4f", sim)
	}
}

func TestMinHash_DifferentSongLowSimilarity(t *testing.T) {
	dstorvPath := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	randomPath := filepath.Join(testdataDir(t), "[Clue]Random", "_random_s4.bms")

	dstorvParsed, err := bms.ParseBMSFile(dstorvPath)
	if err != nil {
		t.Fatal(err)
	}
	randomParsed, err := bms.ParseBMSFile(randomPath)
	if err != nil {
		t.Fatal(err)
	}

	dstorvSig := bms.ComputeMinHash(dstorvParsed.WAVFiles)
	randomSig := bms.ComputeMinHash(randomParsed.WAVFiles)
	sim := dstorvSig.Similarity(randomSig)

	t.Logf("Dstorv vs Random similarity: %.4f", sim)
	if sim > 0.1 {
		t.Errorf("different song similarity should be <= 0.1, got %.4f", sim)
	}
}

func TestMinHash_EmptySet(t *testing.T) {
	sig := bms.ComputeMinHash(nil)
	sim := sig.Similarity(sig)
	// 空集合同士の類似度は1.0とする
	if sim != 1.0 {
		t.Errorf("empty vs empty should be 1.0, got %.4f", sim)
	}
}

func TestMinHash_SerializeRoundtrip(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sig := bms.ComputeMinHash(parsed.WAVFiles)

	// シリアライズ→デシリアライズ
	blob := sig.Bytes()
	if len(blob) != 256 {
		t.Fatalf("expected 256 bytes, got %d", len(blob))
	}
	restored, err := bms.MinHashFromBytes(blob)
	if err != nil {
		t.Fatal(err)
	}
	if sig.Similarity(restored) != 1.0 {
		t.Error("roundtrip should produce identical signature")
	}
}

func TestParseBMSFile_ExtensionNormalization(t *testing.T) {
	// WAV定義のファイル名は拡張子除去されたベース名であること
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	for _, f := range parsed.WAVFiles {
		if filepath.Ext(f) != "" {
			t.Errorf("expected no extension, got %q", f)
			break
		}
	}
}

func TestParseBMSFile_HeaderFields(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Clue]Random", "_random_s4.bms")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	// #TITLE はRANDOM外で定義
	if parsed.Title != "Random [SP ANOTHER]" {
		t.Errorf("expected title 'Random [SP ANOTHER]', got %q", parsed.Title)
	}
	// #ARTIST はRANDOM内の#IF 1で定義
	if parsed.Artist == "" {
		t.Error("artist should not be empty")
	}
	// #SUBARTIST はRANDOM外で定義
	if parsed.Subartist == "" {
		t.Error("subartist should not be empty")
	}
	// #GENRE はRANDOM内の#IF 1で定義（文字化けした値）
	if parsed.Genre == "" {
		t.Error("genre should not be empty")
	}
	// WAVFiles は既存テストと同じ件数
	if len(parsed.WAVFiles) != 1063 {
		t.Errorf("expected 1063 WAV files, got %d", len(parsed.WAVFiles))
	}
	// MD5は空でないことを確認
	if parsed.MD5 == "" {
		t.Error("MD5 should not be empty")
	}
	if len(parsed.MD5) != 32 {
		t.Errorf("MD5 should be 32 hex chars, got %d", len(parsed.MD5))
	}
}

func TestParseBMSFile_NonRandomHeaders(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	if parsed.Title == "" {
		t.Error("title should not be empty")
	}
	if parsed.Artist == "" {
		t.Error("artist should not be empty")
	}
	if len(parsed.WAVFiles) != 631 {
		t.Errorf("expected 631 WAV files, got %d", len(parsed.WAVFiles))
	}
}

func TestParseBMSFile_ShiftJIS(t *testing.T) {
	// Shift-JISエンコードのテストファイルを動的に作成
	dir := t.TempDir()
	path := filepath.Join(dir, "sjis_test.bms")

	encoder := japanese.ShiftJIS.NewEncoder()
	content := "#TITLE テスト楽曲\n#ARTIST テストアーティスト\n#WAV01 test.wav\n"
	sjisBytes, err := io.ReadAll(transform.NewReader(strings.NewReader(content), encoder))
	if err != nil {
		t.Fatalf("Shift-JIS encode failed: %v", err)
	}
	os.WriteFile(path, sjisBytes, 0644)

	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}

	if parsed.Title != "テスト楽曲" {
		t.Errorf("Title = %q, want %q", parsed.Title, "テスト楽曲")
	}
	if parsed.Artist != "テストアーティスト" {
		t.Errorf("Artist = %q, want %q", parsed.Artist, "テストアーティスト")
	}
}
