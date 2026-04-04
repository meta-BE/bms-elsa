# ProgressBar コンポーネント設計

## 概要

進捗バーのUI表示を統一する再利用可能なSvelteコンポーネントを作成する。
現在、Settings.svelteでは `<progress>` + 横テキスト、他の箇所ではテキストのみ（`n / m`）と表示が分かれている。
これらを「進捗バーの上に件数をオーバーレイ表示、ゲージ境界でテキスト色が2色に分かれる」統一デザインに置き換える。

## デザイン仕様

### ビジュアル

- **方式**: clip-path 2色分割（中央配置）
- **バー高さ**: 16px（`h-4`）
- **テキストサイズ**: 10px（`text-[10px]`）
- **角丸**: `rounded-full`
- **背景色**: `bg-base-300`（daisyUI テーマ）
- **ゲージ色**: `bg-primary`（daisyUI テーマ）
- **ゲージ上テキスト色**: `text-primary-content`
- **背景上テキスト色**: `text-base-content/70`
- **テキストフォーマット**: `current.toLocaleString() / total.toLocaleString()`（中央配置）

### clip-path 2色分割の仕組み

同一テキストを2層重ねる:
1. 下層: 背景色上のテキスト（`text-base-content/70`、全幅表示）
2. 上層: ゲージ色上のテキスト（`text-primary-content`、`clip-path: inset(0 {100-percent}% 0 0)` でゲージ幅のみ表示）

ゲージ境界でテキスト色が自然に分かれ、どの進捗率でも常にコントラストが確保される。

## コンポーネントインターフェース

### ファイル配置

`frontend/src/components/ProgressBar.svelte`

### Props

| Prop | 型 | 必須 | 説明 |
|------|----|------|------|
| `current` | `number` | ○ | 現在の進捗値 |
| `total` | `number` | ○ | 全体数 |

### Props（続き）

| Prop | 型 | 必須 | 説明 |
|------|----|------|------|
| `cancelable` | `boolean` | - | `true` の場合、停止ボタンを表示する。デフォルト `false` |

### Events

| Event | 説明 |
|-------|------|
| `cancel` | `cancelable` が `true` のとき、停止ボタンクリックで `createEventDispatcher` 経由でディスパッチ |

### 幅の制御

コンポーネント自体は `flex-1` で親の残り幅を埋める。幅の制御は呼び出し側の責任。

### 使用例

```svelte
<!-- 停止ボタンなし（Settings.svelte） -->
<ProgressBar current={scanProgress.current} total={scanProgress.total} />

<!-- 停止ボタンあり（SongTable.svelte等） -->
<ProgressBar current={syncProgress.current} total={syncProgress.total} cancelable on:cancel={stopSync} />
```

## DOM構造

```html
<div class="flex items-center gap-2 flex-1">
  <!-- 進捗バー -->
  <div class="relative h-4 rounded-full bg-base-300 overflow-hidden flex-1">
    <!-- ゲージ -->
    <div class="absolute inset-y-0 left-0 bg-primary rounded-full"
         style="width: {percent}%"></div>
    <!-- テキスト: 背景色上（明色） -->
    <span class="absolute inset-0 flex items-center justify-center
                 text-[10px] font-semibold text-base-content/70">
      {label}
    </span>
    <!-- テキスト: ゲージ上（暗色、clip-pathで制限） -->
    <span class="absolute inset-0 flex items-center justify-center
                 text-[10px] font-semibold text-primary-content"
          style="clip-path: inset(0 {100-percent}% 0 0)">
      {label}
    </span>
  </div>
  <!-- 停止ボタン（cancelable時のみ） -->
  {#if cancelable}
    <button class="btn btn-xs btn-error btn-outline" on:click={() => dispatch('cancel')}>停止</button>
  {/if}
</div>
```

## リアクティブロジック

```typescript
import { createEventDispatcher } from 'svelte'

export let current: number
export let total: number
export let cancelable = false

const dispatch = createEventDispatcher<{ cancel: void }>()

$: percent = total > 0 ? Math.min((current / total) * 100, 100) : 0
$: label = `${current.toLocaleString()} / ${total.toLocaleString()}`
```

## 適用箇所

| ファイル | 現状の表示 | 変更内容 |
|---------|-----------|---------|
| `frontend/src/settings/Settings.svelte` | `<progress>` + 横テキスト ×3 | `<ProgressBar>` ×3（停止ボタンなし） |
| `frontend/src/views/SongTable.svelte` | `同期中: n / m` テキスト + 停止ボタン | `<ProgressBar cancelable on:cancel>` |
| `frontend/src/components/BulkFetchButton.svelte` | `取得中: n / m` テキスト + 停止ボタン | `<ProgressBar cancelable on:cancel>` |
| `frontend/src/views/DiffImportView.svelte` | `推定中: n / m` テキスト + 停止ボタン | `<ProgressBar cancelable on:cancel>` |
| `frontend/src/settings/DifficultyTableSettings.svelte` | `更新中: n/m テーブル完了` テキスト + 停止ボタン | `<ProgressBar cancelable on:cancel>`（結果リスト部分はそのまま） |

各箇所の状態管理（state/running変数、result表示、error表示）やイベント購読ロジックは変更しない。進捗表示のHTML部分のみコンポーネントに差し替える。
