# 詳細画面カードの最小化機能 設計

- 起票元: `docs/TODO.md` 「詳細画面の各カードUIを最小化可能に（ペイン×カード種別ごとに開閉状態を保存、最小化ボタンは左上）」
- 作成日: 2026-05-03
- ブランチ: `feat/collapsible-detail-cards`

## 1. 目的とスコープ

詳細画面の各カードを最小化可能にし、開閉状態を「ペイン × カード種別」単位で `config.json` に永続化する。縦スクロールが長くなりがちな詳細画面で、ユーザーが見たいカードだけを展開しておけるようにする。

### 対象ペイン（`paneId`）

- `SongDetail`（楽曲詳細） → `paneId: "song"`
- `ChartDetail`（譜面詳細） → `paneId: "chart"`
- `EntryDetail`（難易度表エントリ詳細） → `paneId: "entry"`
- `DuplicateDetail` は対象カードコンポーネントを使用していないため、結果として paneId は実質 `song`/`chart`/`entry` の 3 種

### 対象カード（`cardId`）

- `ChartInfoCard` → `cardId: "chartInfo"`
- `IRInfoCard` → `cardId: "irInfo"`
- `BMSSearchInfoCard` → `cardId: "bmsSearch"`
- `InstallCandidateCard` → `cardId: "installCandidate"`

### 実際に発生する組み合わせ

|                  | song | chart | entry |
|------------------|:----:|:-----:|:-----:|
| chartInfo        |  ○   |   ○   |   ○   |
| irInfo           |  ○   |   ○   |   ○   |
| bmsSearch        |  ○   |   ○   |   ○   |
| installCandidate |  –   |   –   |   ○   |

### 非対象（このタスクでは触らない）

- 各 Detail ビュー内のヘッダーブロック
- `SongDetail` の譜面一覧テーブル
- `EntryDetail` のエントリ基本情報セクション
- `DuplicateDetail` のメンバーカード（複数個動的に出現するため、静的な `cardId` 列挙では扱えない）
- 最小化アニメーション
- マニュアル更新（必要であれば実装後に判断）

## 2. 要件まとめ

| 項目 | 決定 |
|---|---|
| ペイン × カード種別の状態独立 | A: ペイン別に独立して保存（同じカード種別でもペインごとに別） |
| 対象カード | A: コンポーネント化済みの 4 種のみ |
| 永続化先 | A: `config.json`（既存 `Config` 構造体に追加） |
| 最小化時の見た目 | A: タイトル行のみ残し、中身とヘッダー右側のアクションも非表示 |
| 未保存時のデフォルト | A: すべて展開 |
| 実装アプローチ | 案 1: 共通ラッパーコンポーネント `CollapsibleCard.svelte` を新設 |

## 3. データ構造と永続化

### 3-1. Go 側 `Config` の拡張（`app.go:227`）

```go
type Config struct {
    SongdataDBPath string                        `json:"songdataDBPath"`
    FileLog        bool                          `json:"fileLog"`
    ColumnWidths   map[string]map[string]float64 `json:"columnWidths,omitempty"`
    CardCollapsed  map[string]map[string]bool    `json:"cardCollapsed,omitempty"`
}
```

- 外側マップキー: `paneId`（`"song"` / `"chart"` / `"entry"`）
- 内側マップキー: `cardId`（`"chartInfo"` / `"irInfo"` / `"bmsSearch"` / `"installCandidate"`）
- 値: `true` = 最小化、`false` または欠如 = 展開

### 3-2. 未保存時の解釈

- ペインキー欠如 → そのペインの全カードは展開状態
- カードキー欠如 → そのカードは展開状態
- **デフォルト = 展開**

### 3-3. 値が `false` の扱い

`false` 値は冗長（欠如と同等）なので、保存時に値が `false` になったキーは**削除**する。同様に、内側マップが空になったらペインキー自体を削除する。`omitempty` により外側マップが空なら JSON 出力に含まれない。

### 3-4. `config.json` の例

```json
{
  "songdataDBPath": "...",
  "columnWidths": { "...": { "...": 0.5 } },
  "cardCollapsed": {
    "song": { "irInfo": true },
    "entry": { "installCandidate": true, "bmsSearch": true }
  }
}
```

### 3-5. マイグレーション

既存 `config.json` には `cardCollapsed` キーが存在しないが、`omitempty` + ゼロ値処理により問題なし。読み込み時にキー欠如 → `nil` map → 全展開、で動作する。

## 4. コンポーネント API

### 4-1. 新規: `frontend/src/components/CollapsibleCard.svelte`

```svelte
<script lang="ts">
  import { cardCollapsed, toggleCard } from '../stores/cardCollapsed'
  import Icon from './Icon.svelte'

  export let paneId: 'song' | 'chart' | 'entry'
  export let cardId: 'chartInfo' | 'irInfo' | 'bmsSearch' | 'installCandidate'

  $: collapsed = $cardCollapsed[paneId]?.[cardId] === true
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between" class:mb-2={!collapsed}>
    <div class="flex items-center gap-1 min-w-0">
      <button
        class="btn btn-ghost btn-xs btn-square"
        on:click={() => toggleCard(paneId, cardId)}
        title={collapsed ? '展開' : '最小化'}
      >
        <Icon name={collapsed ? 'chevronRight' : 'chevronDown'} cls="h-4 w-4" />
      </button>
      <h3 class="text-sm font-semibold truncate">
        <slot name="title" />
      </h3>
    </div>
    {#if !collapsed}
      <slot name="actions" />
    {/if}
  </div>
  {#if !collapsed}
    <slot />
  {/if}
</div>
```

ポイント:

- `slot name="title"` — 単純なテキストだけでなく、`IRInfoCard` の `<a>` タグや `BMSSearchInfoCard` の警告アイコン入りタイトルにも対応
- `slot name="actions"` — `IRInfoCard` の「IR取得」ボタン等、ヘッダー右側のアクション類
- デフォルトスロット — カード本体
- 最小化時: `actions` と本体スロットを描画しない。`mb-2` も外してタイトル行が下マージン無しで完結
- 外枠 `bg-base-200 rounded-lg p-3` を `CollapsibleCard` 側に移管 → 既存 4 種カードからは外枠を削除

### 4-2. 既存カードの改修

各カードの外枠 `<div class="bg-base-200 rounded-lg p-3">` と内部の `<h3>` / タイトル行を削り、`<CollapsibleCard>` でラップする。

例: `ChartInfoCard.svelte`

```svelte
<script lang="ts">
  import type { dto } from '../../wailsjs/go/models'
  import { modeLabel, diffLabel } from '../utils/chartLabels'
  import CollapsibleCard from './CollapsibleCard.svelte'

  export let chart: dto.ChartDTO
  export let paneId: 'song' | 'chart' | 'entry'
</script>

<CollapsibleCard {paneId} cardId="chartInfo">
  <span slot="title">譜面情報</span>
  <div class="text-xs space-y-1">
    <!-- 既存の中身そのまま -->
  </div>
</CollapsibleCard>
```

各カードに `paneId: 'song' | 'chart' | 'entry'` の props を追加し、呼び出し側（`SongDetail` 等）で渡す。`IRInfoCard` の「IR取得」ボタンや `BMSSearchInfoCard` の `lookup`/`unlink` ボタンは `slot="actions"` に配置する。

### 4-3. アイコン追加（`frontend/src/components/icons.ts`）

`chevronRight` / `chevronDown` を Heroicons から追加。具体的なパスデータは実装時に Heroicons から正確に転記。

### 4-4. ストア: `frontend/src/stores/cardCollapsed.ts`

```ts
import { writable } from 'svelte/store'
import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App'

type PaneId = 'song' | 'chart' | 'entry'
type CardId = 'chartInfo' | 'irInfo' | 'bmsSearch' | 'installCandidate'
type CollapsedMap = Partial<Record<PaneId, Partial<Record<CardId, boolean>>>>

export const cardCollapsed = writable<CollapsedMap>({})

let initialized = false

export async function initCardCollapsed(): Promise<void> {
  if (initialized) return
  initialized = true
  const cfg = await GetConfig()
  cardCollapsed.set((cfg.cardCollapsed as CollapsedMap) ?? {})
}

export async function toggleCard(paneId: PaneId, cardId: CardId): Promise<void> {
  let next: CollapsedMap = {}
  cardCollapsed.update(curr => {
    const pane = { ...(curr[paneId] ?? {}) }
    if (pane[cardId]) {
      delete pane[cardId]
    } else {
      pane[cardId] = true
    }
    const updated = { ...curr }
    if (Object.keys(pane).length === 0) {
      delete updated[paneId]
    } else {
      updated[paneId] = pane
    }
    next = updated
    return updated
  })
  const cfg = await GetConfig()
  await SaveConfig({ ...cfg, cardCollapsed: next })
}
```

ポイント:

- 値が `false` になるケースは「キーごと削除」。`config.json` に `false` を書かない方針
- `App.svelte` の起動時に `initCardCollapsed()` を呼んで初期同期
- `toggleCard` 呼び出しごとに `SaveConfig`（既存 `columnResize.ts` と同じ即時保存）

## 5. データフロー

### 5-1. 起動時の初期化

```
App.svelte onMount
   ├─ rewriteRules ストア初期化（既存）
   └─ initCardCollapsed()
         └─ GetConfig() → cardCollapsed ストア set()
```

`App.svelte` の `onMount` 内で既存の初期化処理に並べて `initCardCollapsed()` を呼ぶ。fire-and-forget でよい（理由は §6-a）。

### 5-2. トグル時のフロー

```
ユーザーがカード左上のボタンをクリック
   ↓
CollapsibleCard が toggleCard(paneId, cardId)
   ↓
cardCollapsed ストアを update（即時 UI 反映）
   ↓
GetConfig() で最新 Config 取得 → SaveConfig() で永続化
```

UI 更新を先に行い、永続化は非同期。`columnResize.ts` の `saveColumnWidths` と同じパターン。

### 5-3. 呼び出し側の改修例（`SongDetail.svelte`）

```svelte
<ChartInfoCard chart={selectedChart} paneId="song" />
<IRInfoCard md5={selectedChart.md5} ir={selectedChart} paneId="song" on:lookup={...} />
<BMSSearchInfoCard info={bmsSearchInfo} loading={bmsSearchLoading} paneId="song" ... />
```

`ChartDetail` は `paneId="chart"`、`EntryDetail` は `paneId="entry"`。

## 6. エッジケース

### a. 初期化前にカードが描画されるケース

`cardCollapsed` ストアの初期値 `{}` でも全展開動作になるので、初期化遅延中に一瞬展開状態で出ても致命的ではない（`columnWidths` も同様の挙動）。

**方針**: 初期値 `{}` で問題なし。`App.svelte` で fire-and-forget で `initCardCollapsed()` を呼ぶ。

### b. 連続トグルによる SaveConfig レース

`toggleCard` が連続で呼ばれた場合、各呼び出しが `GetConfig` → `SaveConfig` するため、後勝ちで上書きされる可能性。ただし「同じ `cardCollapsed` フィールドを連続で書く」だけなら、各 `toggleCard` 内で最新の store 値を使って `SaveConfig` するため、シリアライズすれば最終状態は一致する。

**方針**: 既存 `columnResize.ts` と同じく即時保存。連続クリック時は呼び出し順に await する形でも実質問題なし。必要ならデバウンスを後で追加可能。

### c. `config.json` の手書き編集による不正値

`cardCollapsed.song.unknownCard: true` のような未定義 cardId が来ても、`CollapsibleCard` は対応する `cardId` をクエリしないため害なし（デッドエントリとして無視）。`paneId` 未定義値も同様。

**方針**: 不正値は無視。クリーンアップは行わない（`columnWidths` のキー集合チェックのような厳密検証は不要 — boolean フラグは単純で破損リスクが低い）。

### d. 最小化中にカードが props 変更で再マウント

ストア駆動のため再マウント後も `$cardCollapsed[paneId]?.[cardId]` を参照して同じ状態に復元される。問題なし。

### e. `IRInfoCard` の「IR取得」ボタンが最小化時に隠れる

最小化時はタイトル行のみ表示する仕様（要件 Q4 = A）。アクションを使いたいなら一度展開する必要がある。これは仕様。

## 7. テスト観点（実装計画段階で詳細化）

- 各カードを最小化 → 別タブに切り替え → 戻る → 最小化状態が維持される
- 各カードを最小化 → アプリ再起動 → 最小化状態が `config.json` から復元される
- 同じカード種別が `SongDetail` と `ChartDetail` で**独立に**最小化される（ペイン × カード種別の独立性検証）
- `EntryDetail` の `InstallCandidateCard` 最小化が他ペインに影響しない
- `config.json` を手で編集して未知の paneId/cardId を入れてもアプリが起動・動作する

## 8. 影響範囲（変更ファイル）

| ファイル | 変更内容 |
|---|---|
| `app.go` | `Config` に `CardCollapsed` フィールド追加 |
| `frontend/src/components/CollapsibleCard.svelte` | 新規 |
| `frontend/src/stores/cardCollapsed.ts` | 新規 |
| `frontend/src/components/icons.ts` | `chevronRight` / `chevronDown` 追加 |
| `frontend/src/components/ChartInfoCard.svelte` | 外枠 / タイトル削除 + `CollapsibleCard` ラップ + `paneId` props |
| `frontend/src/components/IRInfoCard.svelte` | 同上（`actions` slot に「IR取得」） |
| `frontend/src/components/BMSSearchInfoCard.svelte` | 同上（`actions` slot に lookup/unlink） |
| `frontend/src/components/InstallCandidateCard.svelte` | 同上 |
| `frontend/src/views/SongDetail.svelte` | 各カードに `paneId="song"` を渡す |
| `frontend/src/views/ChartDetail.svelte` | `paneId="chart"` |
| `frontend/src/views/EntryDetail.svelte` | `paneId="entry"` |
| `frontend/src/App.svelte` | `initCardCollapsed()` を `onMount` で呼び出し |
| `wailsjs/go/models.ts` 等の生成物 | `wails generate module` で再生成 |

## 9. 完了条件

- 上記 4 種のカードが `paneId × cardId` で独立に最小化・展開できる
- 開閉状態が `config.json` の `cardCollapsed` に永続化される
- アプリ再起動後も状態が復元される
- 最小化ボタンが各カードのタイトル左に表示される
- 未保存時はすべて展開状態で表示される
