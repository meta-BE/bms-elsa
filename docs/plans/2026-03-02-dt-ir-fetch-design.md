# 難易度表IR一括取得 + chart_meta PK変更 デザイン

## 目的

難易度表ビューから、選択中のテーブルに含まれる全エントリ（未導入含む）のLR2IR情報を一括取得できるようにする。
これに伴い、chart_metaテーブルのPKを`(md5, sha256)`から`md5`のみに変更する。

## 背景

- ChartListViewには既にIR一括取得機能がある（`BulkFetchIRUseCase`）
- 現状は`songdata.song`に存在する譜面のみが対象（md5+sha256が必要）
- 難易度表には未導入の譜面も含まれるが、LR2IRはmd5のみでルックアップ可能
- `chart_meta`の`UNIQUE(md5, sha256)`制約が、未導入譜面（sha256不明）の保存を阻む

## アプローチ

**BulkFetchIRUseCaseをmd5リスト受け取り型に汎用化する。**

- chart_metaのPKを`md5`のみに変更
- BulkFetchIRUseCase.Executeがmd5リストを引数で受け取る形に変更
- 呼び出し元（IRHandler）が用途に応じてmd5リストを構築

## 変更内容

### 1. スキーマ変更（chart_meta）

マイグレーションで既存テーブルを新スキーマに移行:

```sql
-- 新テーブル作成
CREATE TABLE chart_meta_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    md5              TEXT NOT NULL UNIQUE,
    sha256           TEXT NOT NULL DEFAULT '',
    lr2ir_tags       TEXT,
    lr2ir_body_url   TEXT,
    lr2ir_diff_url   TEXT,
    lr2ir_notes      TEXT,
    lr2ir_fetched_at TEXT,
    working_body_url TEXT,
    working_diff_url TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

-- データ移行（md5重複時は最新を採用）
INSERT INTO chart_meta_new (md5, sha256, lr2ir_tags, ...)
  SELECT md5, sha256, lr2ir_tags, ...
  FROM chart_meta
  GROUP BY md5
  HAVING id = MAX(id);

-- 旧テーブル削除・リネーム
DROP TABLE chart_meta;
ALTER TABLE chart_meta_new RENAME TO chart_meta;

-- インデックス再作成（sha256は不要、md5はUNIQUE制約でカバー）
```

### 2. Repository層

| メソッド | 変更 |
|---------|------|
| `GetChartMeta(ctx, md5)` | sha256パラメータ削除 |
| `UpsertChartMeta(ctx, meta)` | `ON CONFLICT(md5)` に変更 |
| `BulkUpsertChartMeta(ctx, metas)` | 同上 |
| `UpdateWorkingURLs(ctx, md5, ...)` | sha256パラメータ削除 |
| `ListUnfetchedChartMD5s(ctx)` | 旧`ListUnfetchedChartKeys`をリネーム。`[]string`を返す |
| `ListUnfetchedDTEntryMD5s(ctx, tableID)` | **新規**。指定テーブルのmd5で未取得のものを返す |

### 3. Domain Model

- `ChartKey`型を削除（md5だけで十分）
- `MetaRepository`インターフェースを更新

### 4. Usecase層

`BulkFetchIRUseCase.Execute(ctx, md5s []string, progressFn)` に変更:
- 引数でmd5リストを受け取る
- 内部でリポジトリを呼ばない

### 5. IRHandler

```
StartBulkFetch()  // 既存（ChartListView用）
  → metaRepo.ListUnfetchedChartMD5s()
  → bulkFetchIR.Execute(ctx, md5s, progressFn)

StartDifficultyTableBulkFetch(tableID int)  // 新規（難易度表用）
  → dtRepo.ListUnfetchedDTEntryMD5s(tableID)
  → bulkFetchIR.Execute(ctx, md5s, progressFn)
```

排他制御は既存のmutexを共有。イベント名も同じ`ir:progress`/`ir:done`。

### 6. フロントエンド

**DifficultyTableView.svelte:**
- ChartListView.svelteと同じパターンでIR取得UIを追加
- ヘッダーバーに「IR取得」ボタン / 進捗表示 / 完了メッセージ
- `StartDifficultyTableBulkFetch(tableID)` を呼び出し
- `ir:progress` / `ir:done` イベントをリッスン

難易度表ビューへのIRカラム追加はなし。

### 7. 影響を受ける既存コード

chart_metaのsha256をPKから外すため、sha256を参照している箇所すべてを修正:
- `elsa_repository.go` — 上記Repository層の全メソッド
- `songdata_reader.go` — `GetChartByMD5`がchart_metaをJOINしている箇所
- `bulk_fetch_ir.go` — ChartKey → md5文字列に変更
- `bulk_fetch_ir_test.go` — テスト更新
- `usecase_test.go` — mockのインターフェース更新
- `ir_handler.go` — StartBulkFetchの呼び出し方法変更
- `app.go` — DI部分の調整
