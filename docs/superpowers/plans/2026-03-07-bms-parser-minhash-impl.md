# BMSパーサー・MinHash・ノート数表示 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** BMSファイルからWAV定義を抽出しMinHashで類似度比較できるライブラリを作成し、songdata.dbのノート数をUIに表示する。

**Architecture:** BMSパーサーとMinHash計算は`internal/domain/bms/`パッケージに新設。ノート数表示はsongdata.dbの`notes`カラムを既存のデータパイプライン（モデル→DTO→ハンドラー→フロントエンド）に追加する形で実現。DBスキーマは`chart_meta`に`wav_minhash`カラムを追加。

**Tech Stack:** Go, SQLite, Svelte 4, TypeScript, Wails v2

---

### Task 1: BMSパーサー — WAV定義抽出

**Files:**
- Create: `internal/domain/bms/parser.go`
- Create: `internal/domain/bms/parser_test.go`

**Step 1: テストファイルを作成**

`internal/domain/bms/parser_test.go`:
```go
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
	// RANDOM内の#IF 1のみを処理した場合のWAV定義数を記録
	t.Logf("Random [SP ANOTHER] WAV files (IF 1 only): %d", len(result))
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
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/bms/...`
Expected: コンパイルエラー（パッケージ未存在）

**Step 3: パーサーを実装**

`internal/domain/bms/parser.go`:
```go
package bms

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ParseWAVFiles はBMSファイルからWAV定義のファイル名集合を抽出する。
// RANDOM内は#IF 1のブロックのみ処理する。
// ファイル名は拡張子を除去したベース名で返す（大文字小文字を保持）。
func ParseWAVFiles(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	// RANDOM処理用のスタック: 現在のRANDOMネストの深さと処理可否
	type randomState struct {
		active bool // このRANDOMブロック内で現在の#IFが処理対象（=1）か
	}
	var stack []randomState
	skipDepth := 0 // スキップ中のネスト深さ（0=スキップしていない）

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] != '#' {
			continue
		}

		upper := strings.ToUpper(line)

		// RANDOM制御
		if strings.HasPrefix(upper, "#RANDOM ") {
			if skipDepth > 0 {
				skipDepth++
			} else {
				// 新しいRANDOMブロック開始、まだどの#IFにも入っていない
				stack = append(stack, randomState{active: false})
			}
			continue
		}
		if strings.HasPrefix(upper, "#ENDRANDOM") {
			if skipDepth > 0 {
				skipDepth--
			} else if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if strings.HasPrefix(upper, "#IF ") {
			if skipDepth > 0 {
				skipDepth++
				continue
			}
			if len(stack) > 0 {
				val := strings.TrimSpace(line[4:])
				if val == "1" {
					stack[len(stack)-1].active = true
				} else {
					skipDepth = 1
				}
			}
			continue
		}
		if strings.HasPrefix(upper, "#ENDIF") {
			if skipDepth > 0 {
				skipDepth--
				if skipDepth == 0 && len(stack) > 0 {
					stack[len(stack)-1].active = false
				}
			} else if len(stack) > 0 {
				stack[len(stack)-1].active = false
			}
			continue
		}

		// スキップ中なら無視
		if skipDepth > 0 {
			continue
		}

		// #WAVxx の処理
		if len(upper) >= 6 && upper[:4] == "#WAV" && upper[4] != ' ' {
			// #WAVxx filename
			rest := line[4:]
			spaceIdx := strings.IndexByte(rest, ' ')
			if spaceIdx < 0 {
				continue
			}
			filename := strings.TrimSpace(rest[spaceIdx+1:])
			if filename == "" {
				continue
			}
			// 拡張子を除去してベース名にする
			ext := filepath.Ext(filename)
			if ext != "" {
				filename = filename[:len(filename)-len(ext)]
			}
			// 大文字小文字を区別せずユニーク化（同じファイル名で大文字小文字違いの重複を防ぐ）
			key := strings.ToLower(filename)
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
```

**Step 4: テストを実行して通ることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/bms/... -v`
Expected: DstorvEgo=631件、DstorvFalseFix=630件のテストがPASS。Randomは件数をログ出力（初回実行で正確な値を確認後、テストの期待値を調整）。

**Step 5: Randomテストの期待値を調整**

初回実行のログ出力から正確なWAV定義数を確認し、`TestParseWAVFiles_RandomSPAnother`の期待値を設定する。

**Step 6: コミット**

```bash
git add internal/domain/bms/parser.go internal/domain/bms/parser_test.go
git commit -m "feat: BMSパーサー（WAV定義抽出、RANDOM対応）"
```

---

### Task 2: MinHash計算

**Files:**
- Create: `internal/domain/bms/minhash.go`
- Modify: `internal/domain/bms/parser_test.go`（MinHashテストを追加）

**Step 1: テストを追加**

`parser_test.go`に以下を追加:
```go
func TestMinHash_SameSongHighSimilarity(t *testing.T) {
	egoPath := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	fixPath := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_single4_fix.bme")

	egoWAVs, err := bms.ParseWAVFiles(egoPath)
	if err != nil {
		t.Fatal(err)
	}
	fixWAVs, err := bms.ParseWAVFiles(fixPath)
	if err != nil {
		t.Fatal(err)
	}

	egoSig := bms.ComputeMinHash(egoWAVs)
	fixSig := bms.ComputeMinHash(fixWAVs)
	sim := egoSig.Similarity(fixSig)

	t.Logf("Dstorv [Ego] vs [false_fix] similarity: %.4f", sim)
	if sim < 0.9 {
		t.Errorf("same song similarity should be >= 0.9, got %.4f", sim)
	}
}

func TestMinHash_DifferentSongLowSimilarity(t *testing.T) {
	dstorvPath := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	randomPath := filepath.Join(testdataDir(t), "[Clue]Random", "_random_s4.bms")

	dstorvWAVs, err := bms.ParseWAVFiles(dstorvPath)
	if err != nil {
		t.Fatal(err)
	}
	randomWAVs, err := bms.ParseWAVFiles(randomPath)
	if err != nil {
		t.Fatal(err)
	}

	dstorvSig := bms.ComputeMinHash(dstorvWAVs)
	randomSig := bms.ComputeMinHash(randomWAVs)
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
	wavs, err := bms.ParseWAVFiles(path)
	if err != nil {
		t.Fatal(err)
	}
	sig := bms.ComputeMinHash(wavs)

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
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/bms/... -run TestMinHash`
Expected: コンパイルエラー

**Step 3: MinHash実装**

`internal/domain/bms/minhash.go`:
```go
package bms

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
)

const MinHashSize = 64

// MinHashSignature はK=64のMinHash署名（256バイト）
type MinHashSignature [MinHashSize]uint32

// ComputeMinHash はファイル名集合からMinHash署名を計算する。
func ComputeMinHash(files []string) MinHashSignature {
	var sig MinHashSignature
	for i := range sig {
		sig[i] = math.MaxUint32
	}
	if len(files) == 0 {
		return sig
	}
	for _, f := range files {
		for i := 0; i < MinHashSize; i++ {
			h := fnv.New32a()
			// シードとしてインデックスを書き込み
			_ = binary.Write(h, binary.LittleEndian, uint32(i))
			h.Write([]byte(f))
			v := h.Sum32()
			if v < sig[i] {
				sig[i] = v
			}
		}
	}
	return sig
}

// Similarity は2つのMinHash署名のJaccard類似度の近似値を返す（0.0〜1.0）。
func (s MinHashSignature) Similarity(other MinHashSignature) float64 {
	// 両方が空集合（全てMaxUint32）の場合は1.0
	allMax := true
	for i := 0; i < MinHashSize; i++ {
		if s[i] != math.MaxUint32 || other[i] != math.MaxUint32 {
			allMax = false
			break
		}
	}
	if allMax {
		return 1.0
	}

	match := 0
	for i := 0; i < MinHashSize; i++ {
		if s[i] == other[i] {
			match++
		}
	}
	return float64(match) / float64(MinHashSize)
}

// Bytes はMinHash署名を256バイトのバイト列にシリアライズする。
func (s MinHashSignature) Bytes() []byte {
	buf := make([]byte, MinHashSize*4)
	for i, v := range s {
		binary.LittleEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}

// MinHashFromBytes は256バイトのバイト列からMinHash署名を復元する。
func MinHashFromBytes(data []byte) (MinHashSignature, error) {
	if len(data) != MinHashSize*4 {
		return MinHashSignature{}, fmt.Errorf("invalid minhash data length: %d", len(data))
	}
	var sig MinHashSignature
	for i := range sig {
		sig[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return sig, nil
}
```

**Step 4: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/bms/... -v -run TestMinHash`
Expected: 全テストPASS

**Step 5: コミット**

```bash
git add internal/domain/bms/minhash.go internal/domain/bms/parser_test.go
git commit -m "feat: MinHash計算（WAV集合類似度比較）"
```

---

### Task 3: chart_metaにwav_minhashカラムを追加

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`

**Step 1: migrations.goのchart_metaテーブル定義にwav_minhashを追加**

`migrations.go`の`CREATE TABLE IF NOT EXISTS chart_meta`文に追加（既存テーブルへの追加はALTER TABLE）:

statements配列の末尾（`CREATE INDEX IF NOT EXISTS idx_dte_md5`の後あたり）に以下を追加:
```go
`ALTER TABLE chart_meta ADD COLUMN wav_minhash BLOB`,
```

ただし`ALTER TABLE ADD COLUMN`は既にカラムが存在するとエラーになるため、冪等にするため以下のように実装:

```go
// wav_minhashカラムの追加（冪等）
var hasWavMinhash int
_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chart_meta') WHERE name='wav_minhash'`).Scan(&hasWavMinhash)
if hasWavMinhash == 0 {
    if _, err := db.Exec(`ALTER TABLE chart_meta ADD COLUMN wav_minhash BLOB`); err != nil {
        return fmt.Errorf("add wav_minhash column: %w", err)
    }
}
```

これをRunMigrationsの`return nil`の直前に追加する。

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 3: 既存テストが壊れていないことを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/... -v -run TestMigrations`
Expected: PASS

**Step 4: コミット**

```bash
git add internal/adapter/persistence/migrations.go
git commit -m "feat: chart_metaにwav_minhashカラムを追加"
```

---

### Task 4: Chartモデル・DTOにNotesフィールドを追加

**Files:**
- Modify: `internal/domain/model/song.go`
- Modify: `internal/app/dto/dto.go`

**Step 1: Chartモデルにフィールド追加**

`song.go`のChart構造体に追加（`Path`の後）:
```go
Notes      int
```

**Step 2: ChartDTOにフィールド追加**

`dto.go`のChartDTO構造体に追加（`Path`の後）:
```go
Notes          int      `json:"notes"`
```

**Step 3: ChartToDTO関数にマッピング追加**

`dto.go`のChartToDTO関数内で`Path: c.Path,`の後に追加:
```go
Notes:      c.Notes,
```

**Step 4: ChartListItem・ChartListItemDTOにフィールド追加**

`songdata_reader.go`のChartListItem構造体（383行付近）に追加（`Difficulty`の後）:
```go
Notes       int
```

`dto.go`のChartListItemDTO構造体に追加（`Difficulty`の後）:
```go
Notes       int     `json:"notes"`
```

**Step 5: chart_handler.goのListChartsマッピングに追加**

`chart_handler.go`の35-45行のDTO変換に追加（`Difficulty: c.Difficulty,`の後）:
```go
Notes:      c.Notes,
```

**Step 6: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 7: コミット**

```bash
git add internal/domain/model/song.go internal/app/dto/dto.go internal/adapter/persistence/songdata_reader.go internal/app/chart_handler.go
git commit -m "feat: Chart/DTOにNotesフィールドを追加"
```

---

### Task 5: songdata_reader.goのSQLクエリにnotesカラムを追加

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`

**Step 1: GetSongByFolderのSQLにnotesを追加**

行257の`s.minbpm, s.maxbpm, s.path`を`s.minbpm, s.maxbpm, s.path, s.notes`に変更。

行272-275のScan呼び出しに`&c.Notes`を追加:
```go
if err := rows.Scan(
    &c.MD5, &c.SHA256, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist,
    &c.Genre, &c.Mode, &c.Difficulty, &c.Level,
    &c.MinBPM, &c.MaxBPM, &c.Path, &c.Notes,
); err != nil {
```

**Step 2: ListAllChartsのSQLにnotesを追加**

行411の`s.difficulty,`の後に`s.notes,`を追加。

行434-437のScan呼び出しに`&c.Notes`を追加:
```go
if err := rows.Scan(
    &c.MD5, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist, &c.Genre,
    &c.MinBPM, &c.MaxBPM, &c.Difficulty, &c.Notes,
    &eventName, &releaseYear, &c.HasIRMeta,
); err != nil {
```

**Step 3: GetChartByMD5のSQLにnotesを追加**

行503-504の`genre, mode, difficulty, level, minbpm, maxbpm, path`を`genre, mode, difficulty, level, minbpm, maxbpm, path, notes`に変更。

行508-512のScan呼び出しに`&c.Notes`を追加:
```go
`, md5).Scan(
    &c.MD5, &c.SHA256, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist,
    &c.Genre, &c.Mode, &c.Difficulty, &c.Level,
    &c.MinBPM, &c.MaxBPM, &c.Path, &c.Notes,
)
```

**Step 4: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/... -v`
Expected: 既存テスト全PASS

**Step 5: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go
git commit -m "feat: songdata_reader.goのSQLクエリにnotesカラムを追加"
```

---

### Task 6: フロントエンド — 譜面一覧テーブルにNOTES列追加

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte`

**Step 1: カラム定義にNOTES列を追加**

`ChartListView.svelte`の96行目（`genre`列定義）の後に追加:
```typescript
    {
      id: 'notes',
      header: 'Notes',
      size: 80,
      accessorFn: (row) => row.notes || 0,
    },
```

**Step 2: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功（Wailsバインディング再生成が必要な場合は先に`wails generate module`を実行）

**Step 3: コミット**

```bash
git add frontend/src/views/ChartListView.svelte
git commit -m "feat: 譜面一覧テーブルにNOTES列を追加"
```

---

### Task 7: フロントエンド — ChartInfoCardにノート数表示

**Files:**
- Modify: `frontend/src/components/ChartInfoCard.svelte`

**Step 1: Level表示の後にNotes表示を追加**

`ChartInfoCard.svelte`の14行目（`Level`表示）の後に追加:
```svelte
      <span><span class="font-semibold">Notes:</span> {chart.notes?.toLocaleString() ?? '-'}</span>
```

**Step 2: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

**Step 3: コミット**

```bash
git add frontend/src/components/ChartInfoCard.svelte
git commit -m "feat: ChartInfoCardにノート数を表示"
```

---

### Task 8: Wailsバインディング再生成 + 統合ビルド

**Step 1: Wailsバインディング再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: 成功

**Step 2: Wailsフルビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: 成功
