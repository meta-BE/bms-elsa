# 難易度表の並び替え機能 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 難易度表設定モーダルでドラッグ&ドロップによる並び替えを可能にし、セレクターの表示順に反映する。

**Architecture:** DBに `sort_order` カラムを追加し、バックエンドに並び替えAPIを新設。フロントエンドは `svelte-dnd-action` でDnD UIを実装し、`on:finalize` で並び替えAPIを呼び出す。

**Tech Stack:** Go (SQLite), Svelte 4, svelte-dnd-action, DaisyUI

---

## Chunk 1: バックエンド（DB + Repository + Handler）

### Task 1: マイグレーションに sort_order カラム追加

**Files:**
- Modify: `internal/adapter/persistence/migrations.go:45-55` (CREATE TABLE)
- Modify: `internal/adapter/persistence/migrations.go:187` (末尾にマイグレーション追加)

- [ ] **Step 1: CREATE TABLE文に sort_order カラムを追加**

`difficulty_table` の CREATE TABLE 文（45-55行目）に `sort_order` を追加:

```sql
CREATE TABLE IF NOT EXISTS difficulty_table (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    url         TEXT NOT NULL UNIQUE,
    header_url  TEXT NOT NULL,
    data_url    TEXT NOT NULL,
    name        TEXT NOT NULL,
    symbol      TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    fetched_at  TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
)
```

- [ ] **Step 2: 既存DBへの冪等マイグレーションを追加**

`RunMigrations` 関数の末尾（`return nil` の直前、wav_minhash マイグレーションの後）に追加:

```go
// sort_orderカラムの追加（冪等）
var hasSortOrder int
_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('difficulty_table') WHERE name='sort_order'`).Scan(&hasSortOrder)
if hasSortOrder == 0 {
    if _, err := db.Exec(`ALTER TABLE difficulty_table ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
        return fmt.Errorf("add sort_order column: %w", err)
    }
    // 既存行にはid順でsort_orderを振る
    if _, err := db.Exec(`UPDATE difficulty_table SET sort_order = id`); err != nil {
        return fmt.Errorf("init sort_order: %w", err)
    }
}
```

- [ ] **Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし

- [ ] **Step 4: コミット**

```bash
git add internal/adapter/persistence/migrations.go
git commit -m "feat: difficulty_table に sort_order カラムを追加"
```

---

### Task 2: Repository に sort_order 対応を追加

**Files:**
- Modify: `internal/adapter/persistence/difficulty_table_repository.go`

- [ ] **Step 1: ListTables の ORDER BY を変更**

41-46行目を変更:

```go
func (r *DifficultyTableRepository) ListTables(ctx context.Context) ([]DifficultyTable, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, url, header_url, data_url, name, symbol, fetched_at
		FROM difficulty_table
		ORDER BY sort_order, name
	`)
```

- [ ] **Step 2: InsertTable で sort_order を MAX+1 に設定**

68-78行目を変更:

```go
func (r *DifficultyTableRepository) InsertTable(ctx context.Context, t DifficultyTable) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO difficulty_table (url, header_url, data_url, name, symbol, sort_order, fetched_at)
		VALUES (?, ?, ?, ?, ?, COALESCE((SELECT MAX(sort_order) FROM difficulty_table), 0) + 1, datetime('now'))
	`, t.URL, t.HeaderURL, t.DataURL, t.Name, t.Symbol)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}
```

- [ ] **Step 3: ReorderTables メソッドを追加**

`DeleteTable` メソッドの後（92行目の後）に追加:

```go
func (r *DifficultyTableRepository) ReorderTables(ctx context.Context, ids []int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE difficulty_table SET sort_order = ?, updated_at = datetime('now') WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, id := range ids {
		if _, err := stmt.ExecContext(ctx, i+1, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}
```

- [ ] **Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/persistence/difficulty_table_repository.go
git commit -m "feat: Repository に sort_order 対応と ReorderTables を追加"
```

---

### Task 3: Handler に ReorderDifficultyTables を追加

**Files:**
- Modify: `internal/app/difficulty_table_handler.go`

- [ ] **Step 1: ReorderDifficultyTables メソッドを追加**

`RemoveDifficultyTable` メソッドの後（164行目の後）に追加:

```go
func (h *DifficultyTableHandler) ReorderDifficultyTables(ids []int) error {
	return h.dtRepo.ReorderTables(h.ctx, ids)
}
```

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし

- [ ] **Step 3: コミット**

```bash
git add internal/app/difficulty_table_handler.go
git commit -m "feat: DifficultyTableHandler に ReorderDifficultyTables を追加"
```

---

## Chunk 2: フロントエンド（DnD UI）

### Task 4: svelte-dnd-action をインストール

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: パッケージインストール**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm install svelte-dnd-action`

- [ ] **Step 2: コミット**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore: svelte-dnd-action を追加"
```

---

### Task 5: TypeScript 型定義を追加

**Files:**
- Create: `frontend/src/app.d.ts`

- [ ] **Step 1: svelte-dnd-action のカスタムイベント型定義を作成**

```typescript
// svelte-dnd-action の Svelte 4 用カスタムイベント型定義
declare namespace svelteHTML {
  interface HTMLAttributes<T> {
    "on:consider"?: (event: CustomEvent) => void;
    "on:finalize"?: (event: CustomEvent) => void;
  }
}
```

- [ ] **Step 2: コミット**

```bash
git add frontend/src/app.d.ts
git commit -m "chore: svelte-dnd-action の TypeScript 型定義を追加"
```

---

### Task 6: DifficultyTableSettings.svelte にDnD並び替えを実装

**Files:**
- Modify: `frontend/src/settings/DifficultyTableSettings.svelte`

- [ ] **Step 1: import を追加**

script 冒頭に追加:

```typescript
import { dndzone } from 'svelte-dnd-action'
import { flip } from 'svelte/animate'
```

Wails バインディングの import に `ReorderDifficultyTables` を追加:

```typescript
import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTables, ReorderDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'
```

- [ ] **Step 2: DnD 定数とハンドラ関数を追加**

`let adding = false` の後に追加:

```typescript
const flipDurationMs = 200

function handleDndConsider(e: CustomEvent) {
  tables = e.detail.items
}

async function handleDndFinalize(e: CustomEvent) {
  tables = e.detail.items
  const ids = tables.map((t: any) => t.id)
  try {
    await ReorderDifficultyTables(ids)
  } catch (e) {
    console.error('並び替え保存に失敗:', e)
    await loadTables()
  }
}
```

- [ ] **Step 3: テンプレートの `<tbody>` を DnD ゾーンに変更し、グリップハンドルを追加**

テンプレートの `<table>` セクション全体（80-103行目）を以下に置き換え:

```svelte
        <table class="table table-xs">
          <thead>
            <tr>
              <th class="w-6"></th>
              <th>名前</th>
              <th>記号</th>
              <th>譜面数</th>
              <th>最終取得</th>
              <th></th>
            </tr>
          </thead>
          <tbody use:dndzone={{ items: tables, flipDurationMs }} on:consider={handleDndConsider} on:finalize={handleDndFinalize}>
            {#each tables as t (t.id)}
              <tr animate:flip={{ duration: flipDurationMs }}>
                <td class="cursor-grab text-base-content/30 text-center select-none">⠿</td>
                <td>{t.name}</td>
                <td>{t.symbol}</td>
                <td>{t.entryCount}</td>
                <td class="text-xs text-base-content/50">{t.fetchedAt || '未取得'}</td>
                <td>
                  <button class="btn btn-ghost btn-xs text-error" on:click={() => handleRemoveTable(t.id)}>削除</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
```

変更ポイント:
- `<thead>` にグリップ列（`<th class="w-6"></th>`）を追加
- `<tbody>` に `use:dndzone` と `on:consider`/`on:finalize` を追加
- `{#each}` に `(t.id)` キーを追加（DnD 必須）
- `<tr>` に `animate:flip` を追加
- 各行の先頭に `⠿` グリップハンドルの `<td>` を追加

- [ ] **Step 4: Svelte チェック**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run check`
Expected: エラーなし（警告のみ許容）

- [ ] **Step 5: コミット**

```bash
git add frontend/src/settings/DifficultyTableSettings.svelte
git commit -m "feat: 難易度表設定モーダルにDnD並び替え機能を追加"
```
