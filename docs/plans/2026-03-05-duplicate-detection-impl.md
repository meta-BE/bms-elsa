# 重複検知機能 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 異なるfolderハッシュ間で類似楽曲パッケージをファジーマッチングで検出し、専用ビューで一覧表示する

**Architecture:** Go側でartist正規化によるブロッキング→グループ内類似度計算。Svelte側で専用「重複検知」タブを追加し、上下SplitPaneでグループ一覧+詳細を表示

**Tech Stack:** Go (SQLite + ファジーマッチング), Svelte + TailwindCSS/DaisyUI, Wails

**設計書:** `docs/plans/2026-03-05-duplicate-detection-design.md`

---

### Task 1: 類似度計算のユーティリティ関数

**Files:**
- Create: `internal/domain/similarity/similarity.go`
- Create: `internal/domain/similarity/similarity_test.go`

**Step 1: テストを書く**

```go
package similarity

import "testing"

func TestPrefixRatio(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"FREEDOM", "FREEDOM DiVE", 0.583}, // 7/12
		{"ABC", "ABC", 1.0},
		{"ABC", "XYZ", 0.0},
		{"", "", 0.0},   // 両方空は0
		{"A", "", 0.0},  // 片方空は0
	}
	for _, tt := range tests {
		got := PrefixRatio(tt.a, tt.b)
		if diff := got - tt.want; diff > 0.01 || diff < -0.01 {
			t.Errorf("PrefixRatio(%q, %q) = %.3f, want %.3f", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBPMOverlap(t *testing.T) {
	tests := []struct {
		minA, maxA, minB, maxB float64
		want                   float64
	}{
		{120, 180, 150, 200, 1.0}, // 重なりあり
		{120, 130, 140, 150, 0.0}, // 重なりなし
		{100, 100, 100, 100, 1.0}, // 完全一致
	}
	for _, tt := range tests {
		got := BPMOverlap(tt.minA, tt.maxA, tt.minB, tt.maxB)
		if got != tt.want {
			t.Errorf("BPMOverlap(%.0f-%.0f, %.0f-%.0f) = %.1f, want %.1f",
				tt.minA, tt.maxA, tt.minB, tt.maxB, got, tt.want)
		}
	}
}

type testSong struct {
	title, artist, genre string
	minBPM, maxBPM       float64
}

func (s testSong) GetTitle() string   { return s.title }
func (s testSong) GetArtist() string  { return s.artist }
func (s testSong) GetGenre() string   { return s.genre }
func (s testSong) GetMinBPM() float64 { return s.minBPM }
func (s testSong) GetMaxBPM() float64 { return s.maxBPM }

func TestScore(t *testing.T) {
	a := testSong{"FREEDOM", "xi", "ARROW", 100, 200}
	b := testSong{"FREEDOM DiVE", "xi", "ARROW", 150, 222}
	result := Score(a, b)

	// 全フィールドで高い類似度 → 総合スコアも高い
	if result.Total < 50 {
		t.Errorf("Total = %d, want >= 50", result.Total)
	}
	if result.Title < 50 {
		t.Errorf("Title = %d, want >= 50", result.Title)
	}
	if result.Artist != 100 {
		t.Errorf("Artist = %d, want 100", result.Artist)
	}
}
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/similarity/ -v`
Expected: FAIL (パッケージが存在しない)

**Step 3: 実装**

```go
package similarity

// Comparable は類似度計算に必要なフィールドを持つ型
type Comparable interface {
	GetTitle() string
	GetArtist() string
	GetGenre() string
	GetMinBPM() float64
	GetMaxBPM() float64
}

// ScoreResult は各フィールドの類似度（%）と総合スコア
type ScoreResult struct {
	Title  int // 0-100
	Artist int // 0-100
	Genre  int // 0-100
	BPM    int // 0 or 100
	Total  int // 加重平均 0-100
}

// PrefixRatio は2つの文字列の前方一致文字数率を返す（0.0〜1.0）
func PrefixRatio(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 0
	}
	match := 0
	minLen := len(ra)
	if len(rb) < minLen {
		minLen = len(rb)
	}
	for i := 0; i < minLen; i++ {
		if ra[i] != rb[i] {
			break
		}
		match++
	}
	return float64(match) / float64(maxLen)
}

// BPMOverlap はBPM範囲が重なっていれば1.0、そうでなければ0.0を返す
func BPMOverlap(minA, maxA, minB, maxB float64) float64 {
	if minA <= maxB && minB <= maxA {
		return 1.0
	}
	return 0.0
}

// Score は2つのComparableの類似度を計算する
func Score(a, b Comparable) ScoreResult {
	titleR := PrefixRatio(a.GetTitle(), b.GetTitle())
	artistR := PrefixRatio(a.GetArtist(), b.GetArtist())
	genreR := PrefixRatio(a.GetGenre(), b.GetGenre())
	bpmR := BPMOverlap(a.GetMinBPM(), a.GetMaxBPM(), b.GetMinBPM(), b.GetMaxBPM())

	total := titleR*0.4 + artistR*0.3 + genreR*0.1 + bpmR*0.2

	return ScoreResult{
		Title:  int(titleR * 100),
		Artist: int(artistR * 100),
		Genre:  int(genreR * 100),
		BPM:    int(bpmR * 100),
		Total:  int(total * 100),
	}
}
```

**Step 4: テストがパスすることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/similarity/ -v`
Expected: PASS

**Step 5: コミット**

```bash
git add internal/domain/similarity/
git commit -m "feat: 類似度計算ユーティリティを追加"
```

---

### Task 2: 重複スキャンのバックエンドロジック

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go` (メソッド追加)
- Modify: `internal/adapter/persistence/songdata_reader_test.go` (テスト追加)

**Step 1: テストを書く**

`songdata_reader_test.go` に追加:

```go
func TestListSongGroupsForDuplicateScan(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	groups, err := reader.ListSongGroupsForDuplicateScan(ctx)
	if err != nil {
		t.Fatalf("ListSongGroupsForDuplicateScan failed: %v", err)
	}

	// songdata.dbに楽曲が存在する
	if len(groups) == 0 {
		t.Fatal("expected at least one song group")
	}

	// 各グループの必須フィールドが埋まっている
	for i, g := range groups {
		if g.FolderHash == "" {
			t.Errorf("groups[%d].FolderHash is empty", i)
		}
		if g.Title == "" {
			t.Errorf("groups[%d].Title is empty", i)
		}
	}
}
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListSongGroupsForDuplicateScan -v`
Expected: FAIL (メソッドが存在しない)

**Step 3: 実装**

`songdata_reader.go` に追加:

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
}

// ListSongGroupsForDuplicateScan はfolder単位で楽曲グループを返す（重複スキャン用）
func (r *SongdataReader) ListSongGroupsForDuplicateScan(ctx context.Context) ([]SongGroup, error) {
	query := `
		SELECT
			s.folder,
			s.title,
			s.artist,
			s.genre,
			MIN(s.minbpm) AS minbpm,
			MAX(s.maxbpm) AS maxbpm,
			COUNT(*) AS chart_count,
			MIN(s.path) AS path
		FROM songdata.song s
		WHERE s.md5 IS NOT NULL AND s.md5 != ''
		GROUP BY s.folder
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListSongGroupsForDuplicateScan: %w", err)
	}
	defer rows.Close()

	var groups []SongGroup
	for rows.Next() {
		var g SongGroup
		if err := rows.Scan(&g.FolderHash, &g.Title, &g.Artist, &g.Genre,
			&g.MinBPM, &g.MaxBPM, &g.ChartCount, &g.Path); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
```

**Step 4: テストがパスすることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListSongGroupsForDuplicateScan -v`
Expected: PASS

**Step 5: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go internal/adapter/persistence/songdata_reader_test.go
git commit -m "feat: 重複スキャン用のfolder単位楽曲グループ取得を追加"
```

---

### Task 3: 重複スキャンのグルーピングロジック

**Files:**
- Create: `internal/domain/similarity/grouping.go`
- Create: `internal/domain/similarity/grouping_test.go`

**Step 1: テストを書く**

```go
package similarity

import "testing"

func TestFindDuplicateGroups(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 3, Path: "/a"},
		{FolderHash: "bbb", Title: "FREEDOM DiVE", Artist: "xi", Genre: "ARROW", MinBPM: 150, MaxBPM: 222, ChartCount: 1, Path: "/b"},
		{FolderHash: "ccc", Title: "全く別の曲", Artist: "someone", Genre: "POP", MinBPM: 80, MaxBPM: 80, ChartCount: 2, Path: "/c"},
	}

	groups := FindDuplicateGroups(songs, 60)

	// aaa と bbb はxi作のFREEDOM系で重複グループになるはず
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 2 {
		t.Errorf("len(groups[0].Members) = %d, want 2", len(groups[0].Members))
	}
	if groups[0].Score < 60 {
		t.Errorf("Score = %d, want >= 60", groups[0].Score)
	}
}

func TestFindDuplicateGroups_NoMatch(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "曲A", Artist: "アーティスト1", Genre: "G1", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/a"},
		{FolderHash: "bbb", Title: "曲B", Artist: "アーティスト2", Genre: "G2", MinBPM: 200, MaxBPM: 200, ChartCount: 1, Path: "/b"},
	}

	groups := FindDuplicateGroups(songs, 60)

	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/similarity/ -run TestFindDuplicateGroups -v`
Expected: FAIL

**Step 3: 実装**

```go
package similarity

import "strings"

// SongInfo は重複スキャン対象の楽曲情報
type SongInfo struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	ChartCount int
	Path       string
}

func (s SongInfo) GetTitle() string   { return s.Title }
func (s SongInfo) GetArtist() string  { return s.Artist }
func (s SongInfo) GetGenre() string   { return s.Genre }
func (s SongInfo) GetMinBPM() float64 { return s.MinBPM }
func (s SongInfo) GetMaxBPM() float64 { return s.MaxBPM }

// DuplicateGroup は重複候補のグループ
type DuplicateGroup struct {
	ID      int
	Members []DuplicateMember
	Score   int // グループ内の最高類似度（%）
}

// DuplicateMember はグループ内の各楽曲
type DuplicateMember struct {
	SongInfo
	Scores ScoreResult // グループ内で最も類似度が高いペアのスコア
}

// FindDuplicateGroups はartist正規化によるブロッキング→グループ内ファジー比較で重複グループを返す
func FindDuplicateGroups(songs []SongInfo, threshold int) []DuplicateGroup {
	// ブロッキング: artist正規化でグループ化
	blocks := make(map[string][]int) // normArtist -> songインデックス群
	for i, s := range songs {
		key := strings.ToLower(strings.TrimSpace(s.Artist))
		blocks[key] = append(blocks[key], i)
	}

	// title完全一致ブロッキングも追加
	titleBlocks := make(map[string][]int)
	for i, s := range songs {
		key := strings.ToLower(strings.TrimSpace(s.Title))
		titleBlocks[key] = append(titleBlocks[key], i)
	}
	for _, indices := range titleBlocks {
		if len(indices) >= 2 {
			// 既存ブロックとマージ（重複排除はペア比較時に行う）
			key := "__title__" + strings.ToLower(strings.TrimSpace(songs[indices[0]].Title))
			blocks[key] = indices
		}
	}

	// グループ内でペア比較
	type pair struct{ i, j int }
	matched := make(map[pair]ScoreResult)
	for _, indices := range blocks {
		if len(indices) < 2 {
			continue
		}
		for a := 0; a < len(indices); a++ {
			for b := a + 1; b < len(indices); b++ {
				p := pair{indices[a], indices[b]}
				if _, ok := matched[p]; ok {
					continue
				}
				result := Score(songs[p.i], songs[p.j])
				if result.Total >= threshold {
					matched[p] = result
				}
			}
		}
	}

	// Union-Findでグループ化
	parent := make([]int, len(songs))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(x, y int) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	bestScore := make(map[int]ScoreResult) // songIndex -> そのsongの最高スコアペア
	for p, score := range matched {
		union(p.i, p.j)
		if score.Total > bestScore[p.i].Total {
			bestScore[p.i] = score
		}
		if score.Total > bestScore[p.j].Total {
			bestScore[p.j] = score
		}
	}

	// グループ集約
	groupMap := make(map[int][]int) // root -> songインデックス群
	for p := range matched {
		groupMap[find(p.i)] = nil
		groupMap[find(p.j)] = nil
	}
	for i := range songs {
		root := find(i)
		if _, ok := groupMap[root]; ok {
			groupMap[root] = append(groupMap[root], i)
		}
	}

	var groups []DuplicateGroup
	id := 1
	for _, indices := range groupMap {
		if len(indices) < 2 {
			continue
		}
		g := DuplicateGroup{ID: id}
		for _, idx := range indices {
			g.Members = append(g.Members, DuplicateMember{
				SongInfo: songs[idx],
				Scores:   bestScore[idx],
			})
			if bestScore[idx].Total > g.Score {
				g.Score = bestScore[idx].Total
			}
		}
		groups = append(groups, g)
		id++
	}

	return groups
}
```

**Step 4: テストがパスすることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/similarity/ -v`
Expected: PASS

**Step 5: コミット**

```bash
git add internal/domain/similarity/
git commit -m "feat: ブロッキング+Union-Findによる重複グルーピングを追加"
```

---

### Task 4: app.goにScanDuplicatesエンドポイントを追加

**Files:**
- Modify: `app.go` (エンドポイント追加)

**Step 1: ScanDuplicatesメソッドを実装**

`app.go` に追加:

```go
import "bms-elsa/internal/domain/similarity"

// ScanDuplicates は楽曲の重複スキャンを実行する
func (a *App) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	groups, err := a.songReader.ListSongGroupsForDuplicateScan(a.ctx)
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
		}
	}

	return similarity.FindDuplicateGroups(songs, 60), nil
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add app.go
git commit -m "feat: ScanDuplicatesエンドポイントを追加"
```

---

### Task 5: フロントエンド — DuplicateView.svelteコンポーネント作成

**Files:**
- Create: `frontend/src/DuplicateView.svelte`

**Step 1: 上ペイン（グループ一覧）を実装**

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { ScanDuplicates } from '../wailsjs/go/main/App'

  const dispatch = createEventDispatcher()

  export let active = false

  type ScoreResult = {
    Title: number
    Artist: number
    Genre: number
    BPM: number
    Total: number
  }

  type DuplicateMember = {
    FolderHash: string
    Title: string
    Artist: string
    Genre: string
    MinBPM: number
    MaxBPM: number
    ChartCount: number
    Path: string
    Scores: ScoreResult
  }

  type DuplicateGroup = {
    ID: number
    Members: DuplicateMember[]
    Score: number
  }

  let groups: DuplicateGroup[] = []
  let scanning = false
  let scanned = false
  let selectedGroupID: number | null = null

  async function handleScan() {
    scanning = true
    try {
      const result = await ScanDuplicates()
      groups = (result || []).sort((a, b) => b.Score - a.Score)
      scanned = true
    } finally {
      scanning = false
    }
  }

  function handleSelect(group: DuplicateGroup) {
    selectedGroupID = group.ID
    dispatch('select', group)
  }

  $: selectedGroup = groups.find(g => g.ID === selectedGroupID) || null
</script>

{#if !scanned}
  <div class="flex items-center justify-center h-full">
    <button class="btn btn-primary" on:click={handleScan} disabled={scanning}>
      {scanning ? 'スキャン中...' : 'スキャン実行'}
    </button>
  </div>
{:else}
  <div class="flex items-center gap-2 px-2 py-1 text-xs text-base-content/60 border-b border-base-300">
    <button class="btn btn-xs btn-ghost" on:click={handleScan} disabled={scanning}>
      {scanning ? '...' : '再スキャン'}
    </button>
    <span>{groups.length} グループ</span>
  </div>
  <div class="overflow-y-auto h-full">
    <table class="table table-xs table-pin-rows">
      <thead>
        <tr>
          <th class="w-16">類似度</th>
          <th>タイトル</th>
          <th class="w-16">件数</th>
        </tr>
      </thead>
      <tbody>
        {#each groups as group}
          <tr
            class="cursor-pointer hover:bg-base-200"
            class:bg-primary/10={selectedGroupID === group.ID}
            on:click={() => handleSelect(group)}
          >
            <td class="font-mono">{group.Score}%</td>
            <td>{group.Members[0]?.Title || ''}</td>
            <td>{group.Members.length}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run check`
Expected: 成功（型エラーなし）

**Step 3: コミット**

```bash
git add frontend/src/DuplicateView.svelte
git commit -m "feat: DuplicateView.svelteの上ペイン（グループ一覧）を作成"
```

---

### Task 6: フロントエンド — DuplicateDetail.svelteコンポーネント作成

**Files:**
- Create: `frontend/src/DuplicateDetail.svelte`

**Step 1: 下ペイン（グループ詳細）を実装**

```svelte
<script lang="ts">
  export let group: {
    ID: number
    Score: number
    Members: {
      FolderHash: string
      Title: string
      Artist: string
      Genre: string
      MinBPM: number
      MaxBPM: number
      ChartCount: number
      Path: string
      Scores: { Title: number; Artist: number; Genre: number; BPM: number; Total: number }
    }[]
  } | null = null

  function formatBPM(min: number, max: number): string {
    if (min === max) return String(Math.round(min))
    return `${Math.round(min)}-${Math.round(max)}`
  }

  function folderPath(path: string): string {
    const sep = path.includes('\\') ? '\\' : '/'
    const parts = path.split(sep)
    parts.pop()
    return parts.join(sep)
  }
</script>

{#if group}
  <div class="p-3 space-y-3">
    <div class="flex items-center gap-2 text-sm font-semibold">
      <span>グループ #{group.ID}</span>
      <span class="badge badge-sm badge-primary">{group.Score}%</span>
    </div>

    {#each group.Members as member, i}
      <div class="card card-compact bg-base-200">
        <div class="card-body">
          <div class="flex items-start justify-between">
            <div>
              <div class="font-semibold">{member.Title}</div>
              <div class="text-sm text-base-content/70">{member.Artist}</div>
            </div>
            <div class="text-right text-xs text-base-content/50">
              <div>{member.Genre}</div>
              <div>BPM {formatBPM(member.MinBPM, member.MaxBPM)}</div>
              <div>{member.ChartCount}譜面</div>
            </div>
          </div>
          <div class="text-xs text-base-content/40 break-all">{folderPath(member.Path)}</div>
        </div>
      </div>
    {/each}

    {#if group.Members.length >= 2}
      <div class="text-xs text-base-content/60 space-y-1">
        <div class="font-semibold">類似度内訳</div>
        {@const scores = group.Members[0].Scores}
        <div class="flex gap-4">
          <span>title {scores.Title}%</span>
          <span>artist {scores.Artist}%</span>
          <span>genre {scores.Genre}%</span>
          <span>BPM {scores.BPM}%</span>
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="flex items-center justify-center h-full text-base-content/40 text-sm">
    グループを選択してください
  </div>
{/if}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run check`
Expected: 成功

**Step 3: コミット**

```bash
git add frontend/src/DuplicateDetail.svelte
git commit -m "feat: DuplicateDetail.svelteの詳細表示を作成"
```

---

### Task 7: App.svelteに重複検知タブを追加

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: タブ追加**

- `activeTab` の型に `'duplicates'` を追加
- import に `DuplicateView` と `DuplicateDetail` を追加
- タブバーに「重複検知」ボタンを追加
- タブコンテンツに `SplitPane` + `DuplicateView` + `DuplicateDetail` を追加

変更箇所:

1. import追加:
```typescript
import DuplicateView from './DuplicateView.svelte'
import DuplicateDetail from './DuplicateDetail.svelte'
```

2. 型と状態:
```typescript
let activeTab: 'songs' | 'charts' | 'difficulty' | 'duplicates' = 'songs'
let selectedDuplicateGroup: DuplicateGroup | null = null
```

3. タブバーに追加:
```svelte
<button class="tab" class:tab-active={activeTab === 'duplicates'} on:click={() => switchTab('duplicates')}>重複検知</button>
```

4. タブコンテンツに追加:
```svelte
{:else if activeTab === 'duplicates'}
  <SplitPane showDetail={!!selectedDuplicateGroup} bind:splitRatio>
    <DuplicateView slot="list" active={activeTab === 'duplicates'} on:select={(e) => selectedDuplicateGroup = e.detail} />
    <svelte:fragment slot="detail">
      {#if selectedDuplicateGroup}
        <DuplicateDetail group={selectedDuplicateGroup} />
      {/if}
    </svelte:fragment>
  </SplitPane>
```

**Step 2: Wails bindingsを再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run check`
Expected: 成功

**Step 4: コミット**

```bash
git add frontend/src/App.svelte frontend/wailsjs/
git commit -m "feat: App.svelteに重複検知タブを追加"
```

---

### Task 8: 統合テスト — wails devで動作確認

**Step 1: dev起動して動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認項目:
- [ ] 「重複検知」タブが表示される
- [ ] 「スキャン実行」ボタンが表示される
- [ ] スキャン実行で結果が表示される（グループ一覧）
- [ ] グループ選択で下ペインに詳細が表示される
- [ ] 類似度%とフィールド内訳が正しく表示される

**Step 2: 問題があれば修正してコミット**

```bash
git add -A
git commit -m "fix: 統合テストで発見した問題を修正"
```
