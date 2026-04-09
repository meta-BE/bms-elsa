# URL書き換えフロントエンド適用化 実装プラン

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** バックグラウンドURL推定＋DB格納方式を廃止し、フロントエンドの表示時にリライトルールを適用する方式に変更する

**Architecture:** Svelteのwritableストアにリライトルールを保持し、IRInfoCard等のURL表示時にTS側のapplyRewriteRules関数で変換する。Go側のInferWorkingURLUseCase、UpdateChartMetaUseCase、DB列working_body_url/working_diff_urlは全て削除する。

**Tech Stack:** Svelte (writable store), TypeScript, Go, SQLite

---

### Task 1: フロントエンド — リライトルールストアとユーティリティ関数

**Files:**
- Create: `frontend/src/stores/rewriteRules.ts`
- Create: `frontend/src/lib/urlRewrite.ts`

- [ ] **Step 1: ストアファイルを作成**

```ts
// frontend/src/stores/rewriteRules.ts
import { writable } from 'svelte/store'

export type RewriteRule = {
  id: number
  ruleType: string
  pattern: string
  replacement: string
  priority: number
}

export const rewriteRules = writable<RewriteRule[]>([])
```

- [ ] **Step 2: URL書き換えユーティリティを作成**

```ts
// frontend/src/lib/urlRewrite.ts
import type { RewriteRule } from '../stores/rewriteRules'

/**
 * リライトルールをURLに適用する。
 * priority降順（ListRewriteRulesの返却順）で試行し、最初にマッチしたルールで置換。
 * マッチなしの場合は元URLをそのまま返す。
 */
export function applyRewriteRules(url: string, rules: RewriteRule[]): string {
  if (!url) return ''
  for (const rule of rules) {
    if (rule.ruleType === 'replace') {
      if (url.includes(rule.pattern)) {
        return url.replace(rule.pattern, rule.replacement)
      }
    } else if (rule.ruleType === 'regex') {
      try {
        const re = new RegExp(rule.pattern)
        if (re.test(url)) {
          return url.replace(re, rule.replacement)
        }
      } catch {
        // 不正な正規表現はスキップ
        continue
      }
    }
  }
  return url
}
```

- [ ] **Step 3: コミット**

```bash
git add frontend/src/stores/rewriteRules.ts frontend/src/lib/urlRewrite.ts
git commit -m "feat: リライトルールストアとURL書き換えユーティリティを追加"
```

---

### Task 2: フロントエンド — ストア初期化と RewriteRuleManager 連携

**Files:**
- Modify: `frontend/src/App.svelte:137-155` (onMount内にストア初期化追加)
- Modify: `frontend/src/settings/RewriteRuleManager.svelte:39-62` (ルール変更時にストア更新)

- [ ] **Step 1: App.svelte にストア初期化を追加**

`frontend/src/App.svelte` のimportに以下を追加:
```ts
import { rewriteRules } from './stores/rewriteRules'
import { ListRewriteRules } from '../wailsjs/go/app/RewriteHandler'
```

onMount内の先頭（既存のクリックハンドラの前）に以下を追加:
```ts
ListRewriteRules().then(rules => {
  rewriteRules.set(rules ?? [])
})
```

- [ ] **Step 2: RewriteRuleManager にストア更新を追加**

`frontend/src/settings/RewriteRuleManager.svelte` のimportに追加:
```ts
import { rewriteRules } from '../stores/rewriteRules'
```

`loadRules`関数（行23-27）を修正。現在:
```ts
async function loadRules() {
  try {
    rules = await ListRewriteRules() ?? []
  } catch (e: any) {
    error = e?.message || '読み込みに失敗しました'
  }
}
```

変更後:
```ts
async function loadRules() {
  try {
    rules = await ListRewriteRules() ?? []
    rewriteRules.set(rules)
  } catch (e: any) {
    error = e?.message || '読み込みに失敗しました'
  }
}
```

- [ ] **Step 3: コミット**

```bash
git add frontend/src/App.svelte frontend/src/settings/RewriteRuleManager.svelte
git commit -m "feat: 起動時・ルール編集時にrewriteRulesストアを更新"
```

---

### Task 3: フロントエンド — IRInfoCard をリライト適用方式に変更

**Files:**
- Modify: `frontend/src/components/IRInfoCard.svelte` (全面改修)

- [ ] **Step 1: IRInfoCard.svelte を改修**

`frontend/src/components/IRInfoCard.svelte` を以下のように書き換える:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { rewriteRules } from '../stores/rewriteRules'
  import { applyRewriteRules } from '../lib/urlRewrite'

  const dispatch = createEventDispatcher<{
    lookup: void
  }>()

  export let md5: string
  export let ir: {
    hasIrMeta: boolean
    lr2irTags?: string
    lr2irBodyUrl?: string
    lr2irDiffUrl?: string
    lr2irNotes?: string
  } | null = null

  function linkify(text: string): string {
    const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    return escaped.replace(
      /https?:\/\/(?:(?!https?:\/\/)[^\s<])+/g,
      url => {
        const rewritten = applyRewriteRules(url, $rewriteRules)
        return `<a href="${rewritten}" target="_blank" rel="noopener noreferrer" class="link link-primary">${rewritten}</a>`
      }
    )
  }
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between mb-2">
    <h3 class="text-sm font-semibold"><a href="http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5={md5}" target="_blank" rel="noopener noreferrer" class="link link-primary">LR2IR情報</a></h3>
    <button class="btn btn-ghost btn-xs" on:click={() => dispatch('lookup')}>IR取得</button>
  </div>
  {#if ir?.hasIrMeta}
    <div class="text-xs space-y-1">
      {#if ir.lr2irTags}
        <p><span class="font-semibold">タグ:</span> {ir.lr2irTags}</p>
      {/if}
      {#if ir.lr2irBodyUrl}
        <p>
          <span class="font-semibold">本体URL:</span>
          <a href={applyRewriteRules(ir.lr2irBodyUrl, $rewriteRules)} target="_blank" rel="noopener noreferrer" class="link link-primary">{applyRewriteRules(ir.lr2irBodyUrl, $rewriteRules)}</a>
        </p>
      {/if}
      {#if ir.lr2irDiffUrl}
        <p>
          <span class="font-semibold">差分URL:</span>
          <a href={applyRewriteRules(ir.lr2irDiffUrl, $rewriteRules)} target="_blank" rel="noopener noreferrer" class="link link-primary">{applyRewriteRules(ir.lr2irDiffUrl, $rewriteRules)}</a>
        </p>
      {/if}
      {#if ir.lr2irNotes}
        <p class="whitespace-pre-wrap"><span class="font-semibold">備考:</span> {@html linkify(ir.lr2irNotes)}</p>
      {/if}
    </div>
  {:else}
    <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
  {/if}
</div>
```

- [ ] **Step 2: コミット**

```bash
git add frontend/src/components/IRInfoCard.svelte
git commit -m "feat: IRInfoCardでリライトルールを表示時適用に変更"
```

---

### Task 4: フロントエンド — 親コンポーネントからsaveWorkingUrls関連を削除

**Files:**
- Modify: `frontend/src/views/ChartDetail.svelte:4,41-45,76`
- Modify: `frontend/src/views/EntryDetail.svelte:5,51-54,121`
- Modify: `frontend/src/views/SongDetail.svelte:5,93-97,289`

- [ ] **Step 1: ChartDetail.svelte を修正**

importから `UpdateChartMeta` を削除:
```ts
// 変更前
import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
// 変更後
import { LookupByMD5 } from '../../wailsjs/go/app/IRHandler'
```

`saveWorkingUrls`関数（行41-45）を削除:
```ts
// 以下を削除
async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
  if (!chart) return
  await UpdateChartMeta(chart.md5, e.detail.bodyUrl, e.detail.diffUrl)
  await loadChart(md5, folderHash)
}
```

IRInfoCardから`on:save`を削除:
```svelte
<!-- 変更前 -->
<IRInfoCard md5={chart.md5} ir={chart} on:lookup={lookupIR} on:save={saveWorkingUrls} />
<!-- 変更後 -->
<IRInfoCard md5={chart.md5} ir={chart} on:lookup={lookupIR} />
```

- [ ] **Step 2: EntryDetail.svelte を修正**

importから `UpdateChartMeta` を削除:
```ts
// 変更前
import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
// 変更後
import { LookupByMD5 } from '../../wailsjs/go/app/IRHandler'
```

`saveWorkingUrls`関数（行51-54）を削除:
```ts
// 以下を削除
async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
  await UpdateChartMeta(md5, e.detail.bodyUrl, e.detail.diffUrl)
  await loadEntry(md5, tableID)
}
```

IRInfoCardから`on:save`を削除:
```svelte
<!-- 変更前 -->
<IRInfoCard {md5} {ir} on:lookup={lookupIR} on:save={saveWorkingUrls} />
<!-- 変更後 -->
<IRInfoCard {md5} {ir} on:lookup={lookupIR} />
```

- [ ] **Step 3: SongDetail.svelte を修正**

importから `UpdateChartMeta` を削除:
```ts
// 変更前
import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
// 変更後
import { LookupByMD5 } from '../../wailsjs/go/app/IRHandler'
```

`saveWorkingUrls`関数（行93-97）を削除:
```ts
// 以下を削除
async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
  if (!selectedChart) return
  await UpdateChartMeta(selectedChart.md5, e.detail.bodyUrl, e.detail.diffUrl)
  if (detail) await loadDetail(detail.folderHash)
}
```

IRInfoCardから`on:save`を削除:
```svelte
<!-- 変更前 -->
<IRInfoCard md5={selectedChart.md5} ir={selectedChart} on:lookup={() => selectedChart && lookupIR(selectedChart)} on:save={saveWorkingUrls} />
<!-- 変更後 -->
<IRInfoCard md5={selectedChart.md5} ir={selectedChart} on:lookup={() => selectedChart && lookupIR(selectedChart)} />
```

- [ ] **Step 4: コミット**

```bash
git add frontend/src/views/ChartDetail.svelte frontend/src/views/EntryDetail.svelte frontend/src/views/SongDetail.svelte
git commit -m "refactor: 親コンポーネントからsaveWorkingUrls関連を削除"
```

---

### Task 5: フロントエンド — Settings.svelte から動作URL推定セクションを削除

**Files:**
- Modify: `frontend/src/settings/Settings.svelte:29-32,119-120,163-175,197-198,295-313`

- [ ] **Step 1: Settings.svelte から rewrite関連の状態・リスナー・UIを削除**

状態変数を削除（行29-32）:
```ts
// 以下を削除
let rewriteState: 'running' | 'done' | 'error' = 'done'
let rewriteProgress = { current: 0, total: 0 }
let rewriteError = ''
let rewriteResult = ''
```

イベントリスナー変数を削除（行119-120）:
```ts
// 以下を削除
let offRewriteProgress: (() => void) | null = null
let offRewriteDone: (() => void) | null = null
```

onMount内のイベントリスナー登録を削除（行163-175）:
```ts
// 以下を削除
offRewriteProgress = EventsOn('rewrite:progress', (data: { current: number; total: number }) => {
  rewriteState = 'running'
  rewriteProgress = data
})
offRewriteDone = EventsOn('rewrite:done', (data: { applied: number; skipped: number; total: number; error: string }) => {
  if (data.error) {
    rewriteState = 'error'
    rewriteError = data.error
  } else {
    rewriteState = 'done'
    rewriteResult = `${data.applied}件適用 / ${data.skipped}件スキップ`
  }
})
```

onDestroy内のクリーンアップを削除（行197-198）:
```ts
// 以下を削除
offRewriteProgress?.()
offRewriteDone?.()
```

UI「動作URL推定」セクションを削除（行295-313）:
```svelte
<!-- 以下を削除 -->
<div>
  <div class="flex items-center justify-between text-sm mb-1">
    <span>動作URL推定</span>
    ...
  </div>
  ...
</div>
```

`EventsOn`のインポートが他で使われていなければ削除（使われている場合はそのまま残す）。
`ProgressBar`のインポートが他で使われていなければ削除。

- [ ] **Step 2: コミット**

```bash
git add frontend/src/settings/Settings.svelte
git commit -m "refactor: Settings.svelteから動作URL推定セクションを削除"
```

---

### Task 6: Go側 — モデル・DTO・リポジトリインターフェースからWorkingURL関連を削除

**Files:**
- Modify: `internal/domain/model/song.go:64-75`
- Modify: `internal/domain/model/repository.go:79,96`
- Modify: `internal/app/dto/dto.go:61-62,74-75,168-169,247-248,217-221`
- Modify: `internal/adapter/persistence/elsa_repository.go:55-91,93-120,122-132,369-395`

- [ ] **Step 1: ChartIRMeta モデルからフィールド削除**

`internal/domain/model/song.go` の `ChartIRMeta` 構造体から `WorkingBodyURL` と `WorkingDiffURL` を削除:

```go
// 変更前
type ChartIRMeta struct {
	MD5            string
	SHA256         string
	Tags           []string
	LR2IRBodyURL   string
	LR2IRDiffURL   string
	LR2IRNotes     string
	WorkingBodyURL string
	WorkingDiffURL string
	FetchedAt      *time.Time
}

// 変更後
type ChartIRMeta struct {
	MD5          string
	SHA256       string
	Tags         []string
	LR2IRBodyURL string
	LR2IRDiffURL string
	LR2IRNotes   string
	FetchedAt    *time.Time
}
```

- [ ] **Step 2: MetaRepository インターフェースから不要メソッドを削除**

`internal/domain/model/repository.go` から以下を削除:
```go
// 以下を削除
UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
// 以下を削除
ListChartsForWorkingURLInference(ctx context.Context) ([]ChartIRMeta, error)
```

- [ ] **Step 3: DTO から WorkingURL フィールドと InferWorkingURLResultDTO を削除**

`internal/app/dto/dto.go`:

`ChartDTO` から削除:
```go
// 以下2行を削除
WorkingBodyURL string  `json:"workingBodyUrl,omitempty"`
WorkingDiffURL   string               `json:"workingDiffUrl,omitempty"`
```

`ChartIRMetaDTO` から削除:
```go
// 以下2行を削除
WorkingBodyURL string `json:"workingBodyUrl,omitempty"`
WorkingDiffURL string `json:"workingDiffUrl,omitempty"`
```

`ChartToDTO` 関数から削除:
```go
// 以下2行を削除
d.WorkingBodyURL = c.IRMeta.WorkingBodyURL
d.WorkingDiffURL = c.IRMeta.WorkingDiffURL
```

`ChartIRMetaToDTO` 関数から削除:
```go
// 以下2行を削除
d.WorkingBodyURL = m.WorkingBodyURL
d.WorkingDiffURL = m.WorkingDiffURL
```

`InferWorkingURLResultDTO` 構造体を削除:
```go
// 以下を削除
type InferWorkingURLResultDTO struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}
```

- [ ] **Step 4: elsa_repository.go から不要メソッドを削除し、クエリを修正**

`internal/adapter/persistence/elsa_repository.go`:

`UpdateWorkingURLs` メソッド（行122-132）を削除。

`ListChartsForWorkingURLInference` メソッド（行369-395）を削除。

`GetChartMeta`（行55-91）のSQLとscanからworking_body_url/working_diff_urlを削除:

```go
// 変更後
func (r *ElsaRepository) GetChartMeta(ctx context.Context, md5 string) (*model.ChartIRMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, sha256, lr2ir_tags,
		        COALESCE(lr2ir_body_url, ''), COALESCE(lr2ir_diff_url, ''), COALESCE(lr2ir_notes, ''),
		        lr2ir_fetched_at
		 FROM chart_meta WHERE md5 = ?`,
		md5,
	)

	var m model.ChartIRMeta
	var tagsStr sql.NullString
	var fetchedAtStr sql.NullString

	if err := row.Scan(
		&m.MD5, &m.SHA256, &tagsStr,
		&m.LR2IRBodyURL, &m.LR2IRDiffURL, &m.LR2IRNotes,
		&fetchedAtStr,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if tagsStr.Valid && tagsStr.String != "" {
		m.Tags = strings.Split(tagsStr.String, ",")
	}
	if fetchedAtStr.Valid && fetchedAtStr.String != "" {
		t, err := time.ParseInLocation(timeLayout, fetchedAtStr.String, time.UTC)
		if err != nil {
			return nil, err
		}
		m.FetchedAt = &t
	}

	return &m, nil
}
```

`UpsertChartMeta`（行93-120）からworking_body_url/working_diff_urlを削除:

```go
// 変更後
func (r *ElsaRepository) UpsertChartMeta(ctx context.Context, meta model.ChartIRMeta) error {
	tagsStr := strings.Join(meta.Tags, ",")

	var fetchedAtStr *string
	if meta.FetchedAt != nil {
		s := meta.FetchedAt.UTC().Format(timeLayout)
		fetchedAtStr = &s
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chart_meta (md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes, lr2ir_fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(md5) DO UPDATE SET
		   sha256           = COALESCE(NULLIF(excluded.sha256, ''), chart_meta.sha256),
		   lr2ir_tags       = excluded.lr2ir_tags,
		   lr2ir_body_url   = excluded.lr2ir_body_url,
		   lr2ir_diff_url   = excluded.lr2ir_diff_url,
		   lr2ir_notes      = excluded.lr2ir_notes,
		   lr2ir_fetched_at = excluded.lr2ir_fetched_at,
		   updated_at       = datetime('now')`,
		meta.MD5, meta.SHA256, tagsStr,
		meta.LR2IRBodyURL, meta.LR2IRDiffURL, meta.LR2IRNotes,
		fetchedAtStr,
	)
	return err
}
```

- [ ] **Step 5: ビルド確認**

```bash
go build ./...
```

コンパイルエラーが出た場合、参照箇所を修正する。

- [ ] **Step 6: コミット**

```bash
git add internal/domain/model/song.go internal/domain/model/repository.go internal/app/dto/dto.go internal/adapter/persistence/elsa_repository.go
git commit -m "refactor: モデル・DTO・リポジトリからWorkingURL関連を削除"
```

---

### Task 7: Go側 — ユースケース・ハンドラーの削除と修正

**Files:**
- Delete: `internal/usecase/infer_working_url.go`
- Delete: `internal/usecase/infer_working_url_test.go`
- Delete: `internal/usecase/update_chart_meta.go`
- Modify: `internal/app/rewrite_handler.go:13-20,22-24,62-117`
- Modify: `internal/app/ir_handler.go:14-24,26-33,57-59`
- Modify: `app.go:108-109,142`

- [ ] **Step 1: 不要なユースケースファイルを削除**

```bash
rm internal/usecase/infer_working_url.go
rm internal/usecase/infer_working_url_test.go
rm internal/usecase/update_chart_meta.go
```

- [ ] **Step 2: rewrite_handler.go からバックグラウンド推定関連を削除**

`internal/app/rewrite_handler.go` の構造体から不要フィールドを削除:

```go
// 変更後
type RewriteHandler struct {
	ctx      context.Context
	metaRepo model.MetaRepository
}

func NewRewriteHandler(metaRepo model.MetaRepository) *RewriteHandler {
	return &RewriteHandler{metaRepo: metaRepo}
}
```

以下のメソッドを削除:
- `InferWorkingURLs()` (行62-72)
- `StartInferWorkingURLs()` (行75-110)
- `IsInferring()` (行113-117)

不要なimportを整理（`sync`, `usecase`, `wailsRuntime`が他で使われていなければ削除）。

- [ ] **Step 3: ir_handler.go から UpdateChartMeta 関連を削除**

`internal/app/ir_handler.go` の構造体から `updateChart` フィールドを削除:

```go
// 変更後
type IRHandler struct {
	ctx         context.Context
	lookupIR    *usecase.LookupIRUseCase
	bulkFetchIR *usecase.BulkFetchIRUseCase
	metaRepo    model.MetaRepository

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}
```

コンストラクタを修正:
```go
// 変更後
func NewIRHandler(
	li *usecase.LookupIRUseCase,
	bf *usecase.BulkFetchIRUseCase,
	mr model.MetaRepository,
) *IRHandler {
	return &IRHandler{lookupIR: li, bulkFetchIR: bf, metaRepo: mr}
}
```

`UpdateChartMeta` メソッド（行57-59）を削除。

不要なimport（`update_chart_meta`関連）を整理。

- [ ] **Step 4: app.go のDI・起動処理を修正**

`app.go`:

`inferWorkingURLs`と`updateChartMeta`のDIを削除。`RewriteHandler`と`IRHandler`のコンストラクタ呼び出しを修正:

```go
// 変更前
inferWorkingURLs := usecase.NewInferWorkingURLUseCase(elsaRepo)
a.RewriteHandler = internalapp.NewRewriteHandler(inferWorkingURLs, elsaRepo)

// 変更後
a.RewriteHandler = internalapp.NewRewriteHandler(elsaRepo)
```

```go
// 変更前（IRHandler関連、元のDI部分を確認して修正）
updateChartMeta := usecase.NewUpdateChartMetaUseCase(elsaRepo)
// ...
a.IRHandler = internalapp.NewIRHandler(lookupIR, bulkFetchIR, updateChartMeta, elsaRepo)

// 変更後
a.IRHandler = internalapp.NewIRHandler(lookupIR, bulkFetchIR, elsaRepo)
```

startup関数から`StartInferWorkingURLs`呼び出しを削除:
```go
// 以下を削除
a.RewriteHandler.StartInferWorkingURLs()
```

- [ ] **Step 5: ビルド確認**

```bash
go build ./...
```

- [ ] **Step 6: コミット**

```bash
git add -A
git commit -m "refactor: InferWorkingURL・UpdateChartMetaユースケースとバックグラウンド推定を削除"
```

---

### Task 8: Go側 — テストの修正

**Files:**
- Modify: `internal/usecase/usecase_test.go:52-83,240-266`
- Modify: `internal/adapter/persistence/elsa_repository_test.go:228-260`

- [ ] **Step 1: usecase_test.go のモックとテストを修正**

`internal/usecase/usecase_test.go`:

`mockMetaRepo` 構造体から `updateWorkingURLsFunc` フィールドを削除:
```go
// 以下を削除
updateWorkingURLsFunc   func(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
```

`UpdateWorkingURLs` メソッド実装を削除:
```go
// 以下を削除
func (m *mockMetaRepo) UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error {
	return m.updateWorkingURLsFunc(ctx, md5, workingBodyURL, workingDiffURL)
}
```

`ListChartsForWorkingURLInference` のスタブ実装を削除（存在する場合）。

`TestUpdateChartMeta` テスト全体を削除（行240-266）。

- [ ] **Step 2: elsa_repository_test.go から TestUpdateWorkingURLs を削除**

`internal/adapter/persistence/elsa_repository_test.go` から `TestUpdateWorkingURLs` テスト全体を削除。

- [ ] **Step 3: テスト実行**

```bash
go test ./...
```

- [ ] **Step 4: コミット**

```bash
git add internal/usecase/usecase_test.go internal/adapter/persistence/elsa_repository_test.go
git commit -m "test: WorkingURL関連のテストを削除"
```

---

### Task 9: DBマイグレーション — working_body_url / working_diff_url カラムの削除

**Files:**
- Modify: `internal/adapter/persistence/migrations.go:41-54（テーブル定義）, 末尾（マイグレーション追加）`

- [ ] **Step 1: CREATE TABLE文からカラム定義を削除**

`internal/adapter/persistence/migrations.go` のchart_metaのCREATE TABLE文を修正:

```sql
-- 変更前
CREATE TABLE IF NOT EXISTS chart_meta (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    md5              TEXT NOT NULL UNIQUE,
    sha256           TEXT NOT NULL DEFAULT '',
    lr2ir_tags       TEXT,
    lr2ir_body_url   TEXT,
    lr2ir_diff_url   TEXT,
    lr2ir_notes      TEXT,
    lr2ir_fetched_at TEXT,
    working_body_url TEXT,
    working_diff_url TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
)

-- 変更後
CREATE TABLE IF NOT EXISTS chart_meta (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    md5              TEXT NOT NULL UNIQUE,
    sha256           TEXT NOT NULL DEFAULT '',
    lr2ir_tags       TEXT,
    lr2ir_body_url   TEXT,
    lr2ir_diff_url   TEXT,
    lr2ir_notes      TEXT,
    lr2ir_fetched_at TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
)
```

- [ ] **Step 2: RunMigrations末尾にDROP COLUMNマイグレーションを追加**

`RunMigrations`関数の `return nil` の直前に追加:

```go
// working_body_url / working_diff_url カラムの削除（冪等）
var hasWorkingBodyURL int
_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chart_meta') WHERE name='working_body_url'`).Scan(&hasWorkingBodyURL)
if hasWorkingBodyURL > 0 {
    if _, err := db.Exec(`ALTER TABLE chart_meta DROP COLUMN working_body_url`); err != nil {
        return fmt.Errorf("drop working_body_url: %w", err)
    }
}
var hasWorkingDiffURL int
_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chart_meta') WHERE name='working_diff_url'`).Scan(&hasWorkingDiffURL)
if hasWorkingDiffURL > 0 {
    if _, err := db.Exec(`ALTER TABLE chart_meta DROP COLUMN working_diff_url`); err != nil {
        return fmt.Errorf("drop working_diff_url: %w", err)
    }
}
```

- [ ] **Step 3: ビルド・テスト確認**

```bash
go build ./... && go test ./...
```

- [ ] **Step 4: コミット**

```bash
git add internal/adapter/persistence/migrations.go
git commit -m "migration: chart_metaからworking_body_url/working_diff_urlカラムを削除"
```

---

### Task 10: Wails バインディング再生成とフロントエンドビルド確認

**Files:**
- Regenerated: `frontend/wailsjs/go/models.ts` (自動生成)
- Regenerated: `frontend/wailsjs/go/app/IRHandler.{js,d.ts}` (自動生成)
- Regenerated: `frontend/wailsjs/go/app/RewriteHandler.{js,d.ts}` (自動生成)

- [ ] **Step 1: Wails バインディングを再生成**

```bash
wails generate module
```

- [ ] **Step 2: 生成結果を確認**

`models.ts`から`workingBodyUrl`/`workingDiffUrl`/`InferWorkingURLResultDTO`が消えていること、`IRHandler.d.ts`から`UpdateChartMeta`が消えていること、`RewriteHandler.d.ts`から`InferWorkingURLs`/`StartInferWorkingURLs`/`IsInferring`が消えていることを確認。

- [ ] **Step 3: フロントエンドビルド確認**

```bash
cd frontend && npm run build
```

TypeScriptエラーが出た場合、フロントエンドで削除済みの型やメソッドを参照している箇所を修正。

- [ ] **Step 4: コミット**

```bash
cd .. && git add frontend/wailsjs/
git commit -m "chore: Wailsバインディングを再生成"
```

---

### Task 11: マニュアル更新

**Files:**
- Modify: `docs/manual.md`

- [ ] **Step 1: マニュアルの該当セクションを確認**

`docs/manual.md`内で「動作URL」「URL推定」「URL書き換え」に関する記述を検索し、現在の動作に合わせて更新する。

- URL書き換えルールの説明は残す（設定画面でのルール管理は継続）
- 「起動時に自動でURL推定を行う」等の記述を削除
- 「IRInfoCardに表示されるURLは書き換えルールに従って自動変換される」旨に更新

- [ ] **Step 2: コミット**

```bash
git add docs/manual.md
git commit -m "docs: マニュアルのURL書き換え説明を更新"
```
