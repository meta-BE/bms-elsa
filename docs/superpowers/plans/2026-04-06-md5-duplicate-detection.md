# MD5完全一致による重複検知追加 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重複検知スキャンにMD5完全一致検出を第1段階として追加し、ファジーマッチングと統合したグループを返す

**Architecture:** `SongdataReader` にMD5重複クエリを追加 → `FindDuplicateGroups` を2段階パイプライン化（MD5→ファジー） → フロントのDuplicateDetailでMD5一致表示に対応

**Tech Stack:** Go, SQLite, Svelte 4 (TypeScript)

---

## ファイル構成

| 操作 | ファイル | 責務 |
|------|---------|------|
| 変更 | `internal/domain/model/repository.go` | `MD5DuplicatePair` 構造体追加、`SongRepository` にメソッド追加 |
| 変更 | `internal/adapter/persistence/songdata_reader.go` | `ListMD5DuplicateFolders` 実装 |
| 変更 | `internal/domain/similarity/grouping.go` | `FolderPair` 追加、`DuplicateMember.MD5Match` 追加、`FindDuplicateGroups` を2段階化 |
| 変更 | `internal/domain/similarity/grouping_test.go` | MD5ペア関連のテスト追加、既存テストのシグネチャ修正 |
| 変更 | `internal/usecase/scan_duplicates.go` | MD5ペア取得・変換を追加 |
| 変更 | `frontend/src/views/DuplicateDetail.svelte` | MD5一致時の表示切替 |

---

### Task 1: モデル層 — MD5DuplicatePair とインターフェース追加

**Files:**
- Modify: `internal/domain/model/repository.go:14-18` (DuplicateGroup定義の後)
- Modify: `internal/domain/model/repository.go:51` (SongRepository)

- [ ] **Step 1: `MD5DuplicatePair` 構造体を追加**

`internal/domain/model/repository.go` の `SongGroup` 構造体の後（37行目の後）に追加：

```go
// MD5DuplicatePair は同一MD5を共有するフォルダのペア
type MD5DuplicatePair struct {
	FolderA string
	FolderB string
	MD5     string
}
```

- [ ] **Step 2: `SongRepository` にメソッド追加**

`internal/domain/model/repository.go` の `SongRepository` インターフェース末尾（`ListSongGroupsForDuplicateScan` の後）に追加：

```go
// 同一MD5が複数フォルダに存在するペアを返す
ListMD5DuplicateFolders(ctx context.Context) ([]MD5DuplicatePair, error)
```

- [ ] **Step 3: ビルド確認**

Run: `go build ./...`
Expected: `SongdataReader` が `SongRepository` を満たさないためビルドエラー。これは想定通り。

- [ ] **Step 4: コミット**

```bash
git add internal/domain/model/repository.go
git commit -m "feat: MD5DuplicatePair構造体とSongRepositoryメソッドを追加"
```

---

### Task 2: データ取得層 — ListMD5DuplicateFolders 実装

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go` (ListSongGroupsForDuplicateScan の後に追加)

- [ ] **Step 1: `ListMD5DuplicateFolders` を実装**

`internal/adapter/persistence/songdata_reader.go` の `ListSongGroupsForDuplicateScan` メソッドの後に追加：

```go
// ListMD5DuplicateFolders は同一MD5が複数フォルダに存在するペアを返す
func (r *SongdataReader) ListMD5DuplicateFolders(ctx context.Context) ([]model.MD5DuplicatePair, error) {
	query := `
		SELECT s1.folder, s2.folder, s1.md5
		FROM songdata.song s1
		JOIN songdata.song s2 ON s1.md5 = s2.md5 AND s1.folder < s2.folder
		WHERE s1.md5 IS NOT NULL AND s1.md5 != ''
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListMD5DuplicateFolders: %w", err)
	}
	defer rows.Close()

	var pairs []model.MD5DuplicatePair
	for rows.Next() {
		var p model.MD5DuplicatePair
		if err := rows.Scan(&p.FolderA, &p.FolderB, &p.MD5); err != nil {
			return nil, err
		}
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}
```

- [ ] **Step 2: ビルド確認**

Run: `go build ./...`
Expected: PASS（`SongdataReader` が `SongRepository` を再び満たす）

- [ ] **Step 3: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go
git commit -m "feat: ListMD5DuplicateFolders をSongdataReaderに実装"
```

---

### Task 3: グルーピングロジック — テストを先に書く

**Files:**
- Modify: `internal/domain/similarity/grouping_test.go`

- [ ] **Step 1: 既存テストのシグネチャを修正**

`FindDuplicateGroups` の引数に `nil`（md5Pairs）を追加。既存テストが引き続きパスすることを確認するため。

`internal/domain/similarity/grouping_test.go` の既存テストを修正：

```go
package similarity

import "testing"

func TestFindDuplicateGroups(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 3, Path: "/a"},
		{FolderHash: "bbb", Title: "FREEDOM DiVE", Artist: "xi", Genre: "ARROW", MinBPM: 150, MaxBPM: 222, ChartCount: 1, Path: "/b"},
		{FolderHash: "ccc", Title: "全く別の曲", Artist: "someone", Genre: "POP", MinBPM: 80, MaxBPM: 80, ChartCount: 2, Path: "/c"},
	}

	groups := FindDuplicateGroups(songs, nil, 40)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 2 {
		t.Errorf("len(groups[0].Members) = %d, want 2", len(groups[0].Members))
	}
	if groups[0].Score < 40 {
		t.Errorf("Score = %d, want >= 40", groups[0].Score)
	}
}

func TestFindDuplicateGroups_NoMatch(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "曲A", Artist: "アーティスト1", Genre: "G1", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/a"},
		{FolderHash: "bbb", Title: "曲B", Artist: "アーティスト2", Genre: "G2", MinBPM: 200, MaxBPM: 200, ChartCount: 1, Path: "/b"},
	}

	groups := FindDuplicateGroups(songs, nil, 60)

	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}

func TestFindDuplicateGroups_MD5Only(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "曲A", Artist: "アーティスト1", Genre: "G1", MinBPM: 120, MaxBPM: 120, ChartCount: 2, Path: "/a"},
		{FolderHash: "bbb", Title: "曲A別名", Artist: "アーティスト2", Genre: "G2", MinBPM: 120, MaxBPM: 120, ChartCount: 1, Path: "/b"},
	}
	md5Pairs := []FolderPair{{FolderA: "aaa", FolderB: "bbb"}}

	groups := FindDuplicateGroups(songs, md5Pairs, 60)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if groups[0].Score != 100 {
		t.Errorf("Score = %d, want 100", groups[0].Score)
	}
	// 両メンバーがMD5Matchフラグを持つ
	for _, m := range groups[0].Members {
		if !m.MD5Match {
			t.Errorf("member %s: MD5Match = false, want true", m.FolderHash)
		}
	}
}

func TestFindDuplicateGroups_MD5PlusFuzzy(t *testing.T) {
	// A-B: MD5一致、B-C: ファジーマッチ（同一アーティスト+類似タイトル） → {A, B, C}
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 2, Path: "/a"},
		{FolderHash: "bbb", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 1, Path: "/b"},
		{FolderHash: "ccc", Title: "FREEDOM DiVE", Artist: "xi", Genre: "ARROW", MinBPM: 150, MaxBPM: 222, ChartCount: 1, Path: "/c"},
	}
	md5Pairs := []FolderPair{{FolderA: "aaa", FolderB: "bbb"}}

	groups := FindDuplicateGroups(songs, md5Pairs, 40)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 3 {
		t.Errorf("len(Members) = %d, want 3", len(groups[0].Members))
	}
	if groups[0].Score != 100 {
		t.Errorf("Score = %d, want 100", groups[0].Score)
	}
	// MD5Matchフラグの確認：aaa, bbbはtrue、cccはfalse
	md5MatchMap := map[string]bool{}
	for _, m := range groups[0].Members {
		md5MatchMap[m.FolderHash] = m.MD5Match
	}
	if !md5MatchMap["aaa"] {
		t.Error("aaa: MD5Match = false, want true")
	}
	if !md5MatchMap["bbb"] {
		t.Error("bbb: MD5Match = false, want true")
	}
	if md5MatchMap["ccc"] {
		t.Error("ccc: MD5Match = true, want false")
	}
}

func TestFindDuplicateGroups_MD5SkipsFuzzy(t *testing.T) {
	// 同じペアがMD5で一致している場合、ファジースコア計算はスキップされる
	// → MD5一致メンバーのScoresはTotal=100、WAV/Title等の内訳は0
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "SAME", Artist: "SAME", Genre: "SAME", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/a"},
		{FolderHash: "bbb", Title: "SAME", Artist: "SAME", Genre: "SAME", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/b"},
	}
	md5Pairs := []FolderPair{{FolderA: "aaa", FolderB: "bbb"}}

	groups := FindDuplicateGroups(songs, md5Pairs, 60)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	// ファジースコア計算がスキップされるため、ScoresのTotal=100だがWAV等は0
	for _, m := range groups[0].Members {
		if m.Scores.Total != 100 {
			t.Errorf("member %s: Scores.Total = %d, want 100", m.FolderHash, m.Scores.Total)
		}
		if m.Scores.WAV != 0 {
			t.Errorf("member %s: Scores.WAV = %d, want 0 (fuzzy skipped)", m.FolderHash, m.Scores.WAV)
		}
	}
}
```

- [ ] **Step 2: テスト実行して失敗を確認**

Run: `go test ./internal/domain/similarity/ -v -run TestFindDuplicateGroups`
Expected: コンパイルエラー（`FindDuplicateGroups` の引数が合わない、`FolderPair` 未定義）

- [ ] **Step 3: コミット**

```bash
git add internal/domain/similarity/grouping_test.go
git commit -m "test: MD5ペア対応のグルーピングテストを追加"
```

---

### Task 4: グルーピングロジック — FindDuplicateGroups を2段階パイプライン化

**Files:**
- Modify: `internal/domain/similarity/grouping.go`

- [ ] **Step 1: `FolderPair` 型と `DuplicateMember.MD5Match` を追加**

`internal/domain/similarity/grouping.go` の `DuplicateMember` 定義を以下に変更し、`FolderPair` を追加：

```go
// FolderPair はMD5完全一致で検出されたフォルダのペア
type FolderPair struct {
	FolderA string
	FolderB string
}

// DuplicateMember はグループ内の各楽曲
type DuplicateMember struct {
	SongInfo
	Scores   ScoreResult // グループ内で最も類似度が高いペアのスコア
	MD5Match bool        // MD5完全一致で検出されたか
}
```

- [ ] **Step 2: `FindDuplicateGroups` を2段階パイプラインに書き換え**

`internal/domain/similarity/grouping.go` の `FindDuplicateGroups` 関数全体を以下に置き換え：

```go
// FindDuplicateGroups はMD5完全一致→ファジー比較の2段階で重複グループを返す
func FindDuplicateGroups(songs []SongInfo, md5Pairs []FolderPair, threshold int) []DuplicateGroup {
	// FolderHash → index マップ
	folderIdx := make(map[string]int, len(songs))
	for i, s := range songs {
		folderIdx[s.FolderHash] = i
	}

	// Union-Find
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

	bestScore := make(map[int]ScoreResult)
	md5Matched := make(map[[2]int]bool) // MD5一致済みペア
	md5MatchedIdx := make(map[int]bool) // MD5一致に関与したindex

	// --- 第1段階: MD5完全一致ペア ---
	md5Score := ScoreResult{Total: 100}
	for _, p := range md5Pairs {
		idxA, okA := folderIdx[p.FolderA]
		idxB, okB := folderIdx[p.FolderB]
		if !okA || !okB {
			continue
		}
		union(idxA, idxB)
		if md5Score.Total > bestScore[idxA].Total {
			bestScore[idxA] = md5Score
		}
		if md5Score.Total > bestScore[idxB].Total {
			bestScore[idxB] = md5Score
		}
		key := [2]int{idxA, idxB}
		if idxA > idxB {
			key = [2]int{idxB, idxA}
		}
		md5Matched[key] = true
		md5MatchedIdx[idxA] = true
		md5MatchedIdx[idxB] = true
	}

	// --- 第2段階: ファジーマッチ ---

	// ブロッキング: artist正規化でグループ化
	blocks := make(map[string][]int)
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
			key := "__title__" + strings.ToLower(strings.TrimSpace(songs[indices[0]].Title))
			blocks[key] = indices
		}
	}

	// グループ内でペア比較（MD5一致済みペアはスキップ）
	for _, indices := range blocks {
		if len(indices) < 2 {
			continue
		}
		for a := 0; a < len(indices); a++ {
			for b := a + 1; b < len(indices); b++ {
				idxA, idxB := indices[a], indices[b]
				key := [2]int{idxA, idxB}
				if idxA > idxB {
					key = [2]int{idxB, idxA}
				}
				if md5Matched[key] {
					continue
				}
				result := Score(songs[idxA], songs[idxB])
				if result.Total >= threshold {
					union(idxA, idxB)
					if result.Total > bestScore[idxA].Total {
						bestScore[idxA] = result
					}
					if result.Total > bestScore[idxB].Total {
						bestScore[idxB] = result
					}
				}
			}
		}
	}

	// グループ集約
	groupMap := make(map[int][]int)
	// MD5一致ペアのルートを登録
	for p := range md5Matched {
		groupMap[find(p[0])] = nil
		groupMap[find(p[1])] = nil
	}
	// ファジーマッチのルートを登録
	for i := range songs {
		root := find(i)
		if _, ok := groupMap[root]; ok {
			continue
		}
		if bestScore[i].Total > 0 {
			groupMap[root] = nil
		}
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
				MD5Match: md5MatchedIdx[idx],
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

- [ ] **Step 3: テスト実行**

Run: `go test ./internal/domain/similarity/ -v -run TestFindDuplicateGroups`
Expected: 全テストPASS

- [ ] **Step 4: ビルド確認**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/similarity/grouping.go
git commit -m "feat: FindDuplicateGroupsをMD5+ファジーの2段階パイプラインに変更"
```

---

### Task 5: ユースケース層 — MD5ペア取得の統合

**Files:**
- Modify: `internal/usecase/scan_duplicates.go`

- [ ] **Step 1: `Execute` メソッドにMD5ペア取得を追加**

`internal/usecase/scan_duplicates.go` 全体を以下に置き換え：

```go
package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
)

const defaultThreshold = 60

type ScanDuplicatesUseCase struct {
	songRepo model.SongRepository
}

func NewScanDuplicatesUseCase(songRepo model.SongRepository) *ScanDuplicatesUseCase {
	return &ScanDuplicatesUseCase{songRepo: songRepo}
}

func (u *ScanDuplicatesUseCase) Execute(ctx context.Context) ([]similarity.DuplicateGroup, error) {
	md5Pairs, err := u.songRepo.ListMD5DuplicateFolders(ctx)
	if err != nil {
		return nil, err
	}

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

	folderPairs := make([]similarity.FolderPair, len(md5Pairs))
	for i, p := range md5Pairs {
		folderPairs[i] = similarity.FolderPair{FolderA: p.FolderA, FolderB: p.FolderB}
	}

	return similarity.FindDuplicateGroups(songs, folderPairs, defaultThreshold), nil
}
```

- [ ] **Step 2: ビルド確認**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: コミット**

```bash
git add internal/usecase/scan_duplicates.go
git commit -m "feat: ScanDuplicatesUseCaseにMD5ペア取得を統合"
```

---

### Task 6: フロントエンド — DuplicateDetail のMD5一致表示

**Files:**
- Modify: `frontend/src/views/DuplicateDetail.svelte`

- [ ] **Step 1: 類似度内訳セクションをMD5一致対応に変更**

`frontend/src/views/DuplicateDetail.svelte` の類似度内訳セクション（167〜179行目）を以下に置き換え：

```svelte
    {#if group.Members.length >= 2}
      {@const hasMD5Match = group.Members.some(m => m.MD5Match)}
      {@const fuzzyMembers = group.Members.filter(m => !m.MD5Match)}
      <div class="text-base-content/60 space-y-1">
        {#if hasMD5Match}
          <div class="text-sm"><span class="badge badge-sm badge-success">MD5一致</span></div>
        {/if}
        {#if fuzzyMembers.length > 0}
          {@const scores = fuzzyMembers[0].Scores}
          <div class="text-sm font-semibold">類似度内訳</div>
          <div class="text-sm flex gap-4">
            <span>WAV定義 {scores.WAV}%</span>
            <span>title {scores.Title}%</span>
            <span>artist {scores.Artist}%</span>
            <span>genre {scores.Genre}%</span>
            <span>BPM {scores.BPM}%</span>
          </div>
        {/if}
      </div>
    {/if}
```

表示ロジック：
- MD5一致メンバーがいる場合：「MD5一致」バッジを表示
- ファジーマッチのみのメンバーがいる場合：従来の類似度内訳を表示
- 両方混在する場合：バッジ + 内訳の両方を表示
- 全員MD5一致の場合：バッジのみ（内訳は非表示）

- [ ] **Step 2: フロントエンドビルド確認**

Run: `cd frontend && npm run build`
Expected: PASS（Wails型はバックエンド変更後に再生成が必要だが、`MD5Match` は `any` として扱われるためビルドは通る）

- [ ] **Step 3: コミット**

```bash
git add frontend/src/views/DuplicateDetail.svelte
git commit -m "feat: DuplicateDetailにMD5一致表示を追加"
```

---

### Task 7: 統合確認

- [ ] **Step 1: Go全体テスト**

Run: `go test ./...`
Expected: 全テストPASS

- [ ] **Step 2: Go全体ビルド**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: フロントエンドビルド**

Run: `cd frontend && npm run build`
Expected: PASS
