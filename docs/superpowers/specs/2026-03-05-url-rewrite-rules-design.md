# URL書き換えルール設計

## 概要

LR2IRから取得したURL（本体URL・差分URL）に対して書き換えルールを適用し、動作URL（working_body_url, working_diff_url）を自動推定する機能。加えて、動作URLの表示を「常に編集フォーム」から「デフォルトはリンク表示、編集ボタンでフォームに切り替え」に変更する。

## ユースケース

- 古いドメインや移転先のURLを自動変換（例: `old-host.com/bms` → `new-host.com/bms`）
- パスを含むプレフィックス置換（例: `aaa.com/ccc` → `bbb.com/ddd`）
- 正規表現による柔軟なURL変換（パス構造の変換など）

## データモデル

### テーブル: `url_rewrite_rule`

```sql
CREATE TABLE IF NOT EXISTS url_rewrite_rule (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_type TEXT NOT NULL CHECK(rule_type IN ('replace', 'regex')),
    pattern TEXT NOT NULL,
    replacement TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(rule_type, pattern)
);
```

### ルールタイプ

| タイプ | 動作 | 例 |
|---|---|---|
| `replace` | patternをreplacementに単純文字列置換（最初の1箇所） | `aaa.com/ccc` → `bbb.com/ddd` |
| `regex` | 正規表現マッチ＋置換 | `old-(\d+)\.com` → `new-$1.com` |

## ルール適用ロジック

```
ApplyRewriteRules(originalURL string, rules []RewriteRule) string:
  1. rules を priority 降順でソート
  2. 各ルールを順に試行:
     - replace タイプ: strings.Replace(url, pattern, replacement, 1) で置換。
       patternが元URLに含まれる場合のみマッチ扱い。
     - regex タイプ: regexp.ReplaceAllString(url, replacement)
       正規表現がマッチした場合のみマッチ扱い。
  3. 最初にマッチしたルールの結果を返す
  4. マッチなしの場合は空文字を返す（元URLをそのまま動作URLにはしない）
```

## バックエンドAPI

### RewriteHandler

| メソッド | 説明 |
|---|---|
| `ListRewriteRules()` | 全ルール一覧取得（priority降順） |
| `UpsertRewriteRule(id, ruleType, pattern, replacement, priority)` | ルール追加・更新 |
| `DeleteRewriteRule(id)` | ルール削除 |
| `InferWorkingURLs()` | 全譜面に対してルールを一括適用 |

### InferWorkingURLs の処理フロー

1. url_rewrite_rule を全件取得（priority降順）
2. chart_meta から working_body_url と working_diff_url が両方空の譜面を取得（lr2ir_body_url または lr2ir_diff_url が存在するもの）
3. 各譜面について:
   - lr2ir_body_url にルール適用 → working_body_url に書き込み
   - lr2ir_diff_url にルール適用 → working_diff_url に書き込み
   - マッチしなければスキップ
4. 結果を返す: `{ applied: N, skipped: M, total: T }`

### 上書きポリシー

手動で動作URLが設定済みの譜面はスキップする（未設定のもののみ対象）。

## フロントエンドUI

### 動作URLの表示モード切り替え

**適用コンポーネント:** SongDetail, ChartDetail, EntryDetail

- デフォルト: クリック可能なリンク表示 + 右に編集アイコン
- 編集アイコンクリック: テキスト入力欄に切り替え
- blurで保存し、リンク表示に戻る
- 未設定の場合: 「未設定」グレーテキスト + 編集アイコン

### 書き換えルール管理UI

イベントマッピング管理と同じ画面にセクションとして追加。

| 列 | 説明 |
|---|---|
| タイプ | `replace` / `regex` のセレクト |
| パターン | 置換元の文字列 or 正規表現 |
| 置換先 | 置換後の文字列 |
| 優先度 | 数値（大きいほど優先） |
| 操作 | 編集・削除ボタン |

### 「動作URL推定」ボタン

楽曲一覧・譜面一覧タブのツールバーに配置。「メタ推定」ボタンと同列。クリックで `InferWorkingURLs()` を呼び出し、結果（applied/skipped/total）を表示。

## 変更ファイル一覧

### バックエンド（新規）

- `internal/domain/model/rewrite_rule.go` — RewriteRule 構造体
- `internal/usecase/infer_working_url.go` — InferWorkingURLUseCase
- `internal/app/rewrite_handler.go` — RewriteHandler

### バックエンド（既存変更）

- `internal/adapter/persistence/migrations.go` — url_rewrite_rule テーブル追加
- `internal/adapter/persistence/elsa_repository.go` — CRUD + 未設定譜面クエリ
- `app.go` — RewriteHandler の初期化・Wailsバインド登録

### フロントエンド（既存変更）

- `frontend/src/SongDetail.svelte` — 動作URL表示モード切り替え
- `frontend/src/ChartDetail.svelte` — 同上
- `frontend/src/EntryDetail.svelte` — 同上
- イベントマッピング管理画面 — ルール管理セクション追加
- 楽曲一覧/譜面一覧ツールバー — 「動作URL推定」ボタン追加

### テスト

- `internal/usecase/infer_working_url_test.go` — ルール適用ロジックのユニットテスト
