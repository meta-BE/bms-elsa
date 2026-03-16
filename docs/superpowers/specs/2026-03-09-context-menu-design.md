# カスタムコンテキストメニュー 設計

## 背景

bms-elsaはWails v2.11.0 + Vite 5.xを使用している。devモードではブラウザデフォルトのコンテキストメニュー（Inspect Element等）が表示されるが、buildでは何も表示されない。

Wails v2の`EnableDefaultContextMenu`オプションはVite 5.xでは動作しないため、フロントエンドでJavaScriptベースのカスタムコンテキストメニューを自前実装する。

## 要件

### 最低限
- カット / コピー / ペースト / 削除の基本操作
- テキスト選択の有無・編集可能要素かどうかで各項目を有効/無効（グレーアウト）制御

### 将来拡張（今回のスコープ外）
- 右クリック位置に応じたカスタムメニュー項目（フォルダを開く、URLコピー等）

## アプローチ選定

| アプローチ | 評価 |
|---|---|
| A. フロントエンドのみ（Svelte + CSS） | **採用** — Wailsバージョン非依存、依存追加なし、自由度高い |
| B. Wails v3移行 + ネイティブメニュー | 却下 — v3 alpha、移行コスト大 |
| C. ライブラリ導入 | 却下 — Svelte 4対応のメンテ済みライブラリが少ない |

## 設計

### アーキテクチャ

汎用コンポーネント `ContextMenu.svelte` を1つ作り、App.svelteに配置。

```
App.svelte
  └─ <ContextMenu />  ← アプリルートに1つだけ
```

- `contextmenu`イベントをwindowレベルでリッスン
- devモード（`import.meta.env.DEV`）では`preventDefault()`せず、ブラウザデフォルトメニューを表示

### コンテキスト判定

クリックされた要素から親をたどって`data-context-type`属性を探索（将来拡張用）。

```html
<!-- 将来: 楽曲一覧の行 -->
<div data-context-type="song-row" data-context-id="{folderHash}">...</div>
```

属性が見つからない場合 → 基本メニュー（カット/コピー/ペースト/削除）のみ表示。

### メニュー項目の有効/無効判定

| メニュー項目 | 有効条件 |
|---|---|
| カット | テキスト選択あり **かつ** 編集可能な要素にフォーカス |
| コピー | テキスト選択あり |
| ペースト | 編集可能な要素にフォーカス |
| 削除 | テキスト選択あり **かつ** 編集可能な要素にフォーカス |

「編集可能な要素」= `<input>`, `<textarea>`, `[contenteditable]`

判定は`contextmenu`イベント発火時に `window.getSelection()` と `document.activeElement` から行う。

### クリップボード操作

- コピー: `navigator.clipboard.writeText(selection)`
- ペースト: `navigator.clipboard.readText()` → `document.execCommand('insertText', false, text)`
- カット: コピー + `document.execCommand('delete')`
- 削除: `document.execCommand('delete')`

### メニュー表示

- 表示位置: `e.clientX`, `e.clientY`（画面端にはみ出す場合は反転）
- 閉じるトリガー: window click / Escape キー / スクロール
- スタイル: DaisyUI `menu` コンポーネント（`bg-base-100 shadow-lg rounded-box`）
- グレーアウト: `opacity-40 cursor-default` + クリック無効

### 将来拡張ポイント

`data-context-type` の値に応じてメニュー項目を分岐追加できる設計。

```
song-row → フォルダを開く / タイトルをコピー + 基本メニュー
chart-row → フォルダを開く / タイトルをコピー + 基本メニュー
difficulty-row → URLをコピー（URLありの場合）+ 基本メニュー
```

## 依存追加

なし
