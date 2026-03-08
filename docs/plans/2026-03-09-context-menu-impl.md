# カスタムコンテキストメニュー Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** buildモードでカット/コピー/ペースト/削除の基本操作ができるカスタムコンテキストメニューを実装する

**Architecture:** `ContextMenu.svelte`コンポーネントをApp.svelteに1つ配置し、windowレベルの`contextmenu`イベントでメニューを表示。devモードではブラウザデフォルトメニューをそのまま使用。クリップボード操作はNavigator Clipboard API + execCommandで実現。

**Tech Stack:** Svelte 4, TypeScript, DaisyUI/TailwindCSS, Wails v2 (フロントエンドのみ)

---

### Task 1: ContextMenu.svelte コンポーネント作成

**Files:**
- Create: `frontend/src/components/ContextMenu.svelte`

**Step 1: コンポーネントの基本構造を作成**

`frontend/src/components/ContextMenu.svelte` を以下の内容で作成する。

```svelte
<script lang="ts">
  type MenuItem = {
    label: string
    action: () => void
    disabled: boolean
  }

  let visible = false
  let x = 0
  let y = 0
  let items: MenuItem[] = []

  // 編集可能な要素かどうか判定
  function isEditable(el: Element | null): boolean {
    if (!el) return false
    const tag = el.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA') return true
    if ((el as HTMLElement).isContentEditable) return true
    return false
  }

  // テキスト選択があるか判定
  function hasSelection(): boolean {
    const sel = window.getSelection()
    return !!sel && sel.toString().length > 0
  }

  function handleContextMenu(e: MouseEvent) {
    // devモードではブラウザデフォルトメニューを表示
    if (import.meta.env.DEV) return

    e.preventDefault()

    const target = e.target as Element
    const editable = isEditable(document.activeElement)
    const selected = hasSelection()

    items = [
      {
        label: 'カット',
        disabled: !(selected && editable),
        action: async () => {
          const sel = window.getSelection()
          if (sel) {
            await navigator.clipboard.writeText(sel.toString())
            document.execCommand('delete')
          }
        },
      },
      {
        label: 'コピー',
        disabled: !selected,
        action: async () => {
          const sel = window.getSelection()
          if (sel) {
            await navigator.clipboard.writeText(sel.toString())
          }
        },
      },
      {
        label: 'ペースト',
        disabled: !editable,
        action: async () => {
          const text = await navigator.clipboard.readText()
          document.execCommand('insertText', false, text)
        },
      },
      {
        label: '削除',
        disabled: !(selected && editable),
        action: () => {
          document.execCommand('delete')
        },
      },
    ]

    // 表示位置を計算（画面端ではみ出す場合は反転）
    const menuWidth = 160
    const menuHeight = items.length * 32 + 8
    x = e.clientX + menuWidth > window.innerWidth ? e.clientX - menuWidth : e.clientX
    y = e.clientY + menuHeight > window.innerHeight ? e.clientY - menuHeight : e.clientY

    visible = true
  }

  function close() {
    visible = false
  }

  function handleClick(item: MenuItem) {
    if (item.disabled) return
    item.action()
    close()
  }
</script>

<svelte:window
  on:contextmenu={handleContextMenu}
  on:click={close}
  on:keydown={(e) => { if (e.key === 'Escape') close() }}
  on:scroll={close}
/>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div
    class="fixed z-[9999] bg-base-100 border border-base-300 rounded-box shadow-lg py-1 min-w-[160px]"
    style="left: {x}px; top: {y}px;"
    on:click|stopPropagation
  >
    {#each items as item}
      <button
        class="w-full text-left px-4 py-1.5 text-sm transition-colors
          {item.disabled ? 'opacity-40 cursor-default' : 'hover:bg-base-200 cursor-pointer'}"
        on:click={() => handleClick(item)}
        disabled={item.disabled}
      >
        {item.label}
      </button>
    {/each}
  </div>
{/if}
```

**Step 2: ビルド確認**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json`
Expected: エラーなし

**Step 3: コミット**

```bash
git add frontend/src/components/ContextMenu.svelte
git commit -m "feat: カスタムコンテキストメニューコンポーネントを追加"
```

---

### Task 2: App.svelteにContextMenuを配置

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: ContextMenuをインポートして配置**

`frontend/src/App.svelte` に以下の変更を加える:

1. scriptブロック先頭のimportに追加:
```typescript
import ContextMenu from './components/ContextMenu.svelte'
```

2. テンプレート末尾（`</div>` と `<style>` の間、`<RewriteRuleManager>` の後）に追加:
```svelte
<ContextMenu />
```

**Step 2: ビルド確認**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json`
Expected: エラーなし

**Step 3: 動作確認**

Run: `cd /path/to/bms-elsa && wails dev`

確認事項:
- devモード: 右クリックでブラウザデフォルトメニュー（Inspect Element等）が表示される
- テキスト選択 → 右クリック: ブラウザデフォルトメニューが表示される

※ buildモードでの確認は `wails build` 後に行う。

**Step 4: コミット**

```bash
git add frontend/src/App.svelte
git commit -m "feat: App.svelteにContextMenuコンポーネントを配置"
```

---

### Task 3: buildモードでの動作確認

**Step 1: ビルド**

Run: `cd /path/to/bms-elsa && wails build`

**Step 2: 動作確認**

ビルドされたアプリを起動し、以下を確認:

- テキスト選択なし + 非編集要素上で右クリック → メニュー表示、全項目グレーアウト（ペーストも含む）
- テキスト選択あり + 非編集要素上で右クリック → 「コピー」のみ有効、他はグレーアウト
- 入力フィールド上（テキスト選択なし）で右クリック → 「ペースト」のみ有効
- 入力フィールド上（テキスト選択あり）で右クリック → 全項目有効
- メニュー外クリック → メニュー閉じる
- Escキー → メニュー閉じる
- 画面右端/下端での右クリック → メニューが画面内に収まる
