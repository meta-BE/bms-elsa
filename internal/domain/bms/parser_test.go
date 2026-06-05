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

	egoSig := bms.ComputeMinHash(egoParsed.WAVRefCounts)
	fixSig := bms.ComputeMinHash(fixParsed.WAVRefCounts)
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

	dstorvSig := bms.ComputeMinHash(dstorvParsed.WAVRefCounts)
	randomSig := bms.ComputeMinHash(randomParsed.WAVRefCounts)
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
	sig := bms.ComputeMinHash(parsed.WAVRefCounts)

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

func TestParseBMSFile_WAVRefCounts_BasicChannels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ref_counts.bms")
	content := strings.Join([]string{
		"#TITLE refcounts test",
		"#WAV01 alpha.wav",
		"#WAV02 beta.wav",
		"#WAV03 gamma.wav",
		"#00101:01010101",       // BGM × 4
		"#00111:0201",           // 1P 可視 (alpha=01 × 1, beta=02 × 1)
		"#00121:030003",         // 2P 可視 (gamma × 2)
		"#00131:0100",           // 1P 不可視 (alpha × 1)
		"#00151:01",             // 1P ロング (alpha × 1, 1 ペアのみ)
		"#00161:02",             // 2P ロング (beta × 1)
		"#001D1:0202",           // 1P 地雷 (除外)
		"#001E1:03",             // 2P 地雷 (除外)
		"#00102:01",             // 小節長 (除外)
		"#00103:F0F0",           // BPM 変更 (除外)
		"#00104:0101",           // BGA base (除外)
		"#00106:0101",           // BGA poor (除外)
		"#00109:0101",           // STOP (除外)
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}

	want := map[string]int{
		"alpha": 4 + 1 + 1 + 1, // BGM(4) + 11(1) + 31(1) + 51(1)
		"beta":  1 + 1,         // 11(1) + 61(1)
		"gamma": 2,             // 21(2)
	}
	for k, v := range want {
		if got := parsed.WAVRefCounts[k]; got != v {
			t.Errorf("WAVRefCounts[%q] = %d, want %d", k, got, v)
		}
	}
	if _, hasMine := parsed.WAVRefCounts["__slot:D1"]; hasMine {
		t.Error("mine slot should not appear in ref counts")
	}
}

func TestParseBMSFile_WAVRefCounts_BackfillForwardReference(t *testing.T) {
	// データ行が #WAVxx 定義より先に出現するケース。__slot:XX として保留され、
	// 走査終了後に basename に振り替えられることを確認する。
	dir := t.TempDir()
	path := filepath.Join(dir, "backfill.bms")
	content := strings.Join([]string{
		"#TITLE backfill test",
		// データ行が先に出現
		"#00101:0101",       // BGM × 2 (slot 01 未定義の時点)
		"#00111:020002",     // 1P 可視 (slot 02 × 2)
		"#00121:99",         // slot 99 (未定義のまま終わる)
		// 定義は後から
		"#WAV01 forward.wav",
		"#WAV02 later.wav",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}

	if got := parsed.WAVRefCounts["forward"]; got != 2 {
		t.Errorf("WAVRefCounts[\"forward\"] = %d, want 2 (backfilled from __slot:01)", got)
	}
	if got := parsed.WAVRefCounts["later"]; got != 2 {
		t.Errorf("WAVRefCounts[\"later\"] = %d, want 2 (backfilled from __slot:02)", got)
	}
	// 未解決の __slot:99 はマップに残ってはいけない（定義なしのため破棄）
	for k := range parsed.WAVRefCounts {
		if strings.HasPrefix(k, "__slot:") {
			t.Errorf("unresolved placeholder remained: %q", k)
		}
	}
}

func TestParseBMSFile_WAVRefCounts_RandomBlockSelectsIfOne(t *testing.T) {
	// #RANDOM 内では #IF 1 のデータ行のみカウントされ、#IF 2 のデータ行は除外される。
	dir := t.TempDir()
	path := filepath.Join(dir, "random.bms")
	content := strings.Join([]string{
		"#TITLE random test",
		"#WAV01 picked.wav",
		"#WAV02 skipped.wav",
		"#RANDOM 2",
		"#IF 1",
		"#00111:01010101", // picked × 4 (取り込む)
		"#ENDIF",
		"#IF 2",
		"#00111:02020202", // skipped × 4 (取り込まない)
		"#ENDIF",
		"#ENDRANDOM",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}

	if got := parsed.WAVRefCounts["picked"]; got != 4 {
		t.Errorf("WAVRefCounts[\"picked\"] = %d, want 4 (#IF 1 block)", got)
	}
	if got := parsed.WAVRefCounts["skipped"]; got != 0 {
		t.Errorf("WAVRefCounts[\"skipped\"] = %d, want 0 (#IF 2 must be skipped)", got)
	}
}

func TestComputeMinHash_SameBasenamesDifferentCountsLowerSimilarity(t *testing.T) {
	// 同じ basename 集合でも、参照回数階層 (バケット) が大きく違えば類似度は下がる。
	// バケット閾値: 1, 2, 4, 8, 16, 32, 64
	light := map[string]int{
		"00": 1, "01": 1, "02": 1, "03": 1, "04": 1,
	}
	heavy := map[string]int{
		"00": 64, "01": 64, "02": 64, "03": 64, "04": 64,
	}
	lightSig := bms.ComputeMinHash(light)
	heavySig := bms.ComputeMinHash(heavy)
	sim := lightSig.Similarity(heavySig)
	t.Logf("same basenames, count=1 vs count=64 similarity: %.4f", sim)
	if sim >= 0.5 {
		t.Errorf("different count buckets should produce sim < 0.5, got %.4f", sim)
	}
}

func TestComputeMinHash_SameBasenamesSimilarCountsHighSimilarity(t *testing.T) {
	// 同じ basename 集合 + 同じバケット帯なら類似度は 1.0。
	a := map[string]int{"kick": 5, "snare": 5, "bgm": 5}
	b := map[string]int{"kick": 6, "snare": 6, "bgm": 6}
	sim := bms.ComputeMinHash(a).Similarity(bms.ComputeMinHash(b))
	if sim != 1.0 {
		t.Errorf("same buckets should produce sim == 1.0, got %.4f", sim)
	}
}

func TestComputeMinHash_BasenameOnlyContribution(t *testing.T) {
	// 参照回数が 0 (未参照だが定義はある) でも basename 単独要素は集合に入る。
	a := map[string]int{"kick": 0, "snare": 0}
	b := map[string]int{"kick": 0, "snare": 0}
	sim := bms.ComputeMinHash(a).Similarity(bms.ComputeMinHash(b))
	if sim != 1.0 {
		t.Errorf("identical zero-count maps should produce sim == 1.0, got %.4f", sim)
	}
}
