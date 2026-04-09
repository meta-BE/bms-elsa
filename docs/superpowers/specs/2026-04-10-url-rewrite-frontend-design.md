# URL書き換えのフロントエンド表示時適用化

## 概要

現在のURL書き換え機能（動作URL推定）は、バックグラウンドでリライトルールを適用し結果をDBの`working_body_url`/`working_diff_url`カラムに格納する方式。これを廃止し、フロントエンドの表示時にリライトルールを適用する方式に変更する。

### 動機

IRの備考欄にはURLが複数出現することがあり、どのURLを`workingBodyURL`/`workingDiffURL`に格納すべきか自動判定は不可能。フロントエンドで全URLにルールを適用して表示することで、ユーザー自身が適切なリンクを判断できるようにする。

## 設計

### データフロー

```
起動時 / ルール編集時
  ListRewriteRules() → rewriteRules ストア (writable<RewriteRule[]>)

表示時 (IRInfoCard)
  本体URL + $rewriteRules → applyRewriteRules() → リライト済みURL表示
  差分URL + $rewriteRules → applyRewriteRules() → リライト済みURL表示
  備考テキスト + $rewriteRules → linkify() 内でルール適用 → リンク化済みHTML
```

### フロントエンド新規ファイル

#### `frontend/src/stores/rewriteRules.ts`

Svelteのwritableストアでルール配列を保持。

```ts
import { writable } from 'svelte/store'

export type RewriteRule = {
  id: number
  ruleType: string  // "replace" | "regex"
  pattern: string
  replacement: string
  priority: number
}

export const rewriteRules = writable<RewriteRule[]>([])
```

#### `frontend/src/lib/urlRewrite.ts`

Go側`applyRewriteRules`と同等のロジック。ルール配列はpriority降順（ListRewriteRulesの返却順）で処理し、最初にマッチしたルールを適用。マッチなしの場合は元URLをそのまま返す（Go側は空文字を返していたが変更）。

```ts
export function applyRewriteRules(url: string, rules: RewriteRule[]): string {
  if (!url) return ''
  for (const rule of rules) {
    if (rule.ruleType === 'replace') {
      if (url.includes(rule.pattern)) {
        return url.replace(rule.pattern, rule.replacement)
      }
    } else if (rule.ruleType === 'regex') {
      const re = new RegExp(rule.pattern)
      if (re.test(url)) {
        return url.replace(re, rule.replacement)
      }
    }
  }
  return url
}
```

### IRInfoCard.svelte 変更

#### 削除対象
- props: `workingBodyUrl`, `workingDiffUrl`
- 状態: `editingWorkingUrl`, `editWorkingBodyUrl`, `editWorkingDiffUrl`, `lastMd5`
- イベント: `save`ディスパッチャー
- UI: divider以下の「動作URL」セクション全体（編集モード含む）

#### 変更対象
- `lr2irBodyUrl` / `lr2irDiffUrl` のリンク表示に`applyRewriteRules`を適用（hrefと表示テキスト両方を置き換え）
- `linkify`関数を改修:
  - URL検出正規表現を `https?:\/\/(?:(?!https?:\/\/)[^\s<])+` に変更（連結URL対応）
  - 各検出URLに`applyRewriteRules`を適用

#### 変更後の表示構成
- タグ
- 本体URL（リライト適用済み）
- 差分URL（リライト適用済み）
- 備考（内部URLもリライト適用済み）

### 親コンポーネント変更

`ChartDetail.svelte`, `SongDetail.svelte`, `EntryDetail.svelte` で以下を削除:
- `saveWorkingUrls`関数
- `on:save={saveWorkingUrls}` バインディング
- `UpdateChartMeta`のインポート（他で使われていなければ）

### ストア初期化

- `App.svelte` の `onMount` で `ListRewriteRules()` を呼び、結果を `rewriteRules` ストアにセット
- `RewriteRuleManager.svelte` でルール追加・削除後に `ListRewriteRules()` で再取得しストアを更新

### Settings.svelte 変更

- 「動作URL推定」セクション（進捗バー、状態表示、`rewrite:progress`/`rewrite:done`イベントリスナー）を削除

### Go側削除

| 対象 | 内容 |
|------|------|
| `internal/usecase/infer_working_url.go` | ファイル全体削除 |
| `internal/usecase/infer_working_url_test.go` | ファイル全体削除 |
| `internal/usecase/update_chart_meta.go` | ファイル全体削除 |
| `internal/app/rewrite_handler.go` | `StartInferWorkingURLs`メソッド削除 |
| `internal/app/ir_handler.go` | `UpdateChartMeta`メソッド削除、`updateChart`フィールド削除 |
| `internal/adapter/persistence/elsa_repository.go` | `UpdateWorkingURLs`、`ListChartsForWorkingURLInference` 削除 |
| `internal/domain/model/repository.go` | `UpdateWorkingURLs`、`ListChartsForWorkingURLInference` をインターフェースから削除 |
| `internal/domain/model/song.go` | `ChartIRMeta`から`WorkingBodyURL`/`WorkingDiffURL`削除 |
| `internal/app/dto/dto.go` | `ChartDTO`、`ChartIRMetaDTO`から`WorkingBodyURL`/`WorkingDiffURL`削除、変換ロジックも削除 |
| `app.go` | `a.RewriteHandler.StartInferWorkingURLs()` 呼び出し削除、`inferWorkingURLs`/`updateChartMeta` DI削除 |

### DBマイグレーション

`chart_meta`テーブルから`working_body_url`、`working_diff_url`カラムをDROPするマイグレーションを追加。

### テスト変更

- `internal/usecase/usecase_test.go`: `TestUpdateChartMeta`削除、モックの`updateWorkingURLsFunc`/`UpdateWorkingURLs`削除
- `internal/adapter/persistence/elsa_repository_test.go`: `TestUpdateWorkingURLs`削除
- `internal/usecase/infer_working_url_test.go`: ファイル全体削除（前述）
