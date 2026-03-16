# 導入先推定機能 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 難易度表の未導入譜面について、タイトル一致・LR2IR本体URL一致で導入済み譜面を検索し、インストール先フォルダを推定・表示する。

**Architecture:** SongRepositoryに2つの検索メソッドを追加し、ユースケース層でマージ・重複排除する。DifficultyTableHandlerに新しいWailsバインディングメソッドを追加し、フロントエンドではInstallCandidateCardコンポーネントを新設してEntryDetailに組み込む。

**Tech Stack:** Go, SQLite, Svelte 4, TypeScript, Wails v2

---

### Task 1: ドメインモデルにInstallCandidateを追加

**Files:**
- Modify: `internal/domain/model/song.go`

**Step 1: song.goの末尾にInstallCandidate構造体を追加**

```go
// InstallCandidate は導入先推定の候補
type InstallCandidate struct {
	FolderPath string   // 楽曲フォルダのパス（songdata.songのpath/folderから導出）
	Title      string   // フォルダ内の代表タイトル
	Artist     string   // フォルダ内の代表アーティスト
	MatchTypes []string // マッチ理由: "title", "body_url"
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/domain/model/song.go
git commit -m "feat: InstallCandidate構造体を追加"
```

---

### Task 2: SongRepositoryインターフェースに検索メソッドを追加

**Files:**
- Modify: `internal/domain/model/repository.go`

**Step 1: SongRepositoryに2メソッドを追加**

`SongRepository` interfaceの末尾（`GetSongByFolder`の後）に追加:

```go
// タイトル完全一致（大文字小文字無視）で導入済み譜面をfolder単位で検索
FindChartFoldersByTitle(ctx context.Context, title string) ([]InstallCandidate, error)
// LR2IR本体URLが一致する導入済み譜面をfolder単位で検索
FindChartFoldersByBodyURL(ctx context.Context, bodyURL string) ([]InstallCandidate, error)
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: コンパイルエラー（SongdataReaderがインターフェースを満たさない）

**Step 3: songdata_reader.goにスタブ実装を追加**

`internal/adapter/persistence/songdata_reader.go` の末尾に追加:

```go
func (r *SongdataReader) FindChartFoldersByTitle(ctx context.Context, title string) ([]model.InstallCandidate, error) {
	return nil, nil
}

func (r *SongdataReader) FindChartFoldersByBodyURL(ctx context.Context, bodyURL string) ([]model.InstallCandidate, error) {
	return nil, nil
}
```

**Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 5: コミット**

```bash
git add internal/domain/model/repository.go internal/adapter/persistence/songdata_reader.go
git commit -m "feat: SongRepositoryにfolder検索メソッドを追加（スタブ）"
```

---

### Task 3: FindChartFoldersByTitleの実装

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`

**Step 1: FindChartFoldersByTitleのスタブを本実装に置換**

```go
func (r *SongdataReader) FindChartFoldersByTitle(ctx context.Context, title string) ([]model.InstallCandidate, error) {
	if title == "" {
		return nil, nil
	}

	query := `
		SELECT
			s.folder,
			MIN(s.title) AS title,
			MIN(s.artist) AS artist,
			MIN(s.path) AS path
		FROM songdata.song s
		WHERE LOWER(s.title) = LOWER(?)
		GROUP BY s.folder
	`

	rows, err := r.db.QueryContext(ctx, query, title)
	if err != nil {
		return nil, fmt.Errorf("FindChartFoldersByTitle: %w", err)
	}
	defer rows.Close()

	var candidates []model.InstallCandidate
	for rows.Next() {
		var folder, t, a, path string
		if err := rows.Scan(&folder, &t, &a, &path); err != nil {
			return nil, fmt.Errorf("FindChartFoldersByTitle scan: %w", err)
		}
		candidates = append(candidates, model.InstallCandidate{
			FolderPath: path,
			Title:      t,
			Artist:     a,
			MatchTypes: []string{"title"},
		})
	}
	return candidates, rows.Err()
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go vet ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go
git commit -m "feat: FindChartFoldersByTitleを実装（タイトル完全一致検索）"
```

---

### Task 4: FindChartFoldersByBodyURLの実装

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`

**Step 1: FindChartFoldersByBodyURLのスタブを本実装に置換**

```go
func (r *SongdataReader) FindChartFoldersByBodyURL(ctx context.Context, bodyURL string) ([]model.InstallCandidate, error) {
	if bodyURL == "" {
		return nil, nil
	}

	// chart_meta.lr2ir_body_urlが一致する譜面のmd5を取得し、songdata.songと突合
	query := `
		SELECT
			s.folder,
			MIN(s.title) AS title,
			MIN(s.artist) AS artist,
			MIN(s.path) AS path
		FROM main.chart_meta cm
		INNER JOIN songdata.song s ON s.md5 = cm.md5
		WHERE cm.lr2ir_body_url = ?
		GROUP BY s.folder
	`

	rows, err := r.db.QueryContext(ctx, query, bodyURL)
	if err != nil {
		return nil, fmt.Errorf("FindChartFoldersByBodyURL: %w", err)
	}
	defer rows.Close()

	var candidates []model.InstallCandidate
	for rows.Next() {
		var folder, t, a, path string
		if err := rows.Scan(&folder, &t, &a, &path); err != nil {
			return nil, fmt.Errorf("FindChartFoldersByBodyURL scan: %w", err)
		}
		candidates = append(candidates, model.InstallCandidate{
			FolderPath: path,
			Title:      t,
			Artist:     a,
			MatchTypes: []string{"body_url"},
		})
	}
	return candidates, rows.Err()
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go vet ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go
git commit -m "feat: FindChartFoldersByBodyURLを実装（IR本体URL一致検索）"
```

---

### Task 5: ユースケースの実装

**Files:**
- Create: `internal/usecase/estimate_install_location.go`

**Step 1: ユースケースファイルを作成**

```go
package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type EstimateInstallLocationUseCase struct {
	songRepo model.SongRepository
	metaRepo model.MetaRepository
}

func NewEstimateInstallLocationUseCase(songRepo model.SongRepository, metaRepo model.MetaRepository) *EstimateInstallLocationUseCase {
	return &EstimateInstallLocationUseCase{songRepo: songRepo, metaRepo: metaRepo}
}

// Execute は難易度表エントリのtitleとmd5をもとに、導入先候補をfolder単位で返す。
// タイトル一致とLR2IR本体URL一致の両方で検索し、結果をマージする。
func (u *EstimateInstallLocationUseCase) Execute(ctx context.Context, title string, md5 string) ([]model.InstallCandidate, error) {
	// 1. タイトル完全一致検索
	titleCandidates, err := u.songRepo.FindChartFoldersByTitle(ctx, title)
	if err != nil {
		return nil, err
	}

	// 2. md5からchart_metaのbody_urlを取得
	var urlCandidates []model.InstallCandidate
	meta, err := u.metaRepo.GetChartMeta(ctx, md5)
	if err != nil {
		return nil, err
	}
	if meta != nil && meta.LR2IRBodyURL != "" {
		urlCandidates, err = u.songRepo.FindChartFoldersByBodyURL(ctx, meta.LR2IRBodyURL)
		if err != nil {
			return nil, err
		}
	}

	// 3. folder単位でマージ（matchTypesを統合）
	merged := make(map[string]*model.InstallCandidate)
	for i := range titleCandidates {
		c := &titleCandidates[i]
		merged[c.FolderPath] = c
	}
	for _, c := range urlCandidates {
		if existing, ok := merged[c.FolderPath]; ok {
			existing.MatchTypes = append(existing.MatchTypes, "body_url")
		} else {
			merged[c.FolderPath] = &c
		}
	}

	// 4. map→スライスに変換
	result := make([]model.InstallCandidate, 0, len(merged))
	for _, c := range merged {
		result = append(result, *c)
	}

	return result, nil
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go vet ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/usecase/estimate_install_location.go
git commit -m "feat: EstimateInstallLocationUseCaseを追加"
```

---

### Task 6: DTOとハンドラーメソッドの追加

**Files:**
- Modify: `internal/app/dto/dto.go`
- Modify: `internal/app/difficulty_table_handler.go`
- Modify: `app.go`

**Step 1: dto.goにInstallCandidateDTOを追加**

`dto.go`の末尾（`InferWorkingURLResultDTO`の後あたり）に追加:

```go
type InstallCandidateDTO struct {
	FolderPath string   `json:"folderPath"`
	Title      string   `json:"title"`
	Artist     string   `json:"artist"`
	MatchTypes []string `json:"matchTypes"`
}
```

**Step 2: DifficultyTableHandlerにestimateUseCaseフィールドとメソッドを追加**

`difficulty_table_handler.go` を修正:

構造体にフィールド追加:
```go
type DifficultyTableHandler struct {
	ctx             context.Context
	dtRepo          *persistence.DifficultyTableRepository
	dtFetcher       *gateway.DifficultyTableFetcher
	songReader      *persistence.SongdataReader
	estimateUseCase *usecase.EstimateInstallLocationUseCase
}
```

コンストラクタの引数に追加:
```go
func NewDifficultyTableHandler(
	dtRepo *persistence.DifficultyTableRepository,
	dtFetcher *gateway.DifficultyTableFetcher,
	songReader *persistence.SongdataReader,
	estimateUseCase *usecase.EstimateInstallLocationUseCase,
) *DifficultyTableHandler {
	return &DifficultyTableHandler{
		dtRepo:          dtRepo,
		dtFetcher:       dtFetcher,
		songReader:      songReader,
		estimateUseCase: estimateUseCase,
	}
}
```

メソッドを追加（ファイル末尾）:
```go
func (h *DifficultyTableHandler) EstimateInstallLocation(md5 string, tableID int) ([]dto.InstallCandidateDTO, error) {
	// 難易度表エントリからtitleを取得
	entry, err := h.dtRepo.GetEntry(h.ctx, tableID, md5)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	candidates, err := h.estimateUseCase.Execute(h.ctx, entry.Title, md5)
	if err != nil {
		return nil, err
	}

	result := make([]dto.InstallCandidateDTO, len(candidates))
	for i, c := range candidates {
		result[i] = dto.InstallCandidateDTO{
			FolderPath: c.FolderPath,
			Title:      c.Title,
			Artist:     c.Artist,
			MatchTypes: c.MatchTypes,
		}
	}
	return result, nil
}
```

**Step 3: app.goのDI組み立てを修正**

`app.go`の`Init()`内、`a.DifficultyTableHandler = ...`の行を修正:

```go
estimateInstallLocation := usecase.NewEstimateInstallLocationUseCase(songdataReader, elsaRepo)
a.DifficultyTableHandler = internalapp.NewDifficultyTableHandler(dtRepo, dtFetcher, songdataReader, estimateInstallLocation)
```

importに`"github.com/meta-BE/bms-elsa/internal/usecase"`を追加（既に存在するので不要の可能性あり）。

**Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go vet ./...`
Expected: 成功

**Step 5: コミット**

```bash
git add internal/app/dto/dto.go internal/app/difficulty_table_handler.go app.go
git commit -m "feat: EstimateInstallLocationハンドラーを追加"
```

---

### Task 7: Wailsバインディング再生成

**Step 1: Wailsバインディングを再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`

Expected: `frontend/wailsjs/go/app/DifficultyTableHandler.js` に `EstimateInstallLocation` が追加される

**Step 2: 生成されたファイルを確認**

`frontend/wailsjs/go/app/DifficultyTableHandler.js` に `EstimateInstallLocation` 関数が存在することを確認。

**Step 3: コミット**

```bash
git add frontend/wailsjs/
git commit -m "chore: Wailsバインディングを再生成"
```

---

### Task 8: InstallCandidateCardコンポーネントの作成

**Files:**
- Create: `frontend/src/components/InstallCandidateCard.svelte`

**Step 1: コンポーネントを作成**

```svelte
<script lang="ts">
  import { EstimateInstallLocation } from '../../wailsjs/go/app/DifficultyTableHandler'
  import { OpenFolder } from '../../wailsjs/go/main/App'

  export let md5: string
  export let tableID: number

  type Candidate = {
    folderPath: string
    title: string
    artist: string
    matchTypes: string[]
  }

  let candidates: Candidate[] = []
  let loading = false

  $: if (md5 && tableID) load(md5, tableID)

  async function load(hash: string, tid: number) {
    loading = true
    candidates = []
    try {
      candidates = (await EstimateInstallLocation(hash, tid)) || []
    } catch (e) {
      console.error('Failed to estimate install location:', e)
    } finally {
      loading = false
    }
  }

  function matchLabel(mt: string): string {
    return mt === 'title' ? 'タイトル一致' : 'URL一致'
  }
</script>

<div class="bg-base-200 rounded-lg p-3">
  <h3 class="text-sm font-semibold mb-2">導入先の推定</h3>

  {#if loading}
    <div class="flex justify-center py-2">
      <span class="loading loading-spinner loading-sm"></span>
    </div>
  {:else if candidates.length === 0}
    <p class="text-sm text-base-content/50">一致する導入済み楽曲が見つかりませんでした</p>
  {:else}
    <div class="space-y-2">
      {#each candidates as c}
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <p class="text-sm truncate">{c.title} / {c.artist}</p>
            <p class="text-xs text-base-content/50 truncate">{c.folderPath}</p>
            <div class="flex gap-1 mt-0.5">
              {#each c.matchTypes as mt}
                <span class="badge badge-xs">{matchLabel(mt)}</span>
              {/each}
            </div>
          </div>
          <button
            class="btn btn-ghost btn-xs shrink-0"
            title="フォルダを開く"
            on:click={() => OpenFolder(c.folderPath)}
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
            </svg>
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>
```

**Step 2: コミット**

```bash
git add frontend/src/components/InstallCandidateCard.svelte
git commit -m "feat: InstallCandidateCardコンポーネントを作成"
```

---

### Task 9: EntryDetail.svelteにInstallCandidateCardを組み込み

**Files:**
- Modify: `frontend/src/views/EntryDetail.svelte`

**Step 1: importを追加**

`EntryDetail.svelte`のscriptセクション、`IRInfoCard`のimportの後に追加:

```typescript
import InstallCandidateCard from '../components/InstallCandidateCard.svelte'
```

**Step 2: テンプレートに組み込み**

`{#if chart}` ブロックの後、`<IRInfoCard .../>` の前に以下を追加:

```svelte
    <!-- 導入先推定（未導入の場合のみ） -->
    {#if !chart}
      <InstallCandidateCard {md5} {tableID} />
    {/if}
```

最終的なテンプレート順序:
1. エントリ基本情報カード
2. ChartInfoCard（導入済のみ）
3. **InstallCandidateCard（未導入のみ）** ← 新規追加
4. IRInfoCard

**Step 3: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

**Step 4: コミット**

```bash
git add frontend/src/views/EntryDetail.svelte
git commit -m "feat: EntryDetailに導入先推定カードを組み込み"
```

---

### Task 10: 統合ビルド確認

**Step 1: Wailsフルビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: 成功

**Step 2: 動作確認**

アプリを起動し、以下を確認:
- 難易度表タブで未導入譜面を選択
- EntryDetailに「導入先の推定」カードが表示される
- タイトル一致・URL一致でヒットした場合、候補が表示される
- 「フォルダを開く」ボタンが動作する
- 導入済み譜面を選択した場合はカードが表示されない
