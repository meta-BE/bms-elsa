# コンテキストメニュー リンクアクション追加 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** リンク上の右クリック時にコンテキストメニューへ「開く」「URLをコピー」を追加する

**Architecture:** 既存の `ContextMenu.svelte` の `handleContextMenu` でリンク検出を追加し、リンク項目とテキスト編集項目を2グループで管理・描画する。`OpenURL`（Wailsバインディング）でシステムブラウザを開き、`ClipboardSetText`（Wailsランタイム）でURLをコピーする。

**Tech Stack:** Svelte 4, TypeScript, Wails v2 runtime/bindings

---

## ファイル構成

- Modify: `frontend/src/components/ContextMenu.svelte` — リンク検出、リンクメニュー項目追加、2グループ描画

変更は1ファイルのみ。

---

## Chunk 1: 実装

### Task 1: リンク検出とメニュー項目追加

**Files:**
- Modify: `frontend/src/components/ContextMenu.svelte`

**背景:**
- 現在の `ContextMenu.svelte` はテキスト編集系メニュー（カット/コピー/ペースト/削除）のみ
- `items: MenuItem[]` 1配列でメニューを管理し、全disabledなら非表示
- `OpenURL` は `../../wailsjs/go/main/App` からインポート可能（`App.svelte` で使用実績あり）
- `ClipboardSetText` は既にインポート済み

- [ ] **Step 1: importに `OpenURL` を追加**

`frontend/src/components/ContextMenu.svelte` の先頭に `OpenURL` のインポートを追加:

```ts
import { OpenURL } from '../../wailsjs/go/main/App'
```

既存のインポート行の直後に配置する。

- [ ] **Step 2: state変数を `linkItems` と `editItems` の2配列に変更**

既存の `let items: MenuItem[] = []` を以下に置換:

```ts
let linkItems: MenuItem[] = []
let editItems: MenuItem[] = []
```

- [ ] **Step 3: `handleContextMenu` にリンク検出ロジックを追加**

`e.preventDefault()` の直後（L79の後）にリンク検出を追加:

```ts
// リンク検出
const anchor = (e.target as Element).closest('a[href]')
const href = anchor?.getAttribute('href') ?? ''
```

- [ ] **Step 4: リンクメニュー項目を構築**

既存の `const newItems: MenuItem[] = [...]` の前にリンク項目を構築:

```ts
const newLinkItems: MenuItem[] = []
if (href) {
  newLinkItems.push(
    {
      label: '開く',
      disabled: false,
      action: () => OpenURL(href),
    },
    {
      label: 'URLをコピー',
      disabled: false,
      action: () => { ClipboardSetText(href) },
    },
  )
}
```

- [ ] **Step 5: 既存の `newItems` を `newEditItems` にリネームし、表示判定を更新**

既存の変数名 `newItems` → `newEditItems` にリネーム。

表示判定を変更（既存の `if (newItems.every(...)) return` を置換）:

```ts
const hasLinkItems = newLinkItems.length > 0
const hasEditItems = newEditItems.some((i) => !i.disabled)

// いずれも表示するものがなければメニューを出さない
if (!hasLinkItems && !hasEditItems) return

linkItems = newLinkItems
editItems = hasEditItems ? newEditItems : []
```

テキスト編集項目が全disabledの場合、`editItems` を空にしてセパレータもろとも非表示にする。

- [ ] **Step 6: メニュー高さ計算を更新**

```ts
const totalItems = linkItems.length + editItems.length
const separatorHeight = linkItems.length > 0 && editItems.length > 0 ? 9 : 0
const menuHeight = totalItems * 32 + 8 + separatorHeight
```

- [ ] **Step 7: テンプレートを2グループ描画に変更**

既存の `{#each items as item}...{/each}` ブロックを以下に置換:

```svelte
{#each linkItems as item}
  <button
    class="block w-full text-left px-4 py-1.5 text-sm whitespace-nowrap transition-colors
      {item.disabled ? 'opacity-40 cursor-default' : 'hover:bg-primary/20 cursor-pointer'}"
    on:click={() => handleClick(item)}
    disabled={item.disabled}
  >
    {item.label}
  </button>
{/each}
{#if linkItems.length > 0 && editItems.length > 0}
  <div class="divider my-0 h-px"></div>
{/if}
{#each editItems as item}
  <button
    class="block w-full text-left px-4 py-1.5 text-sm whitespace-nowrap transition-colors
      {item.disabled ? 'opacity-40 cursor-default' : 'hover:bg-primary/20 cursor-pointer'}"
    on:click={() => handleClick(item)}
    disabled={item.disabled}
  >
    {item.label}
  </button>
{/each}
```

- [ ] **Step 8: ビルド確認**

Run: `cd frontend && npm run build`
Expected: ビルド成功

- [ ] **Step 9: コミット**

```bash
git add frontend/src/components/ContextMenu.svelte
git commit -m "feat: コンテキストメニューにリンクアクション（開く・URLをコピー）を追加"
```

### 手動テスト項目

- 備考内のURL（`{@html linkify(...)}`で生成された`<a>`タグ）を右クリック → 「開く」「URLをコピー」+ セパレータ + テキスト編集メニューが表示される
- 「開く」クリック → システムブラウザでURLが開く
- 「URLをコピー」クリック → クリップボードにURLがコピーされる
- LR2IR情報ヘッダーのリンクを右クリック → 同様にリンクメニューが表示される
- リンク以外の箇所を右クリック → 従来通りテキスト編集メニューのみ
- 編集不可エリアのリンク上で右クリック → リンクメニューのみ表示（テキスト編集は全disabled→非表示）
- input内で右クリック → 従来通りテキスト編集メニューのみ
