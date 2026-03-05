# URL書き換えルール 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** LR2IR URLに書き換えルールを適用して動作URLを自動推定する機能を追加し、動作URL表示をリンク+編集ボタンに改善する

**Architecture:** event_mapping と同様のレイヤー構成（Handler → UseCase → Repository）で url_rewrite_rule テーブルを管理。ルール適用ロジックは UseCase 層に実装し、replace/regex の2タイプをサポート。フロントエンドは既存の InferenceModal/EventMappingManager パターンを踏襲。

**Tech Stack:** Go (Wails v2), SQLite, Svelte (TypeScript), DaisyUI

---

### Task 1: RewriteRule ドメインモデル

**Files:**
- Modify: `internal/domain/model/song.go` (末尾に追加)

**Step 1: RewriteRule 構造体を追加**

`internal/domain/model/song.go` の末尾に以下を追加:

```go
// RewriteRule はURL書き換えルール
type RewriteRule struct {
	ID          int
	RuleType    string // "replace" or "regex"
	Pattern     string
	Replacement string
	Priority    int
}
```

**Step 2: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: SUCCESS

**Step 3: コミット**

```bash
git add internal/domain/model/song.go
git commit -m "feat: RewriteRule ドメインモデルを追加"
```

---

### Task 2: MetaRepository インターフェース拡張

**Files:**
- Modify: `internal/domain/model/repository.go` (MetaRepository に3メソッド追加)

**Step 1: MetaRepository にURL書き換えルール関連メソッドを追加**

`internal/domain/model/repository.go` の MetaRepository インターフェースに以下を追加:

```go
// URL書き換えルール
ListRewriteRules(ctx context.Context) ([]RewriteRule, error)
UpsertRewriteRule(ctx context.Context, rule RewriteRule) error
DeleteRewriteRule(ctx context.Context, id int) error
// 動作URL未設定の譜面（lr2ir URLあり）を取得
ListChartsForWorkingURLInference(ctx context.Context) ([]ChartIRMeta, error)
```

**Step 2: コンパイル確認（失敗を確認）**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: FAIL（ElsaRepository が新メソッドを未実装のため）

---

### Task 3: DB マイグレーション + リポジトリ CRUD 実装

**Files:**
- Modify: `internal/adapter/persistence/migrations.go` (statements 配列にテーブル追加)
- Modify: `internal/adapter/persistence/elsa_repository.go` (4メソッド追加)

**Step 1: migrations.go にテーブル定義を追加**

`internal/adapter/persistence/migrations.go` の `statements` スライス末尾（`event_mapping` の後）に追加:

```go
`CREATE TABLE IF NOT EXISTS url_rewrite_rule (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	rule_type   TEXT NOT NULL CHECK(rule_type IN ('replace', 'regex')),
	pattern     TEXT NOT NULL,
	replacement TEXT NOT NULL,
	priority    INTEGER NOT NULL DEFAULT 0,
	created_at  TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
	UNIQUE(rule_type, pattern)
)`,
```

**Step 2: elsa_repository.go に ListRewriteRules を追加**

```go
func (r *ElsaRepository) ListRewriteRules(ctx context.Context) ([]model.RewriteRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, rule_type, pattern, replacement, priority FROM url_rewrite_rule ORDER BY priority DESC, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.RewriteRule
	for rows.Next() {
		var rule model.RewriteRule
		if err := rows.Scan(&rule.ID, &rule.RuleType, &rule.Pattern, &rule.Replacement, &rule.Priority); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}
```

**Step 3: elsa_repository.go に UpsertRewriteRule を追加**

```go
func (r *ElsaRepository) UpsertRewriteRule(ctx context.Context, rule model.RewriteRule) error {
	if rule.ID > 0 {
		_, err := r.db.ExecContext(ctx,
			`UPDATE url_rewrite_rule SET rule_type = ?, pattern = ?, replacement = ?, priority = ?, updated_at = datetime('now') WHERE id = ?`,
			rule.RuleType, rule.Pattern, rule.Replacement, rule.Priority, rule.ID,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO url_rewrite_rule (rule_type, pattern, replacement, priority) VALUES (?, ?, ?, ?)
		 ON CONFLICT(rule_type, pattern) DO UPDATE SET replacement = excluded.replacement, priority = excluded.priority, updated_at = datetime('now')`,
		rule.RuleType, rule.Pattern, rule.Replacement, rule.Priority,
	)
	return err
}
```

**Step 4: elsa_repository.go に DeleteRewriteRule を追加**

```go
func (r *ElsaRepository) DeleteRewriteRule(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM url_rewrite_rule WHERE id = ?`, id)
	return err
}
```

**Step 5: elsa_repository.go に ListChartsForWorkingURLInference を追加**

動作URL未設定（working_body_url と working_diff_url が両方空）で、lr2ir URLがある譜面を取得:

```go
func (r *ElsaRepository) ListChartsForWorkingURLInference(ctx context.Context) ([]model.ChartIRMeta, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT md5, sha256, lr2ir_body_url, lr2ir_diff_url
		 FROM chart_meta
		 WHERE (working_body_url IS NULL OR working_body_url = '')
		   AND (working_diff_url IS NULL OR working_diff_url = '')
		   AND (lr2ir_body_url IS NOT NULL AND lr2ir_body_url != ''
		        OR lr2ir_diff_url IS NOT NULL AND lr2ir_diff_url != '')`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var charts []model.ChartIRMeta
	for rows.Next() {
		var c model.ChartIRMeta
		var bodyURL, diffURL sql.NullString
		if err := rows.Scan(&c.MD5, &c.SHA256, &bodyURL, &diffURL); err != nil {
			return nil, err
		}
		c.LR2IRBodyURL = bodyURL.String
		c.LR2IRDiffURL = diffURL.String
		charts = append(charts, c)
	}
	return charts, rows.Err()
}
```

**Step 6: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: SUCCESS

**Step 7: コミット**

```bash
git add internal/adapter/persistence/migrations.go internal/adapter/persistence/elsa_repository.go
git commit -m "feat: url_rewrite_rule テーブルとリポジトリ CRUD を実装"
```

---

### Task 4: ルール適用ロジック（TDD）

**Files:**
- Create: `internal/usecase/infer_working_url_test.go`
- Create: `internal/usecase/infer_working_url.go`

**Step 1: テストファイルを作成**

```go
package usecase

import (
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

func TestApplyRewriteRules(t *testing.T) {
	rules := []model.RewriteRule{
		{ID: 1, RuleType: "replace", Pattern: "old-host.com/bms", Replacement: "new-host.com/bms", Priority: 10},
		{ID: 2, RuleType: "regex", Pattern: `example\.com/dl/(\d+)`, Replacement: "mirror.com/download?id=$1", Priority: 5},
	}

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"replaceマッチ", "http://old-host.com/bms/song123", "http://new-host.com/bms/song123"},
		{"regexマッチ", "http://example.com/dl/456", "http://mirror.com/download?id=456"},
		{"マッチなし", "http://other-host.com/file", ""},
		{"空URL", "", ""},
		{"priority順（高い方が優先）", "http://old-host.com/bms/example.com/dl/789", "http://new-host.com/bms/example.com/dl/789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyRewriteRules(tt.url, rules)
			if result != tt.expected {
				t.Errorf("applyRewriteRules(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestApplyRewriteRules_EmptyRules(t *testing.T) {
	result := applyRewriteRules("http://example.com", nil)
	if result != "" {
		t.Errorf("空ルールなのに結果が返った: %q", result)
	}
}
```

**Step 2: テスト実行（失敗確認）**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/usecase/ -run TestApplyRewriteRules -v`
Expected: FAIL（applyRewriteRules 未定義）

**Step 3: ルール適用ロジックを実装**

`internal/usecase/infer_working_url.go`:

```go
package usecase

import (
	"context"
	"regexp"
	"strings"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// InferWorkingURLResult は動作URL推定の結果
type InferWorkingURLResult struct {
	Applied int
	Skipped int
	Total   int
}

// InferWorkingURLUseCase は書き換えルールによる動作URL自動推定
type InferWorkingURLUseCase struct {
	metaRepo model.MetaRepository
}

func NewInferWorkingURLUseCase(metaRepo model.MetaRepository) *InferWorkingURLUseCase {
	return &InferWorkingURLUseCase{metaRepo: metaRepo}
}

func (u *InferWorkingURLUseCase) Execute(ctx context.Context) (*InferWorkingURLResult, error) {
	rules, err := u.metaRepo.ListRewriteRules(ctx)
	if err != nil {
		return nil, err
	}

	charts, err := u.metaRepo.ListChartsForWorkingURLInference(ctx)
	if err != nil {
		return nil, err
	}

	result := &InferWorkingURLResult{Total: len(charts)}

	for _, c := range charts {
		bodyURL := applyRewriteRules(c.LR2IRBodyURL, rules)
		diffURL := applyRewriteRules(c.LR2IRDiffURL, rules)

		if bodyURL == "" && diffURL == "" {
			result.Skipped++
			continue
		}

		if err := u.metaRepo.UpdateWorkingURLs(ctx, c.MD5, bodyURL, diffURL); err != nil {
			return nil, err
		}
		result.Applied++
	}

	return result, nil
}

// applyRewriteRules はルールリスト（priority降順を前提）を順に適用し、
// 最初にマッチしたルールの結果を返す。マッチなしなら空文字を返す。
func applyRewriteRules(url string, rules []model.RewriteRule) string {
	if url == "" {
		return ""
	}
	for _, rule := range rules {
		switch rule.RuleType {
		case "replace":
			if strings.Contains(url, rule.Pattern) {
				return strings.Replace(url, rule.Pattern, rule.Replacement, 1)
			}
		case "regex":
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				continue
			}
			if re.MatchString(url) {
				return re.ReplaceAllString(url, rule.Replacement)
			}
		}
	}
	return ""
}
```

**Step 4: テスト実行（成功確認）**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/usecase/ -run TestApplyRewriteRules -v`
Expected: PASS

**Step 5: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: SUCCESS

**Step 6: コミット**

```bash
git add internal/usecase/infer_working_url.go internal/usecase/infer_working_url_test.go
git commit -m "feat: URL書き換えルール適用ロジックを TDD で実装"
```

---

### Task 5: DTO + RewriteHandler

**Files:**
- Modify: `internal/app/dto/dto.go` (DTO追加)
- Create: `internal/app/rewrite_handler.go`

**Step 1: dto.go に RewriteRuleDTO と InferWorkingURLResultDTO を追加**

`internal/app/dto/dto.go` の末尾に追加:

```go
type RewriteRuleDTO struct {
	ID          int    `json:"id"`
	RuleType    string `json:"ruleType"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Priority    int    `json:"priority"`
}

type InferWorkingURLResultDTO struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}
```

**Step 2: rewrite_handler.go を作成**

`internal/app/rewrite_handler.go`:

```go
package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type RewriteHandler struct {
	ctx              context.Context
	inferWorkingURLs *usecase.InferWorkingURLUseCase
	metaRepo         model.MetaRepository
}

func NewRewriteHandler(inferWorkingURLs *usecase.InferWorkingURLUseCase, metaRepo model.MetaRepository) *RewriteHandler {
	return &RewriteHandler{inferWorkingURLs: inferWorkingURLs, metaRepo: metaRepo}
}

func (h *RewriteHandler) SetContext(ctx context.Context) {
	h.ctx = ctx
}

func (h *RewriteHandler) ListRewriteRules() ([]dto.RewriteRuleDTO, error) {
	rules, err := h.metaRepo.ListRewriteRules(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.RewriteRuleDTO, len(rules))
	for i, r := range rules {
		result[i] = dto.RewriteRuleDTO{
			ID:          r.ID,
			RuleType:    r.RuleType,
			Pattern:     r.Pattern,
			Replacement: r.Replacement,
			Priority:    r.Priority,
		}
	}
	return result, nil
}

func (h *RewriteHandler) UpsertRewriteRule(id int, ruleType, pattern, replacement string, priority int) error {
	return h.metaRepo.UpsertRewriteRule(h.ctx, model.RewriteRule{
		ID:          id,
		RuleType:    ruleType,
		Pattern:     pattern,
		Replacement: replacement,
		Priority:    priority,
	})
}

func (h *RewriteHandler) DeleteRewriteRule(id int) error {
	return h.metaRepo.DeleteRewriteRule(h.ctx, id)
}

func (h *RewriteHandler) InferWorkingURLs() (*dto.InferWorkingURLResultDTO, error) {
	result, err := h.inferWorkingURLs.Execute(h.ctx)
	if err != nil {
		return nil, err
	}
	return &dto.InferWorkingURLResultDTO{
		Applied: result.Applied,
		Skipped: result.Skipped,
		Total:   result.Total,
	}, nil
}
```

**Step 3: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: SUCCESS

**Step 4: コミット**

```bash
git add internal/app/dto/dto.go internal/app/rewrite_handler.go
git commit -m "feat: RewriteHandler と DTO を追加"
```

---

### Task 6: app.go に RewriteHandler を登録

**Files:**
- Modify: `app.go` (App 構造体 + Init + startup)
- Modify: `main.go` (Bind に追加)

**Step 1: app.go の App 構造体に RewriteHandler フィールドを追加**

`app.go:22-32` の App 構造体に追加:

```go
RewriteHandler   *internalapp.RewriteHandler
```

**Step 2: app.go の Init() に RewriteHandler の初期化を追加**

`app.go:82-83` の inferMeta 初期化の直後に追加:

```go
	inferWorkingURLs := usecase.NewInferWorkingURLUseCase(elsaRepo)
	a.RewriteHandler = internalapp.NewRewriteHandler(inferWorkingURLs, elsaRepo)
```

**Step 3: app.go の startup() に SetContext を追加**

`app.go:92` の `a.InferenceHandler.SetContext(ctx)` の直後に追加:

```go
	a.RewriteHandler.SetContext(ctx)
```

**Step 4: main.go の Bind に RewriteHandler を追加**

`main.go:31-36` の Bind 配列に追加:

```go
app.RewriteHandler,
```

**Step 5: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: SUCCESS

**Step 6: コミット**

```bash
git add app.go main.go
git commit -m "feat: RewriteHandler を Wails に登録"
```

---

### Task 7: Wails バインディング再生成

**Step 1: Wails バインディングを再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`

**Step 2: 生成されたファイルを確認**

Run: `ls frontend/wailsjs/go/app/RewriteHandler.*`
Expected: `RewriteHandler.js` と `RewriteHandler.d.ts` が生成されている

**Step 3: コミット**

```bash
git add frontend/wailsjs/
git commit -m "chore: RewriteHandler の Wails バインディングを生成"
```

---

### Task 8: 動作URL表示モード切り替え（SongDetail.svelte）

**Files:**
- Modify: `frontend/src/SongDetail.svelte`

**Step 1: 編集モードの状態変数を追加**

`SongDetail.svelte:18-19` の `editWorkingBodyUrl`/`editWorkingDiffUrl` 宣言の直後に追加:

```typescript
let editingWorkingUrl = false
```

**Step 2: 動作URL表示部分をリンク+編集ボタンに変更**

`SongDetail.svelte:176-184` の動作URL入力欄を以下に置き換え:

```svelte
{#if editingWorkingUrl}
  <div class="flex gap-2 items-center">
    <label class="font-semibold" for="working-body-url">動作URL(本体):</label>
    <input id="working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={() => { saveWorkingUrls(); editingWorkingUrl = false }} />
  </div>
  <div class="flex gap-2 items-center">
    <label class="font-semibold" for="working-diff-url">動作URL(差分):</label>
    <input id="working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={() => { saveWorkingUrls(); editingWorkingUrl = false }} />
  </div>
{:else}
  <div class="flex gap-2 items-center">
    <span class="font-semibold">動作URL(本体):</span>
    {#if editWorkingBodyUrl}
      <a href={editWorkingBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary text-xs truncate flex-1">{editWorkingBodyUrl}</a>
    {:else}
      <span class="text-base-content/30 text-xs">未設定</span>
    {/if}
  </div>
  <div class="flex gap-2 items-center justify-between">
    <div class="flex gap-2 items-center flex-1 min-w-0">
      <span class="font-semibold">動作URL(差分):</span>
      {#if editWorkingDiffUrl}
        <a href={editWorkingDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary text-xs truncate flex-1">{editWorkingDiffUrl}</a>
      {:else}
        <span class="text-base-content/30 text-xs">未設定</span>
      {/if}
    </div>
    <button class="btn btn-ghost btn-xs" on:click|stopPropagation={() => editingWorkingUrl = true}>
      編集
    </button>
  </div>
{/if}
```

**Step 3: コミット**

```bash
git add frontend/src/SongDetail.svelte
git commit -m "feat: SongDetail の動作URLをリンク表示+編集ボタンに変更"
```

---

### Task 9: 動作URL表示モード切り替え（ChartDetail.svelte）

**Files:**
- Modify: `frontend/src/ChartDetail.svelte`

**Step 1: 編集モードの状態変数を追加**

`ChartDetail.svelte:14-15` の宣言部分に追加:

```typescript
let editingWorkingUrl = false
```

**Step 2: 動作URL表示部分を置き換え**

`ChartDetail.svelte:125-133` の動作URL入力欄を Task 8 と同じパターンに置き換え（idは `chart-working-body-url`/`chart-working-diff-url` を使用）。blurハンドラーは `saveWorkingUrls(); editingWorkingUrl = false` にする。

**Step 3: コミット**

```bash
git add frontend/src/ChartDetail.svelte
git commit -m "feat: ChartDetail の動作URLをリンク表示+編集ボタンに変更"
```

---

### Task 10: 動作URL表示モード切り替え（EntryDetail.svelte）

**Files:**
- Modify: `frontend/src/EntryDetail.svelte`

**Step 1: 編集モードの状態変数を追加**

`EntryDetail.svelte:18` 付近に追加:

```typescript
let editingWorkingUrl = false
```

**Step 2: 動作URL表示部分を置き換え**

`EntryDetail.svelte:166-174` の動作URL入力欄を Task 8 と同じパターンに置き換え（idは `entry-working-body-url`/`entry-working-diff-url` を使用）。

**Step 3: コミット**

```bash
git add frontend/src/EntryDetail.svelte
git commit -m "feat: EntryDetail の動作URLをリンク表示+編集ボタンに変更"
```

---

### Task 11: RewriteRuleManager.svelte（ルール管理UI）

**Files:**
- Create: `frontend/src/RewriteRuleManager.svelte`

**Step 1: EventMappingManager.svelte と同じパターンでルール管理モーダルを作成**

`frontend/src/RewriteRuleManager.svelte`:

```svelte
<script lang="ts">
  import { ListRewriteRules, UpsertRewriteRule, DeleteRewriteRule } from '../wailsjs/go/app/RewriteHandler'
  import type { dto } from '../wailsjs/go/models'

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let rules: dto.RewriteRuleDTO[] = []
  let error = ''

  let newRuleType = 'replace'
  let newPattern = ''
  let newReplacement = ''
  let newPriority = 0
  let adding = false

  export async function open() {
    error = ''
    resetForm()
    await loadRules()
    dialog.showModal()
  }

  function resetForm() {
    newRuleType = 'replace'
    newPattern = ''
    newReplacement = ''
    newPriority = 0
  }

  async function loadRules() {
    try {
      rules = (await ListRewriteRules()) || []
    } catch (e: any) {
      rules = []
      error = e?.message || 'ルール一覧の取得に失敗しました'
    }
  }

  async function handleAdd() {
    if (!newPattern.trim() || !newReplacement.trim()) return
    adding = true
    error = ''
    try {
      await UpsertRewriteRule(0, newRuleType, newPattern.trim(), newReplacement.trim(), newPriority)
      resetForm()
      await loadRules()
    } catch (e: any) {
      error = e?.message || '追加に失敗しました'
    } finally {
      adding = false
    }
  }

  async function handleDelete(id: number) {
    error = ''
    try {
      await DeleteRewriteRule(id)
      await loadRules()
    } catch (e: any) {
      error = e?.message || '削除に失敗しました'
    }
  }

  function handleClose() {
    dialog.close()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) dialog.close(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">URL書き換えルール管理</h3>

    {#if error}
      <div class="alert alert-error mb-4 py-2 text-sm">{error}</div>
    {/if}

    {#if rules.length > 0}
      <div class="overflow-x-auto">
        <table class="table table-xs">
          <thead>
            <tr>
              <th>タイプ</th>
              <th>パターン</th>
              <th>置換先</th>
              <th>優先度</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each rules as r}
              <tr>
                <td><span class="badge badge-sm badge-outline">{r.ruleType}</span></td>
                <td class="font-mono text-xs">{r.pattern}</td>
                <td class="font-mono text-xs">{r.replacement}</td>
                <td>{r.priority}</td>
                <td>
                  <button class="btn btn-ghost btn-xs text-error" on:click={() => handleDelete(r.id)}>削除</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">ルールが登録されていません</p>
    {/if}

    <div class="divider text-sm">新規追加</div>

    <div class="flex gap-2 items-end flex-wrap">
      <div class="form-control w-24">
        <label class="label py-0" for="rule-type">
          <span class="label-text text-xs">タイプ</span>
        </label>
        <select id="rule-type" class="select select-bordered select-sm" bind:value={newRuleType}>
          <option value="replace">replace</option>
          <option value="regex">regex</option>
        </select>
      </div>
      <div class="form-control flex-1">
        <label class="label py-0" for="rule-pattern">
          <span class="label-text text-xs">パターン</span>
        </label>
        <input
          id="rule-pattern"
          type="text"
          class="input input-bordered input-sm"
          bind:value={newPattern}
          placeholder="old-host.com/path"
        />
      </div>
      <div class="form-control flex-1">
        <label class="label py-0" for="rule-replacement">
          <span class="label-text text-xs">置換先</span>
        </label>
        <input
          id="rule-replacement"
          type="text"
          class="input input-bordered input-sm"
          bind:value={newReplacement}
          placeholder="new-host.com/path"
        />
      </div>
      <div class="form-control w-20">
        <label class="label py-0" for="rule-priority">
          <span class="label-text text-xs">優先度</span>
        </label>
        <input
          id="rule-priority"
          type="number"
          class="input input-bordered input-sm"
          bind:value={newPriority}
        />
      </div>
      <button
        class="btn btn-sm btn-outline shrink-0"
        on:click={handleAdd}
        disabled={adding || !newPattern.trim() || !newReplacement.trim()}
      >
        {adding ? '追加中...' : '追加'}
      </button>
    </div>

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
```

**Step 2: コミット**

```bash
git add frontend/src/RewriteRuleManager.svelte
git commit -m "feat: URL書き換えルール管理モーダルを追加"
```

---

### Task 12: SongTable に「動作URL推定」ボタンを追加

**Files:**
- Modify: `frontend/src/SongTable.svelte`

**Step 1: import と変数を追加**

`SongTable.svelte` の import 部分に追加:

```typescript
import RewriteRuleManager from './RewriteRuleManager.svelte'
import { InferWorkingURLs } from '../wailsjs/go/app/RewriteHandler'
```

変数宣言:

```typescript
let rewriteRuleModal: RewriteRuleManager
let inferringUrls = false
let inferUrlResult = ''
```

**Step 2: 推定実行関数を追加**

```typescript
async function runInferWorkingURLs() {
  inferringUrls = true
  inferUrlResult = ''
  try {
    const result = await InferWorkingURLs()
    inferUrlResult = `${result.applied}件適用 / ${result.skipped}件スキップ / ${result.total}件中`
    setTimeout(() => inferUrlResult = '', 5000)
    loadSongs()
  } catch (e: any) {
    inferUrlResult = e?.message || '推定に失敗しました'
  } finally {
    inferringUrls = false
  }
}
```

**Step 3: ツールバーにボタンを追加**

`SongTable.svelte:134-136` の `<div class="flex items-center gap-2">` 内、メタ推測ボタンの前に追加:

```svelte
<button class="btn btn-xs btn-outline" on:click|stopPropagation={() => rewriteRuleModal.open()}>URL書き換え設定</button>
{#if inferUrlResult}
  <span class="text-xs text-success">{inferUrlResult}</span>
{/if}
<button
  class="btn btn-xs btn-outline"
  on:click|stopPropagation={runInferWorkingURLs}
  disabled={inferringUrls}
>
  {inferringUrls ? '推定中...' : '動作URL推定'}
</button>
```

**Step 4: コンポーネント末尾にモーダルを配置**

InferenceModal と同様に、SongTable.svelte の末尾（`</div>` の後、テンプレート最後）に追加:

```svelte
<RewriteRuleManager bind:this={rewriteRuleModal} />
```

**Step 5: コミット**

```bash
git add frontend/src/SongTable.svelte
git commit -m "feat: SongTable に動作URL推定ボタンとURL書き換え設定を追加"
```

---

### Task 13: ChartListView に「動作URL推定」ボタンを追加

**Files:**
- Modify: `frontend/src/ChartListView.svelte`

**Step 1: import と変数を追加**

SongTable と同様に import + 変数 + runInferWorkingURLs 関数を追加。

**Step 2: ツールバーにボタンを追加**

IR一括取得ボタンの後（SearchInput の前）に、SongTable と同じ3つの要素（URL書き換え設定ボタン、結果表示、動作URL推定ボタン）を追加。

**Step 3: コンポーネント末尾にモーダルを配置**

```svelte
<RewriteRuleManager bind:this={rewriteRuleModal} />
```

**Step 4: コミット**

```bash
git add frontend/src/ChartListView.svelte
git commit -m "feat: ChartListView に動作URL推定ボタンとURL書き換え設定を追加"
```

---

### Task 14: ビルド確認と最終テスト

**Step 1: Go テスト全体実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./... -v`
Expected: ALL PASS

**Step 2: フロントエンドビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: SUCCESS

**Step 3: Wails ビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: SUCCESS

**Step 4: 動作確認ポイント（手動）**

- [ ] 楽曲一覧ツールバーに「URL書き換え設定」「動作URL推定」ボタンが表示される
- [ ] 譜面一覧ツールバーにも同様
- [ ] URL書き換え設定モーダルでルールの追加・削除ができる
- [ ] 動作URL推定で適用件数が表示される
- [ ] 各詳細ビューで動作URLがリンク表示され、編集ボタンで入力欄に切り替わる
