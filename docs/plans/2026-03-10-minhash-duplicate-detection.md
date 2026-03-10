# MinHash重複検知スコアリング統合 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 重複検知のスコアリングにMinHash（WAVファイル集合の類似度）を50%の重みで組み込み、文字列ベースのみの不正確な検知を改善する

**Architecture:** Comparable インターフェースに GetWavMinHash() を追加し、Score() 関数の重み配分を変更（MinHash 50%, Title 20%, Artist 15%, BPM 10%, Genre 5%）。MinHash 未計算のペアはWAVスコア0点で最大50点 → 閾値60で自動フィルタ。データ層では ListSongGroupsForDuplicateScan の SQL に chart_meta を LEFT JOIN して代表 MinHash を取得。

**Tech Stack:** Go 1.24, SQLite

---

## 設計判断

### スコア配分: MinHash なしのフォールバックなし

MinHash が片方でもない場合に従来の重み配分（Title 40% / Artist 30% / BPM 20% / Genre 10%）にフォールバックする選択肢もあったが、MinHash なしは一律 0 点とする:

- MinHash なしの場合の最大スコアは 50 → 閾値 60 で自然にフィルタされる
- 「MinHash スキャン済み」が重複検知の前提条件という明確なルールになる
- フォールバック条件分岐がなく、実装がシンプル

### similarity → bms パッケージ依存

Score() で MinHash 署名のデシリアライズ + Jaccard 計算が必要。bms.MinHashFromBytes + Similarity() を再利用する:

- 同じ domain 層内の依存なのでアーキテクチャ上問題なし
- ロジック複製を避けて DRY を維持

### SQL: LEFT JOIN + MAX() で代表 MinHash 取得

correlated subquery よりシンプルで高速。chart_meta.md5 は UNIQUE なので song 1行に対して最大1行 JOIN、COUNT(*) に影響なし。MAX(BLOB) は任意の非 NULL 値を返すが、同一フォルダ内の譜面は同じ WAV を共有するのでどの署名でも同等。

---

### Task 1: similarity パッケージに MinHash スコアリングを追加

**Files:**
- Modify: `internal/domain/similarity/similarity.go`
- Modify: `internal/domain/similarity/grouping.go`
- Modify: `internal/domain/similarity/similarity_test.go`

**Step 1: テストを更新**

`internal/domain/similarity/similarity_test.go` に MinHash テストケースを追加し、既存テストの期待値を新しい重み配分に合わせる。

testSong に `wavMinHash []byte` フィールドと `GetWavMinHash()` メソッドを追加:

```go
type testSong struct {
	title, artist, genre string
	minBPM, maxBPM       float64
	wavMinHash           []byte
}

func (s testSong) GetTitle() string      { return s.title }
func (s testSong) GetArtist() string     { return s.artist }
func (s testSong) GetGenre() string      { return s.genre }
func (s testSong) GetMinBPM() float64    { return s.minBPM }
func (s testSong) GetMaxBPM() float64    { return s.maxBPM }
func (s testSong) GetWavMinHash() []byte { return s.wavMinHash }
```

既存 TestScore を更新（MinHash なし → 新しい重み配分で Total が変わる）:

```go
func TestScore(t *testing.T) {
	// MinHash なし: Title 58*0.20=11.6, Artist 100*0.15=15, Genre 100*0.05=5, BPM 100*0.10=10 → Total≈41
	a := testSong{"FREEDOM", "xi", "ARROW", 100, 200, nil}
	b := testSong{"FREEDOM DiVE", "xi", "ARROW", 150, 222, nil}
	result := Score(a, b)

	if result.Total < 40 || result.Total > 42 {
		t.Errorf("Total = %d, want ~41 (no MinHash)", result.Total)
	}
	if result.Title < 50 {
		t.Errorf("Title = %d, want >= 50", result.Title)
	}
	if result.Artist != 100 {
		t.Errorf("Artist = %d, want 100", result.Artist)
	}
}
```

MinHash ありのテストを追加:

```go
func TestScoreWithMinHash(t *testing.T) {
	// 同一 MinHash 署名を作成（Jaccard = 1.0）
	sig := bms.ComputeMinHash([]string{"bgm01", "kick", "snare"})
	mh := sig.Bytes()

	a := testSong{"FREEDOM", "xi", "ARROW", 100, 200, mh}
	b := testSong{"FREEDOM DiVE", "xi", "ARROW", 150, 222, mh}
	result := Score(a, b)

	// MinHash 100*0.50=50, Title 58*0.20≈11, Artist 100*0.15=15, Genre 100*0.05=5, BPM 100*0.10=10 → Total≈91
	if result.Total < 89 || result.Total > 93 {
		t.Errorf("Total = %d, want ~91 (identical MinHash)", result.Total)
	}
}

func TestScoreWithDifferentMinHash(t *testing.T) {
	sigA := bms.ComputeMinHash([]string{"bgm01", "kick", "snare"})
	sigB := bms.ComputeMinHash([]string{"piano", "bass", "hihat"})

	a := testSong{"SongA", "ArtistA", "Genre", 120, 120, sigA.Bytes()}
	b := testSong{"SongA", "ArtistA", "Genre", 120, 120, sigB.Bytes()}
	result := Score(a, b)

	// 完全にWAVが異なる → MinHash ≈ 0
	// Title 100*0.20=20, Artist 100*0.15=15, Genre 100*0.05=5, BPM 100*0.10=10 → Total≈50
	if result.Total > 55 {
		t.Errorf("Total = %d, want <= 55 (different MinHash)", result.Total)
	}
}

func TestScoreWithOneMinHashMissing(t *testing.T) {
	sig := bms.ComputeMinHash([]string{"bgm01", "kick"})

	a := testSong{"Song", "Artist", "Genre", 120, 120, sig.Bytes()}
	b := testSong{"Song", "Artist", "Genre", 120, 120, nil}
	result := Score(a, b)

	// 片方 MinHash なし → WAV=0, 残り最大50 → Total=50
	if result.Total > 50 {
		t.Errorf("Total = %d, want <= 50 (one MinHash missing)", result.Total)
	}
}
```

**Step 2: テストが失敗することを確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/similarity/ -v
```

Expected: コンパイルエラー（GetWavMinHash が Comparable にない）

**Step 3: Comparable インターフェースに GetWavMinHash を追加**

`internal/domain/similarity/similarity.go`:

```go
// Comparable は類似度計算に必要なフィールドを持つ型
type Comparable interface {
	GetTitle() string
	GetArtist() string
	GetGenre() string
	GetMinBPM() float64
	GetMaxBPM() float64
	GetWavMinHash() []byte
}
```

**Step 4: SongInfo に WavMinHash フィールドとメソッドを追加**

`internal/domain/similarity/grouping.go`:

```go
type SongInfo struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	ChartCount int
	Path       string
	WavMinHash []byte `json:"-"` // MinHash署名（フロントエンドには送らない）
}

// 既存メソッドの後に追加:
func (s SongInfo) GetWavMinHash() []byte { return s.WavMinHash }
```

**Step 5: Score 関数を更新**

`internal/domain/similarity/similarity.go` の import に `bms` を追加し、Score 関数を書き換え:

```go
import (
	"github.com/meta-BE/bms-elsa/internal/domain/bms"
)
```

```go
// Score は2つのComparableの類似度を計算する
func Score(a, b Comparable) ScoreResult {
	titleR := PrefixRatio(a.GetTitle(), b.GetTitle())
	artistR := PrefixRatio(a.GetArtist(), b.GetArtist())
	genreR := PrefixRatio(a.GetGenre(), b.GetGenre())
	bpmR := BPMOverlap(a.GetMinBPM(), a.GetMaxBPM(), b.GetMinBPM(), b.GetMaxBPM())

	var wavR float64
	mhA, mhB := a.GetWavMinHash(), b.GetWavMinHash()
	if len(mhA) > 0 && len(mhB) > 0 {
		sigA, errA := bms.MinHashFromBytes(mhA)
		sigB, errB := bms.MinHashFromBytes(mhB)
		if errA == nil && errB == nil {
			wavR = sigA.Similarity(sigB)
		}
	}

	total := wavR*0.50 + titleR*0.20 + artistR*0.15 + genreR*0.05 + bpmR*0.10

	return ScoreResult{
		Title:  int(titleR * 100),
		Artist: int(artistR * 100),
		Genre:  int(genreR * 100),
		BPM:    int(bpmR * 100),
		Total:  int(total * 100),
	}
}
```

**Step 6: テストが通ることを確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/similarity/ -v
```

Expected: 全テスト PASS

**Step 7: コミット**

```bash
git add internal/domain/similarity/
git commit -m "feat: 重複検知スコアリングにMinHash類似度を50%の重みで追加"
```

---

### Task 2: データ層に WavMinHash を通す

**Files:**
- Modify: `internal/domain/model/repository.go`
- Modify: `internal/adapter/persistence/songdata_reader.go`
- Modify: `internal/usecase/scan_duplicates.go`
- Modify: `internal/usecase/usecase_test.go`

**Step 1: model.SongGroup に WavMinHash フィールドを追加**

`internal/domain/model/repository.go`:

```go
// SongGroup は重複スキャン用のfolder単位の楽曲情報
type SongGroup struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	ChartCount int
	Path       string // 代表パス（フォルダまで）
	WavMinHash []byte // 代表MinHash署名（未計算ならnil）
}
```

**Step 2: SQL クエリに LEFT JOIN chart_meta を追加**

`internal/adapter/persistence/songdata_reader.go` の `ListSongGroupsForDuplicateScan`:

```go
func (r *SongdataReader) ListSongGroupsForDuplicateScan(ctx context.Context) ([]model.SongGroup, error) {
	query := `
		SELECT
			s.folder,
			s.title,
			s.artist,
			s.genre,
			MIN(s.minbpm) AS minbpm,
			MAX(s.maxbpm) AS maxbpm,
			COUNT(*) AS chart_count,
			MIN(s.path) AS path,
			MAX(cm.wav_minhash) AS wav_minhash
		FROM songdata.song s
		LEFT JOIN chart_meta cm ON s.md5 = cm.md5
		WHERE s.md5 IS NOT NULL AND s.md5 != ''
		GROUP BY s.folder
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListSongGroupsForDuplicateScan: %w", err)
	}
	defer rows.Close()

	var groups []model.SongGroup
	for rows.Next() {
		var g model.SongGroup
		if err := rows.Scan(&g.FolderHash, &g.Title, &g.Artist, &g.Genre,
			&g.MinBPM, &g.MaxBPM, &g.ChartCount, &g.Path, &g.WavMinHash); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
```

**Step 3: scan_duplicates usecase で WavMinHash をコピー**

`internal/usecase/scan_duplicates.go`:

```go
func (u *ScanDuplicatesUseCase) Execute(ctx context.Context) ([]similarity.DuplicateGroup, error) {
	groups, err := u.songRepo.ListSongGroupsForDuplicateScan(ctx)
	if err != nil {
		return nil, err
	}

	songs := make([]similarity.SongInfo, len(groups))
	for i, g := range groups {
		songs[i] = similarity.SongInfo{
			FolderHash: g.FolderHash,
			Title:      g.Title,
			Artist:     g.Artist,
			Genre:      g.Genre,
			MinBPM:     g.MinBPM,
			MaxBPM:     g.MaxBPM,
			ChartCount: g.ChartCount,
			Path:       g.Path,
			WavMinHash: g.WavMinHash,
		}
	}

	return similarity.FindDuplicateGroups(songs, 60), nil
}
```

**Step 4: ビルドとテスト**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./... && go test ./...
```

Expected: 全 PASS

**Step 5: コミット**

```bash
git add internal/domain/model/repository.go internal/adapter/persistence/songdata_reader.go internal/usecase/scan_duplicates.go
git commit -m "feat: 重複スキャンのデータ層にMinHash署名を通す"
```
