# 難易度表エントリ詳細: 重複時の全ファイルパス列挙 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 難易度表の詳細パネルで、重複（同一md5が複数行）の譜面について全ファイルパスを列挙し、各パスにフォルダを開くボタンを付ける。

**Architecture:** `SongdataReader` に md5 から全パスを返すメソッドを追加し、`ChartHandler` 経由でフロントに公開。`EntryDetail.svelte` は `status === 'duplicate'` のとき全パスを取得して一覧表示する。

**Tech Stack:** Go 1.24 + Wails v2 / Svelte 4 + TypeScript / SQLite（modernc.org/sqlite）/ DaisyUI

---

## ファイル構成

- Modify: `internal/adapter/persistence/songdata_reader.go` — `ListChartPathsByMD5` 追加
- Test: `internal/adapter/persistence/songdata_reader_test.go` — Reader メソッドのテスト追加
- Modify: `internal/app/chart_handler.go` — `ListChartPathsByMD5` ハンドラ追加
- Modify: `frontend/wailsjs/go/app/ChartHandler.js` / `.d.ts` — バインディング追加
- Modify: `frontend/src/views/EntryDetail.svelte` — 重複時のパス一覧表示

---

### Task 1: Reader メソッド `ListChartPathsByMD5`

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`（`GetChartByMD5` の直後、`584` 行目付近の後ろに追加）
- Test: `internal/adapter/persistence/songdata_reader_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/persistence/songdata_reader_test.go` の末尾（`songTitles` ヘルパーの前、`549` 行目付近）に追加する。
testdata/songdata.db には重複md5が存在する（例: 同一md5が2行）。テスト内で重複md5を動的に探して検証する。

```go
func TestListChartPathsByMD5_Duplicate(t *testing.T) {
	reader, db := setupSongdataReader(t)
	ctx := context.Background()

	// testdata から「同一md5が複数行」存在するmd5を1つ取得（空md5は除外）
	var md5 string
	var cnt int
	err := db.QueryRowContext(ctx, `
		SELECT md5, COUNT(*) c FROM songdata.song
		WHERE md5 != '' GROUP BY md5 HAVING c > 1 ORDER BY c DESC, md5 LIMIT 1`,
	).Scan(&md5, &cnt)
	if err != nil {
		t.Fatalf("前提: 重複md5の取得に失敗: %v", err)
	}

	paths, err := reader.ListChartPathsByMD5(ctx, md5)
	if err != nil {
		t.Fatalf("ListChartPathsByMD5 failed: %v", err)
	}

	// 重複件数ぶんのパスが返ること
	if len(paths) != cnt {
		t.Errorf("len(paths) = %d, want %d", len(paths), cnt)
	}

	// 各パスが非空であること
	for i, p := range paths {
		if p == "" {
			t.Errorf("paths[%d] is empty", i)
		}
	}

	// ORDER BY path で昇順ソートされていること
	for i := 1; i < len(paths); i++ {
		if paths[i-1] > paths[i] {
			t.Errorf("paths not sorted: %q > %q", paths[i-1], paths[i])
		}
	}
}

func TestListChartPathsByMD5_NotFound(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	paths, err := reader.ListChartPathsByMD5(ctx, "nonexistent_md5_zzz")
	if err != nil {
		t.Fatalf("ListChartPathsByMD5 error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("len(paths) = %d, want 0", len(paths))
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/adapter/persistence/ -run TestListChartPathsByMD5 -v`
Expected: コンパイルエラー（`reader.ListChartPathsByMD5` undefined）

- [ ] **Step 3: 最小実装を書く**

`internal/adapter/persistence/songdata_reader.go` の `GetChartByMD5` 関数（`631` 行目の閉じ括弧）の直後に追加する。

```go
// ListChartPathsByMD5 は指定md5を持つ全譜面のパスを返す（重複時の全パス列挙用）
func (r *SongdataReader) ListChartPathsByMD5(ctx context.Context, md5 string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT path FROM songdata.song WHERE md5 = ? ORDER BY path`, md5)
	if err != nil {
		return nil, fmt.Errorf("ListChartPathsByMD5 query: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("ListChartPathsByMD5 scan: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
```

- [ ] **Step 4: テストを実行して成功を確認**

Run: `go test ./internal/adapter/persistence/ -run TestListChartPathsByMD5 -v`
Expected: PASS（2テストとも）

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go internal/adapter/persistence/songdata_reader_test.go
git commit -m "feat(persistence): md5から全譜面パスを返すListChartPathsByMD5を追加"
```

---

### Task 2: ChartHandler に公開メソッド + Wails バインディング

**Files:**
- Modify: `internal/app/chart_handler.go`（`GetChartMetaByMD5` の後ろ、`83` 行目付近の末尾に追加）
- Modify: `frontend/wailsjs/go/app/ChartHandler.js`
- Modify: `frontend/wailsjs/go/app/ChartHandler.d.ts`

- [ ] **Step 1: ハンドラメソッドを追加**

`internal/app/chart_handler.go` の末尾（`GetChartMetaByMD5` 関数の閉じ括弧の後）に追加する。

```go
// ListChartPathsByMD5 は指定md5を持つ全譜面のパスを返す（難易度表詳細の重複時表示用）
func (h *ChartHandler) ListChartPathsByMD5(md5 string) ([]string, error) {
	return h.songReader.ListChartPathsByMD5(h.ctx, md5)
}
```

- [ ] **Step 2: ビルドして通ることを確認**

Run: `go build ./...`
Expected: エラーなし（出力なしで終了）

- [ ] **Step 3: Wails バインディング(JS)を追加**

`frontend/wailsjs/go/app/ChartHandler.js` の `GetChartMetaByMD5` の後（`11` 行目の後）に追加する。

```javascript
export function ListChartPathsByMD5(arg1) {
  return window['go']['app']['ChartHandler']['ListChartPathsByMD5'](arg1);
}
```

- [ ] **Step 4: Wails バインディング(型定義)を追加**

`frontend/wailsjs/go/app/ChartHandler.d.ts` の `GetChartMetaByMD5` の後（`8` 行目の後）に追加する。

```typescript
export function ListChartPathsByMD5(arg1:string):Promise<Array<string>>;
```

- [ ] **Step 5: コミット**

```bash
git add internal/app/chart_handler.go frontend/wailsjs/go/app/ChartHandler.js frontend/wailsjs/go/app/ChartHandler.d.ts
git commit -m "feat(app): ChartHandlerにListChartPathsByMD5を公開しバインディング追加"
```

---

### Task 3: EntryDetail で重複時に全パスを一覧表示

**Files:**
- Modify: `frontend/src/views/EntryDetail.svelte`

- [ ] **Step 1: import に新バインディングを追加**

`frontend/src/views/EntryDetail.svelte` の `3` 行目を以下に置き換える。

置換前:
```typescript
  import { GetChartDetailByMD5, GetChartMetaByMD5 } from '../../wailsjs/go/app/ChartHandler'
```
置換後:
```typescript
  import { GetChartDetailByMD5, GetChartMetaByMD5, ListChartPathsByMD5 } from '../../wailsjs/go/app/ChartHandler'
```

- [ ] **Step 2: 状態変数 `dupPaths` を追加**

`30` 行目付近の `let bmsSearchLoading = false` の直後に追加する。

```typescript
  let dupPaths: string[] = []
```

- [ ] **Step 3: `loadEntry` で重複時にパスを取得**

`loadEntry` 関数内、`entryData = await GetDifficultyTableEntry(tid, hash)` の直後に追加する。
（取得失敗や非重複時は空配列にリセットされるよう、関数冒頭のリセット群にも `dupPaths = []` を加える）

`44` 行目付近のリセット群を置換する。

置換前:
```typescript
    loading = true
    entryData = null
    chart = null
    irMeta = null
    try {
      entryData = await GetDifficultyTableEntry(tid, hash)
```
置換後:
```typescript
    loading = true
    entryData = null
    chart = null
    irMeta = null
    dupPaths = []
    try {
      entryData = await GetDifficultyTableEntry(tid, hash)
      if (entryData?.status === 'duplicate') {
        try {
          dupPaths = await ListChartPathsByMD5(hash)
        } catch (e) {
          console.error('Failed to load duplicate paths:', e)
          dupPaths = []
        }
      }
```

- [ ] **Step 4: ヘッダーの単一フォルダボタンを重複時は非表示にする**

`105` 行目付近の `OpenFolderButton` を、重複時に隠すよう条件を付ける。

置換前:
```svelte
          <OpenFolderButton path={chart?.path} title="インストール先フォルダを開く" />
```
置換後:
```svelte
          {#if entryData.status !== 'duplicate'}
            <OpenFolderButton path={chart?.path} title="インストール先フォルダを開く" />
          {/if}
```

- [ ] **Step 5: ファイルパス一覧セクションを追加**

`ChartInfoCard`（`134` 行目付近の `{#if chart}` ブロック）の直前に、重複時のパス一覧セクションを追加する。

置換前:
```svelte
    <!-- 譜面メタデータ（導入済の場合のみ） -->
    {#if chart}
      <ChartInfoCard {chart} paneId="entry" />
    {/if}
```
置換後:
```svelte
    <!-- ファイルパス一覧（重複時のみ） -->
    {#if entryData.status === 'duplicate' && dupPaths.length > 0}
      <div class="bg-base-200 rounded-lg p-3">
        <div class="text-sm font-semibold mb-2">ファイルパス一覧 ({dupPaths.length}件)</div>
        <div class="space-y-1">
          {#each dupPaths as p}
            <div class="text-xs text-base-content/70 break-all flex items-center gap-1">
              <OpenFolderButton path={p} size="xs" title="フォルダを開く" />
              <span>{p}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- 譜面メタデータ（導入済の場合のみ） -->
    {#if chart}
      <ChartInfoCard {chart} paneId="entry" />
    {/if}
```

- [ ] **Step 6: フロントエンドの型チェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: `EntryDetail.svelte` 由来の新規エラーがないこと（既存の警告は許容）

- [ ] **Step 7: 手動確認（wails dev）**

Run: `wails dev`
確認内容:
- 難易度表タブで「重複」バッジのエントリを選択 → 「ファイルパス一覧 (N件)」が表示され、全パスが列挙される
- 各行のフォルダアイコンを押すと該当フォルダがファイルマネージャーで開く
- ヘッダー右の単一フォルダボタンは重複時は表示されない
- 「導入済」エントリでは一覧は出ず、従来どおりヘッダーに単一フォルダボタンが1つ表示される

- [ ] **Step 8: コミット**

```bash
git add frontend/src/views/EntryDetail.svelte
git commit -m "feat(frontend): 難易度表詳細で重複時に全ファイルパスを一覧表示"
```

---

### Task 4: マニュアル更新

**Files:**
- Modify: `docs/manual.md`（難易度表セクション）

- [ ] **Step 1: 該当セクションを確認**

Run: `grep -n "難易度表\|重複\|フォルダを開く" docs/manual.md`
難易度表の詳細表示に関する記述を特定する。

- [ ] **Step 2: 記述を追記**

難易度表の詳細パネルに関する説明箇所へ、「重複している譜面（同一md5が複数導入済み）では、全インストール先のファイルパスが一覧表示され、それぞれからフォルダを開ける」旨を、既存の文体に合わせて1〜2文で追記する。
（該当セクションが存在しない場合は、難易度表の機能説明の末尾に追記する）

- [ ] **Step 3: コミット**

```bash
git add docs/manual.md
git commit -m "docs(manual): 難易度表詳細の重複時パス一覧を追記"
```

---

## Self-Review メモ

- spec の「バックエンド: Reader + ハンドラ公開」→ Task 1, 2 で実装
- spec の「フロント: 重複時のみ一覧 / 各行フルパス＋OpenFolderButton / 重複時はヘッダー単一ボタン非表示」→ Task 3 で実装
- spec の「テスト」→ Task 1 で Reader テスト（重複/該当なし）、Task 3 Step 7-8 で手動確認
- spec の「マニュアル」→ Task 4
- 型整合: `ListChartPathsByMD5` のシグネチャは Go (`[]string`) → JS (`Array<string>`) → Svelte (`string[]`) で一貫
- `OpenFolderButton` は `path` にフルファイルパスを渡せば親フォルダを開く（`OpenFolder` 実装で確認済み）ため変換不要
