# wav_minhash アルゴリズム v2 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** wav_minhash の計算アルゴリズムを v1（basename 集合のみ）から v2（basename + 参照回数の累積バケットタグ）に置き換え、起動時マイグレーションで旧データを再計算待ちにする。

**Architecture:** `internal/domain/bms/parser.go` でデータセクションを読んで `WAVRefCounts` を集計し、`internal/domain/bms/minhash.go::ComputeMinHash` のシグネチャを `map[string]int` に変更してバケットタグを生成する。`internal/adapter/persistence/migrations.go` で `PRAGMA user_version` を用いて 1 回だけ `chart_meta.wav_minhash` をクリアする。既存のスキャン起動シーケンス（`app.go::startup` → `StartMinHashScan` → `StartScanDuplicates`）には手を加えず、自動的に背景再計算が走る形にする。

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), Wails v2 (UI イベント)

**Spec:** `docs/superpowers/specs/2026-06-05-wav-minhash-v2-design.md`

---

## File Structure

実装で触るファイル:

| パス | 役割 | 変更種別 |
|---|---|---|
| `internal/domain/bms/parser.go` | BMS パース。`WAVRefCounts` フィールド追加とデータ行集計 | 修正 |
| `internal/domain/bms/parser_test.go` | パーサのテスト。`WAVRefCounts` の新規ケース追加、既存 `ComputeMinHash` 呼び出しの引数更新 | 修正 |
| `internal/domain/bms/minhash.go` | MinHash 実装。`ComputeMinHash` のシグネチャを `map[string]int` に変更、バケットタグ生成 | 修正 |
| `internal/usecase/scan_minhash.go` | バックグラウンドスキャン。`ComputeMinHash` 呼び出しを `parsed.WAVRefCounts` に変更 | 修正 |
| `internal/usecase/estimate_diff_install.go` | 差分インストール推定。同上 | 修正 |
| `internal/adapter/persistence/migrations.go` | スキーマ・データマイグレーション。`PRAGMA user_version` ベースの v2 移行追加 | 修正 |
| `internal/adapter/persistence/migrations_test.go` | マイグレーションのテスト。v2 移行のケース追加 | 修正 |

触らないファイル: `cmd/validate-minhash/`（検証用 dev ツール、別実装で残置）, `internal/domain/similarity/`（`Similarity` メソッドのインタフェースは不変）, `app.go`（起動シーケンスは現状維持）。

---

## Task 1: ParsedBMS に WAVRefCounts を追加してデータ行を集計

**Files:**
- Modify: `internal/domain/bms/parser.go`
- Test: `internal/domain/bms/parser_test.go`

このタスクではパース時のデータ行走査を追加し、`WAVRefCounts map[string]int` に各 basename の参照回数を格納する。`#RANDOM` の扱いは既存と同じく `#IF 1` のみ、`WAV` 参照チャンネルは `01, 11-19, 21-29, 31-39, 41-49, 51-59, 61-69`、地雷 `D1-D9 / E1-E9` は集計対象外。

- [ ] **Step 1: 既存テストをまず走らせて基準値を確認**

Run: `go test ./internal/domain/bms/... -run TestParseBMSFile_DstorvEgo -v`
Expected: PASS（既存 `WAVFiles` 件数=631 のテスト）

- [ ] **Step 2: `WAVRefCounts` フィールドを `ParsedBMS` に追加（空のまま）**

`internal/domain/bms/parser.go` の `ParsedBMS` 構造体を以下に置き換える:

```go
// ParsedBMS はBMSファイルのパース結果を保持する。
type ParsedBMS struct {
	MD5          string         // ファイル全体のMD5ハッシュ（16進小文字32文字）
	Title        string         // #TITLE
	Subtitle     string         // #SUBTITLE
	Artist       string         // #ARTIST
	Subartist    string         // #SUBARTIST
	Genre        string         // #GENRE
	WAVFiles     []string       // WAV定義リスト（拡張子除去・小文字正規化済み）
	WAVRefCounts map[string]int // basename -> データセクション参照回数（#RANDOM #IF 1 のみ）
}
```

そして `ParseBMSFile` 関数の冒頭 (`result := &ParsedBMS{...}` 部分) を以下に変更:

```go
	result := &ParsedBMS{
		MD5:          fmt.Sprintf("%x", hash),
		WAVRefCounts: make(map[string]int),
	}
```

- [ ] **Step 3: 既存テストが引き続き通ることを確認**

Run: `go test ./internal/domain/bms/... -v`
Expected: 全テスト PASS（`WAVRefCounts` は未集計のまま空マップだが既存テストは影響を受けない）

- [ ] **Step 4: 失敗するテストを `parser_test.go` の末尾に追加**

`internal/domain/bms/parser_test.go` の末尾に以下を追加:

```go
func TestParseBMSFile_WAVRefCounts_BasicChannels(t *testing.T) {
	// 手書きの最小 BMS で BGM(01) と プレイチャンネル(11, 21, 31, 51, 61) の
	// 参照を集計できるかを確認。地雷(D1, E1)と非対象(02, 03, 04, 06, 09)は無視。
	dir := t.TempDir()
	path := filepath.Join(dir, "ref_counts.bms")
	content := strings.Join([]string{
		"#TITLE refcounts test",
		"#WAV01 alpha.wav",
		"#WAV02 beta.wav",
		"#WAV03 gamma.wav",
		// 集計対象チャンネル
		"#00101:01010101",       // BGM × 4
		"#00111:0201",           // 1P 可視 (alpha=01 × 1, beta=02 × 1, ... ペアで処理)
		"#00121:030003",         // 2P 可視 (gamma × 2)
		"#00131:0100",           // 1P 不可視 (alpha × 1)
		"#00151:01",             // 1P ロング (alpha × 1, 1 ペアのみ)
		"#00161:02",             // 2P ロング (beta × 1)
		// 非対象チャンネル: 集計されてはいけない
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
	// 地雷の対象 WAV (beta, gamma) が地雷チャンネルの分カウントされていないことの確認は
	// 上記 want のとおり (beta=2, gamma=2 で地雷分は加算されていない)。
	// 念のため 未定義 slot が混入していないことも確認:
	if _, hasMine := parsed.WAVRefCounts["__slot:D1"]; hasMine {
		t.Error("mine slot should not appear in ref counts")
	}
}
```

- [ ] **Step 5: テストを実行して失敗することを確認**

Run: `go test ./internal/domain/bms/... -run TestParseBMSFile_WAVRefCounts_BasicChannels -v`
Expected: FAIL（`WAVRefCounts["alpha"] = 0, want 7` 等。データ行集計が未実装のため）

- [ ] **Step 6: パーサの `#WAVxx` 処理で slot 表を保持するように変更**

`internal/domain/bms/parser.go` の `seen := make(map[string]struct{})` を以下に置き換える:

```go
	seen := make(map[string]struct{})
	slotToBasename := make(map[string]string) // 大文字 2 文字 slot -> basename
```

`#WAVxx` の処理ブロック（`if len(upper) >= 6 && upper[:4] == "#WAV" && upper[4] != ' '` から始まる箇所）の中で、basename を `seen` に追加した直後に slot 表を更新する。具体的には:

既存:

```go
		if len(upper) >= 6 && upper[:4] == "#WAV" && upper[4] != ' ' {
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
			key := strings.ToLower(filename)
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
			}
		}
```

を以下に置き換える:

```go
		if len(upper) >= 6 && upper[:4] == "#WAV" && upper[4] != ' ' {
			rest := line[4:]
			spaceIdx := strings.IndexByte(rest, ' ')
			if spaceIdx < 0 {
				continue
			}
			slot := strings.ToUpper(strings.TrimSpace(rest[:spaceIdx]))
			if len(slot) != 2 {
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
			key := strings.ToLower(filename)
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
			}
			if _, exists := slotToBasename[slot]; !exists {
				slotToBasename[slot] = key
				if _, ok := result.WAVRefCounts[key]; !ok {
					result.WAVRefCounts[key] = 0
				}
			}
			continue
		}
```

- [ ] **Step 7: データ行 `#nnnCC:DDDD...` 処理を追加**

`#WAVxx` 処理ブロックの直後（`if err := scanner.Err(); err != nil {` の手前）に以下を追加:

```go
		// データ行 #nnnCC:DDDD... の WAV 参照を集計
		// 対象チャンネル: 01, 11-19, 21-29, 31-39, 41-49, 51-59, 61-69
		if len(line) >= 7 && line[0] == '#' && isASCIIDigit(line[1]) && isASCIIDigit(line[2]) && isASCIIDigit(line[3]) {
			if line[6] != ':' {
				continue
			}
			channel := strings.ToUpper(line[4:6])
			if !isWAVReferenceChannel(channel) {
				continue
			}
			payload := strings.TrimSpace(line[7:])
			if len(payload) < 2 || len(payload)%2 != 0 {
				continue
			}
			for i := 0; i+1 < len(payload); i += 2 {
				slot := strings.ToUpper(payload[i : i+2])
				if slot == "00" {
					continue
				}
				if basename, ok := slotToBasename[slot]; ok {
					result.WAVRefCounts[basename]++
				} else {
					// slot 定義が未到達。後段で振り替える。
					result.WAVRefCounts["__slot:"+slot]++
				}
			}
			continue
		}
```

そしてファイル末尾に補助関数を追加:

```go
// isASCIIDigit は ASCII 数字 0-9 か判定する。
func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

// isWAVReferenceChannel は WAV を鳴らすチャンネルか判定する。
// 対象: 01 (BGM), 11-19, 21-29 (可視ノート), 31-39, 41-49 (不可視 keysound),
//       51-59, 61-69 (ロングノート)。
// 地雷 (D1-D9, E1-E9) は楽曲音源と無関係として除外する。
func isWAVReferenceChannel(ch string) bool {
	if len(ch) != 2 {
		return false
	}
	if ch == "01" {
		return true
	}
	switch ch[0] {
	case '1', '2', '3', '4', '5', '6':
		return ch[1] >= '1' && ch[1] <= '9'
	}
	return false
}
```

- [ ] **Step 8: 走査終了後の slot 解決バックフィルを追加**

`if err := scanner.Err(); err != nil {` の手前、`return nil, err` よりも前に以下を追加:

```go
	// データ行が WAV 定義より前に出現していたケース: "__slot:XX" として保留したカウントを
	// basename に振り替える。
	for key, cnt := range result.WAVRefCounts {
		if !strings.HasPrefix(key, "__slot:") {
			continue
		}
		slot := key[len("__slot:"):]
		delete(result.WAVRefCounts, key)
		if basename, ok := slotToBasename[slot]; ok {
			result.WAVRefCounts[basename] += cnt
		}
		// 解決できない (定義のない slot) は捨てる
	}
```

- [ ] **Step 9: テストを実行して PASS することを確認**

Run: `go test ./internal/domain/bms/... -run TestParseBMSFile_WAVRefCounts_BasicChannels -v`
Expected: PASS

- [ ] **Step 10: 全ての既存テストが引き続き通ることを確認**

Run: `go test ./internal/domain/bms/... -v`
Expected: 全テスト PASS（`WAVFiles` 件数のアサート、`Similarity` のアサート等が全て通る）

- [ ] **Step 11: コミット**

```bash
git add internal/domain/bms/parser.go internal/domain/bms/parser_test.go
git commit -m "$(cat <<'EOF'
feat(bms): ParsedBMS に WAVRefCounts を追加してデータ行参照を集計

#RANDOM #IF 1 のみ、チャンネル 01, 11-19, 21-29, 31-39, 41-49, 51-59, 61-69 の
WAV 参照を basename ごとにカウントする。地雷 D1-D9 / E1-E9 と BGA/BPM/STOP 等は除外。
slot 定義が後方に出るケースは走査終了後に振り替える。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: ComputeMinHash のシグネチャを `map[string]int` に変更してバケットタグを生成

**Files:**
- Modify: `internal/domain/bms/minhash.go`
- Modify: `internal/domain/bms/parser_test.go`
- Modify: `internal/domain/similarity/similarity_test.go`
- Modify: `internal/usecase/scan_minhash.go`
- Modify: `internal/usecase/estimate_diff_install.go`

シグネチャ変更により呼び出し元 (テスト 2 ファイル + 本番コード 2 ファイル) も同じコミットで更新する。ビルドが壊れた状態を最小化するため、まず失敗テストを書いて TDD で進めるが、コンパイル成功は呼び出し元を全部更新した時点で達成する形になる。

- [ ] **Step 1: 失敗するバケットテストを `parser_test.go` の末尾に追加**

`internal/domain/bms/parser_test.go` の末尾に以下を追加:

```go
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
	// 完全に同じ basename 集合で count=0 同士なら類似度は 1.0 になるべき。
	a := map[string]int{"kick": 0, "snare": 0}
	b := map[string]int{"kick": 0, "snare": 0}
	sim := bms.ComputeMinHash(a).Similarity(bms.ComputeMinHash(b))
	if sim != 1.0 {
		t.Errorf("identical zero-count maps should produce sim == 1.0, got %.4f", sim)
	}
}
```

- [ ] **Step 2: ビルドが通らないことを確認 (コンパイルエラー期待)**

Run: `go build ./internal/domain/bms/...`
Expected: FAIL（`cannot use ... (type map[string]int) as type []string in argument to bms.ComputeMinHash` 等）

- [ ] **Step 3: `minhash.go` の `ComputeMinHash` シグネチャをマップ受け取りに置き換える**

`internal/domain/bms/minhash.go` の `ComputeMinHash` 関数全体を以下に置き換える:

```go
// bucketThresholds は累積タグの閾値（2 の冪）。
// count >= threshold[i] の各 i について "n:<basename>#t<threshold>" を集合に追加する。
var bucketThresholds = [...]int{1, 2, 4, 8, 16, 32, 64}

// ComputeMinHash は basename と参照回数から K=64 の MinHash 署名を計算する。
//
// 入力集合の構築:
//   - 各 basename b について常に "n:" + b を追加（参照回数 0 でも残す）
//   - count が bucketThresholds の各値以上のとき、累積タグ "n:" + b + "#t" + 閾値 を追加
//
// この設計により:
//   - 同曲難易度差分間で参照回数が小さく揺れても、跨ぐバケット数が少なければ類似度は維持される
//   - 同じ basename だけ偶然一致するが参照回数階層が違う別曲は、類似度が大きく下がる
func ComputeMinHash(refCounts map[string]int) MinHashSignature {
	elements := make([]string, 0, len(refCounts)*4)
	for basename, count := range refCounts {
		base := "n:" + basename
		elements = append(elements, base)
		for _, t := range bucketThresholds {
			if count >= t {
				elements = append(elements, base+bucketTagSuffix(t))
			} else {
				break
			}
		}
	}
	return computeMinHashFromElements(elements)
}

func bucketTagSuffix(threshold int) string {
	switch threshold {
	case 1:
		return "#t1"
	case 2:
		return "#t2"
	case 4:
		return "#t4"
	case 8:
		return "#t8"
	case 16:
		return "#t16"
	case 32:
		return "#t32"
	case 64:
		return "#t64"
	default:
		return "#t?"
	}
}

func computeMinHashFromElements(elements []string) MinHashSignature {
	var sig MinHashSignature
	for i := range sig {
		sig[i] = math.MaxUint32
	}
	if len(elements) == 0 {
		return sig
	}
	for _, e := range elements {
		for i := 0; i < MinHashSize; i++ {
			h := fnv.New32a()
			// シードとしてインデックスを書き込み
			_ = binary.Write(h, binary.LittleEndian, uint32(i))
			h.Write([]byte(e))
			v := h.Sum32()
			if v < sig[i] {
				sig[i] = v
			}
		}
	}
	return sig
}
```

- [ ] **Step 4: `parser_test.go` の既存 `ComputeMinHash` 呼び出しを更新**

`internal/domain/bms/parser_test.go` の以下の箇所を `parsed.WAVFiles` から `parsed.WAVRefCounts` に置き換える:

- Line 80: `egoSig := bms.ComputeMinHash(egoParsed.WAVFiles)` → `egoSig := bms.ComputeMinHash(egoParsed.WAVRefCounts)`
- Line 81: `fixSig := bms.ComputeMinHash(fixParsed.WAVFiles)` → `fixSig := bms.ComputeMinHash(fixParsed.WAVRefCounts)`
- Line 103: `dstorvSig := bms.ComputeMinHash(dstorvParsed.WAVFiles)` → `dstorvSig := bms.ComputeMinHash(dstorvParsed.WAVRefCounts)`
- Line 104: `randomSig := bms.ComputeMinHash(randomParsed.WAVFiles)` → `randomSig := bms.ComputeMinHash(randomParsed.WAVRefCounts)`
- Line 114: `sig := bms.ComputeMinHash(nil)` は **変更不要**（`nil` は `map[string]int` の nil として有効）
- Line 128: `sig := bms.ComputeMinHash(parsed.WAVFiles)` → `sig := bms.ComputeMinHash(parsed.WAVRefCounts)`

- [ ] **Step 4.5: `similarity_test.go` の既存 `ComputeMinHash` 呼び出しを更新**

`internal/domain/similarity/similarity_test.go` の以下の 4 箇所を `[]string` リテラルから `map[string]int` リテラルに置き換える。引数の basename 集合は変えず、参照回数はすべて 0 とすることで集合が等価のまま新シグネチャに適合させる:

- Line 78: `sig := bms.ComputeMinHash([]string{"bgm01", "kick", "snare"})` → `sig := bms.ComputeMinHash(map[string]int{"bgm01": 0, "kick": 0, "snare": 0})`
- Line 92: `sigA := bms.ComputeMinHash([]string{"bgm01", "kick", "snare"})` → `sigA := bms.ComputeMinHash(map[string]int{"bgm01": 0, "kick": 0, "snare": 0})`
- Line 93: `sigB := bms.ComputeMinHash([]string{"piano", "bass", "hihat"})` → `sigB := bms.ComputeMinHash(map[string]int{"piano": 0, "bass": 0, "hihat": 0})`
- Line 107: `sig := bms.ComputeMinHash([]string{"bgm01", "kick"})` → `sig := bms.ComputeMinHash(map[string]int{"bgm01": 0, "kick": 0})`

- [ ] **Step 5: `scan_minhash.go` の呼び出しを更新**

`internal/usecase/scan_minhash.go` の line 64 付近 (`sig := bms.ComputeMinHash(parsed.WAVFiles)`) を以下に置き換える:

```go
		sig := bms.ComputeMinHash(parsed.WAVRefCounts)
```

- [ ] **Step 6: `estimate_diff_install.go` の呼び出しを更新**

`internal/usecase/estimate_diff_install.go` の line 90 付近 (`sig := bms.ComputeMinHash(parsed.WAVFiles)`) を以下に置き換える:

```go
	sig := bms.ComputeMinHash(parsed.WAVRefCounts)
```

- [ ] **Step 7: ビルドが通ることを確認**

Run: `go build ./...`
Expected: 成功（コンパイルエラーなし）

- [ ] **Step 8: 新規追加した 3 テストを実行して PASS することを確認**

Run: `go test ./internal/domain/bms/... -run 'TestComputeMinHash_(SameBasenamesDifferentCountsLowerSimilarity|SameBasenamesSimilarCountsHighSimilarity|BasenameOnlyContribution)' -v`
Expected: 3 テスト全て PASS

- [ ] **Step 9: 既存全テストが引き続き通ることを確認**

Run: `go test ./... -count=1`
Expected: 全パッケージ PASS（既存の `TestMinHash_SameSongHighSimilarity` 等が引き続き sim >= 0.9 を満たす）

- [ ] **Step 10: コミット**

```bash
git add internal/domain/bms/minhash.go internal/domain/bms/parser_test.go internal/domain/similarity/similarity_test.go internal/usecase/scan_minhash.go internal/usecase/estimate_diff_install.go
git commit -m "$(cat <<'EOF'
feat(bms): ComputeMinHash を basename + 参照回数累積バケットタグ方式に変更

ComputeMinHash のシグネチャを []string から map[string]int に変更。各 basename
について必ず "n:b" を追加し、参照回数が閾値 (1, 2, 4, 8, 16, 32, 64) 以上の
バケットについて "n:b#tN" を累積的に追加する。これにより、数字命名スキーマで
basename だけ偶然一致する別曲衝突を解消しつつ、同曲難易度差分間の小さな揺れには
耐える。呼び出し元 (scan_minhash, estimate_diff_install) も同時更新。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: PRAGMA user_version ベースで旧 wav_minhash をクリアするマイグレーションを追加

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`
- Modify: `internal/adapter/persistence/migrations_test.go`

`user_version < 2` のときに `chart_meta.wav_minhash IS NOT NULL` の行を一括 NULL クリアし、`PRAGMA user_version = 2` をセットする。冪等。

- [ ] **Step 1: 失敗するマイグレーションテストを追加**

`internal/adapter/persistence/migrations_test.go` の末尾に以下を追加:

```go
func TestRunMigrations_WavMinhashV2Clear(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 初回マイグレーションでテーブル一式を作成
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// v2 マイグレーション以前の状態をシミュレート: user_version=0 に戻し、
	// chart_meta に旧形式 wav_minhash を 1 件入れる
	if _, err := db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	legacy := make([]byte, 256)
	for i := range legacy {
		legacy[i] = byte(i)
	}
	if _, err := db.Exec(
		`INSERT INTO chart_meta (md5, wav_minhash) VALUES ('aabbcc', ?)`,
		legacy,
	); err != nil {
		t.Fatal(err)
	}

	// 再度マイグレーションを走らせると v2 移行が適用される
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// wav_minhash が NULL になっていること
	var stored sql.NullString
	if err := db.QueryRow(
		`SELECT wav_minhash FROM chart_meta WHERE md5 = 'aabbcc'`,
	).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Errorf("wav_minhash should be NULL after v2 migration, got %v", stored)
	}

	// user_version が 2 になっていること
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatal(err)
	}
	if uv != 2 {
		t.Errorf("user_version should be 2, got %d", uv)
	}
}

func TestRunMigrations_WavMinhashV2Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// v2 マイグレーション後の状態で wav_minhash 入りレコードを追加
	// (新アルゴリズムで計算済みの想定)
	freshSig := make([]byte, 256)
	for i := range freshSig {
		freshSig[i] = byte(255 - i)
	}
	if _, err := db.Exec(
		`INSERT INTO chart_meta (md5, wav_minhash) VALUES ('ddeeff', ?)`,
		freshSig,
	); err != nil {
		t.Fatal(err)
	}

	// 2 回目のマイグレーション: user_version はすでに 2 なのでクリアされてはいけない
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	var stored []byte
	if err := db.QueryRow(
		`SELECT wav_minhash FROM chart_meta WHERE md5 = 'ddeeff'`,
	).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 256 {
		t.Fatalf("wav_minhash should be preserved (256 bytes), got %d", len(stored))
	}
	for i, b := range stored {
		if b != byte(255-i) {
			t.Fatalf("byte %d: expected %d, got %d", i, byte(255-i), b)
			break
		}
	}
}
```

- [ ] **Step 2: テストを実行して失敗することを確認**

Run: `go test ./internal/adapter/persistence/... -run 'TestRunMigrations_WavMinhashV2' -v`
Expected: FAIL（`TestRunMigrations_WavMinhashV2Clear` で `user_version should be 2, got 0`）

- [ ] **Step 3: マイグレーションステップを `migrations.go` に追加**

`internal/adapter/persistence/migrations.go` の `RunMigrations` 関数末尾、現状の最後の `return nil` の直前に以下を挿入:

```go
	// wav_minhash アルゴリズム v2 への移行: 旧署名をクリアして再計算待ちにする
	// user_version 管理:
	//   0 = データマイグレーション未適用 (v1 wav_minhash 残存の可能性あり)
	//   1 = 予約 (未使用)
	//   2 = v2 wav_minhash 適用済み
	var userVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if userVersion < 2 {
		if _, err := db.Exec(`UPDATE chart_meta SET wav_minhash = NULL WHERE wav_minhash IS NOT NULL`); err != nil {
			return fmt.Errorf("clear legacy wav_minhash: %w", err)
		}
		if _, err := db.Exec(`PRAGMA user_version = 2`); err != nil {
			return fmt.Errorf("bump user_version: %w", err)
		}
	}

	return nil
```

そして元の `return nil` 行は削除（上記ブロックの末尾の `return nil` が置き換える）。

- [ ] **Step 4: 新規追加した 2 テストが PASS することを確認**

Run: `go test ./internal/adapter/persistence/... -run 'TestRunMigrations_WavMinhashV2' -v`
Expected: 2 テスト両方 PASS

- [ ] **Step 5: 既存マイグレーションテストが引き続き通ることを確認**

Run: `go test ./internal/adapter/persistence/... -v`
Expected: 全テスト PASS（`TestRunMigrations_Idempotent` 等も含む）

- [ ] **Step 6: コミット**

```bash
git add internal/adapter/persistence/migrations.go internal/adapter/persistence/migrations_test.go
git commit -m "$(cat <<'EOF'
feat(persistence): wav_minhash v2 移行のための user_version マイグレーション

PRAGMA user_version で elsa.db のデータマイグレーション世代を管理し、< 2 の場合
は chart_meta.wav_minhash を NULL クリアした上で user_version = 2 をセットする。
冪等。クリア後は起動時の StartMinHashScan が全件を背景再計算する。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 統合確認とプロジェクト全体ビルド

**Files:** 変更なし（ビルド・テスト確認のみ）

- [ ] **Step 1: プロジェクト全体ビルド**

Run: `go build ./...`
Expected: 成功

- [ ] **Step 2: プロジェクト全体テスト**

Run: `go test ./... -count=1`
Expected: 全テスト PASS

- [ ] **Step 3: Windows 用クロスコンパイル（既存 CI と同じ設定）**

Run: `GOOS=windows GOARCH=amd64 go build -o build/bms-elsa.exe .`
Expected: 成功（ファイル生成）

- [ ] **Step 4: 検証 CLI も引き続きビルドできることを確認**

Run: `go build ./cmd/validate-minhash`
Expected: 成功（検証 CLI は独自実装のため本実装と無関係に動く想定）

Note: 検証 CLI は spec の方針通り本実装とは独立に残置。今後のチューニング検証で再利用できる。
