# 導入先推定ロジック改善 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 導入先推定に「ベースタイトル一致」「アーティスト一致」を追加し、スコアリングで結果をランク付けする。

**Architecture:** 既存のEstimateInstallLocationUseCaseにベースタイトル抽出ロジックとアーティスト検索を追加。SongRepositoryにFindChartFoldersByArtistを追加。InstallCandidateにScoreフィールドを追加し、結果をスコア降順でソート。

**Tech Stack:** Go, SQLite, Svelte 4, TypeScript, Wails v2

---

### Task 1: InstallCandidateにScoreフィールドを追加

**Files:**
- Modify: `internal/domain/model/song.go`

**Step 1: InstallCandidate構造体にScoreフィールドを追加**

現在の構造体（song.go末尾）:
```go
// InstallCandidate は導入先推定の候補
type InstallCandidate struct {
	FolderPath string   // 楽曲フォルダのパス（songdata.songのpath/folderから導出）
	Title      string   // フォルダ内の代表タイトル
	Artist     string   // フォルダ内の代表アーティスト
	MatchTypes []string // マッチ理由: "title", "body_url"
}
```

変更後:
```go
// InstallCandidate は導入先推定の候補
type InstallCandidate struct {
	FolderPath string   // 楽曲フォルダのパス（songdata.songのpath/folderから導出）
	Title      string   // フォルダ内の代表タイトル
	Artist     string   // フォルダ内の代表アーティスト
	MatchTypes []string // マッチ理由: "title", "base_title", "body_url", "artist"
	Score      int      // マッチ手法のスコア合算
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/domain/model/song.go
git commit -m "feat: InstallCandidateにScoreフィールドを追加"
```

---

### Task 2: SongRepositoryにFindChartFoldersByArtistを追加

**Files:**
- Modify: `internal/domain/model/repository.go`
- Modify: `internal/adapter/persistence/songdata_reader.go`

**Step 1: SongRepositoryインターフェースにメソッドを追加**

`repository.go`の`SongRepository`、`FindChartFoldersByBodyURL`の後に追加:

```go
	// アーティスト完全一致（大文字小文字無視）で導入済み譜面をfolder単位で検索
	FindChartFoldersByArtist(ctx context.Context, artist string) ([]InstallCandidate, error)
```

**Step 2: songdata_reader.goに実装を追加**

`songdata_reader.go`の`FindChartFoldersByBodyURL`メソッドの後に追加:

```go
func (r *SongdataReader) FindChartFoldersByArtist(ctx context.Context, artist string) ([]model.InstallCandidate, error) {
	if artist == "" {
		return nil, nil
	}

	query := `
		SELECT
			s.folder,
			MIN(s.title) AS title,
			MIN(s.artist) AS artist,
			MIN(s.path) AS path
		FROM songdata.song s
		WHERE LOWER(s.artist) = LOWER(?)
		GROUP BY s.folder
	`

	rows, err := r.db.QueryContext(ctx, query, artist)
	if err != nil {
		return nil, fmt.Errorf("FindChartFoldersByArtist: %w", err)
	}
	defer rows.Close()

	var candidates []model.InstallCandidate
	for rows.Next() {
		var folder, t, a, path string
		if err := rows.Scan(&folder, &t, &a, &path); err != nil {
			return nil, fmt.Errorf("FindChartFoldersByArtist scan: %w", err)
		}
		candidates = append(candidates, model.InstallCandidate{
			FolderPath: path,
			Title:      t,
			Artist:     a,
			MatchTypes: []string{"artist"},
		})
	}
	return candidates, rows.Err()
}
```

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 4: コミット**

```bash
git add internal/domain/model/repository.go internal/adapter/persistence/songdata_reader.go
git commit -m "feat: SongRepositoryにFindChartFoldersByArtistを追加"
```

---

### Task 3: ユースケースにベースタイトル抽出とスコアリングを追加

**Files:**
- Modify: `internal/usecase/estimate_install_location.go`

**Step 1: ファイル全体を以下に置換**

```go
package usecase

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type EstimateInstallLocationUseCase struct {
	songRepo model.SongRepository
	metaRepo model.MetaRepository
}

func NewEstimateInstallLocationUseCase(songRepo model.SongRepository, metaRepo model.MetaRepository) *EstimateInstallLocationUseCase {
	return &EstimateInstallLocationUseCase{songRepo: songRepo, metaRepo: metaRepo}
}

// スコア定義
const (
	scoreTitle     = 3
	scoreBaseTitle = 2
	scoreBodyURL   = 3
	scoreArtist    = 1
)

// 末尾の [...] (...) -...- を繰り返し除去する正規表現
var trailingSuffixRe = regexp.MustCompile(`\s*(\[[^\]]*\]|\([^)]*\)|-[^-]+-)\s*$`)

// extractBaseTitle はエントリtitleから末尾の接尾辞を除去してベースタイトルを返す。
// 例: "影縫 [闇] (集0)" → "影縫"
func extractBaseTitle(title string) string {
	base := title
	for {
		trimmed := trailingSuffixRe.ReplaceAllString(base, "")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == base || trimmed == "" {
			break
		}
		base = trimmed
	}
	return base
}

// Execute は難易度表エントリのtitle, artist, md5をもとに、導入先候補をfolder単位で返す。
// 複数のマッチング手法で検索し、スコアリングで結果をランク付けする。
func (u *EstimateInstallLocationUseCase) Execute(ctx context.Context, title, artist, md5 string) ([]model.InstallCandidate, error) {
	merged := make(map[string]*model.InstallCandidate)

	addCandidates := func(candidates []model.InstallCandidate, matchType string, score int) {
		for _, c := range candidates {
			if existing, ok := merged[c.FolderPath]; ok {
				existing.MatchTypes = append(existing.MatchTypes, matchType)
				existing.Score += score
			} else {
				c.MatchTypes = []string{matchType}
				c.Score = score
				merged[c.FolderPath] = &c
			}
		}
	}

	// 1. タイトル完全一致検索（スコア3）
	titleCandidates, err := u.songRepo.FindChartFoldersByTitle(ctx, title)
	if err != nil {
		return nil, err
	}
	addCandidates(titleCandidates, "title", scoreTitle)

	// 2. ベースタイトル一致検索（スコア2）— 元titleと異なる場合のみ
	baseTitle := extractBaseTitle(title)
	if baseTitle != title && baseTitle != "" {
		baseCandidates, err := u.songRepo.FindChartFoldersByTitle(ctx, baseTitle)
		if err != nil {
			return nil, err
		}
		addCandidates(baseCandidates, "base_title", scoreBaseTitle)
	}

	// 3. body_url一致検索（スコア3）
	meta, err := u.metaRepo.GetChartMeta(ctx, md5)
	if err != nil {
		return nil, err
	}
	if meta != nil && meta.LR2IRBodyURL != "" {
		urlCandidates, err := u.songRepo.FindChartFoldersByBodyURL(ctx, meta.LR2IRBodyURL)
		if err != nil {
			return nil, err
		}
		addCandidates(urlCandidates, "body_url", scoreBodyURL)
	}

	// 4. アーティスト一致検索（スコア1）
	if artist != "" {
		artistCandidates, err := u.songRepo.FindChartFoldersByArtist(ctx, artist)
		if err != nil {
			return nil, err
		}
		addCandidates(artistCandidates, "artist", scoreArtist)
	}

	// 5. map→スライスに変換し、スコア降順でソート
	result := make([]model.InstallCandidate, 0, len(merged))
	for _, c := range merged {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result, nil
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go vet ./internal/usecase/...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/usecase/estimate_install_location.go
git commit -m "feat: ベースタイトル抽出・アーティスト一致・スコアリングを追加"
```

---

### Task 4: DTOにScoreフィールドを追加し、ハンドラーをartist対応に修正

**Files:**
- Modify: `internal/app/dto/dto.go`
- Modify: `internal/app/difficulty_table_handler.go`

**Step 1: InstallCandidateDTOにScoreフィールドを追加**

`dto.go`の`InstallCandidateDTO`を修正:

現在:
```go
type InstallCandidateDTO struct {
	FolderPath string   `json:"folderPath"`
	Title      string   `json:"title"`
	Artist     string   `json:"artist"`
	MatchTypes []string `json:"matchTypes"`
}
```

変更後:
```go
type InstallCandidateDTO struct {
	FolderPath string   `json:"folderPath"`
	Title      string   `json:"title"`
	Artist     string   `json:"artist"`
	MatchTypes []string `json:"matchTypes"`
	Score      int      `json:"score"`
}
```

**Step 2: EstimateInstallLocationメソッドを修正**

`difficulty_table_handler.go`の`EstimateInstallLocation`メソッドを修正。
エントリからartistも取得してユースケースに渡し、DTOにScoreを含める。

現在のExecute呼び出し:
```go
candidates, err := h.estimateUseCase.Execute(h.ctx, entry.Title, md5)
```

変更後:
```go
candidates, err := h.estimateUseCase.Execute(h.ctx, entry.Title, entry.Artist, md5)
```

DTO変換部分も修正:
```go
	result := make([]dto.InstallCandidateDTO, len(candidates))
	for i, c := range candidates {
		result[i] = dto.InstallCandidateDTO{
			FolderPath: c.FolderPath,
			Title:      c.Title,
			Artist:     c.Artist,
			MatchTypes: c.MatchTypes,
			Score:      c.Score,
		}
	}
```

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 4: コミット**

```bash
git add internal/app/dto/dto.go internal/app/difficulty_table_handler.go
git commit -m "feat: DTOにScore追加、ハンドラーをartist対応に修正"
```

---

### Task 5: Wailsバインディング再生成

**Step 1: バインディング再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: 成功（frontend/wailsjsは.gitignoreなのでコミット不要）

---

### Task 6: InstallCandidateCardのmatchLabel対応を追加

**Files:**
- Modify: `frontend/src/components/InstallCandidateCard.svelte`

**Step 1: Candidate型にscoreを追加**

現在:
```typescript
  type Candidate = {
    folderPath: string
    title: string
    artist: string
    matchTypes: string[]
  }
```

変更後:
```typescript
  type Candidate = {
    folderPath: string
    title: string
    artist: string
    matchTypes: string[]
    score: number
  }
```

**Step 2: matchLabel関数を修正**

現在:
```typescript
  function matchLabel(mt: string): string {
    return mt === 'title' ? 'タイトル一致' : 'URL一致'
  }
```

変更後:
```typescript
  function matchLabel(mt: string): string {
    switch (mt) {
      case 'title': return 'タイトル一致'
      case 'base_title': return 'タイトル類似'
      case 'body_url': return 'URL一致'
      case 'artist': return 'アーティスト一致'
      default: return mt
    }
  }
```

**Step 3: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

**Step 4: コミット**

```bash
git add frontend/src/components/InstallCandidateCard.svelte
git commit -m "feat: InstallCandidateCardに新しいmatchType表示を追加"
```

---

### Task 7: 統合ビルド確認

**Step 1: Wailsフルビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: 成功
