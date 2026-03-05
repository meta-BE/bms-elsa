# BMS ELSA (Efficient Library & Storage Agent) アーキテクチャ設計

## 技術スタック

- バックエンド: Wails v2 + Go
- フロントエンド: Svelte 4 + TypeScript + Vite + DaisyUI/TailwindCSS
- 永続化: SQLite（`modernc.org/sqlite` — 純Go実装、CGO不要）
- DI: 手動（app.go Init()で組み立て）
- データソース: beatoraja の songdata.db（読み取り専用）+ ELSA独自の elsa.db

## ディレクトリ構造

```
bms-elsa/
├── main.go                              # Wailsエントリポイント + Bind定義
├── app.go                               # App構造体・設定管理・DI組み立て
├── internal/
│   ├── domain/                         # ドメイン層（最内層・外部依存なし）
│   │   ├── model/
│   │   │   ├── song.go                 # Song/Chart エンティティ、ChartIRMeta等
│   │   │   └── repository.go          # SongRepository/MetaRepository interface
│   │   └── similarity/
│   │       ├── similarity.go           # タイトル・アーティスト類似度判定
│   │       └── grouping.go            # 重複検知・グループ化ロジック
│   │
│   ├── usecase/                        # ユースケース層
│   │   ├── list_songs.go               # 一覧取得（ページング）
│   │   ├── get_song_detail.go          # 楽曲詳細取得
│   │   ├── bulk_fetch_ir.go            # LR2IR一括取得
│   │   ├── lookup_ir.go               # LR2IR照合（単一）
│   │   ├── infer_meta.go              # URLパターン→Event/Year推測
│   │   ├── infer_working_url.go       # URL書き換えルール適用
│   │   ├── update_song_meta.go        # 楽曲メタデータ更新
│   │   └── update_chart_meta.go       # 譜面メタデータ更新
│   │
│   ├── port/                           # ポート定義（usecase層が依存するインターフェース）
│   │   └── ir_client.go                # IRClient interface
│   │
│   ├── adapter/                        # アダプタ層（ポート・リポジトリの実装）
│   │   ├── gateway/
│   │   │   ├── lr2ir_client.go         # LR2IRスクレイピング実装
│   │   │   ├── lr2ir_parser.go         # HTML解析・メタデータ抽出
│   │   │   └── difficulty_table_fetcher.go # 難易度表フェッチ・解析
│   │   └── persistence/
│   │       ├── migrations.go           # SQLiteスキーマ定義・マイグレーション
│   │       ├── elsa_repository.go      # elsa.db操作（chart_meta, rewrite_rule等）
│   │       ├── songdata_reader.go      # beatoraja songdata.db読み取り
│   │       └── difficulty_table_repository.go # 難易度表テーブル操作
│   │
│   └── app/                            # Wailsバインディング層（最外層）
│       ├── song_handler.go             # 楽曲API（ListSongs, GetDetail等）
│       ├── chart_handler.go            # 譜面API（ListCharts, GetChartDetail等）
│       ├── ir_handler.go               # LR2IR API（BulkFetch, Lookup等）
│       ├── difficulty_table_handler.go # 難易度表API（CRUD, Refresh等）
│       ├── inference_handler.go        # メタデータ推測API
│       ├── rewrite_handler.go          # URL書き換えルールAPI
│       └── dto/
│           └── dto.go                  # フロントエンド向けDTO群
│
├── cmd/
│   └── gen-testdata/
│       └── main.go                     # テストデータ生成ツール
│
├── frontend/                           # Svelteフロントエンド
│   └── src/
│       ├── components/                 # 共有UIコンポーネント
│       ├── views/                      # タブ画面・詳細パネル
│       ├── settings/                   # 設定系モーダル
│       └── utils/                      # ユーティリティ関数
│
├── testdata/                           # テスト用songdata.db等
├── docs/                              # ドキュメント・計画書
├── go.mod
└── wails.json
```

## レイヤー構成と依存方向

依存は常に外側から内側への一方向。

```
app.go (DI組み立て・全層を参照)
  ┌──────────────────────────────────────┐
  │ app (Wailsバインディング)            │ → usecase, dto
  │   ┌──────────────────────────────┐   │
  │   │ usecase                      │   │ → domain, port
  │   │   ┌──────────────────────┐   │   │
  │   │   │ domain (最内層)      │   │   │ → 外部依存なし
  │   │   └──────────────────────┘   │   │
  │   └──────────────────────────────┘   │
  └──────────────────────────────────────┘
  adapter (port/repositoryの実装) → port, domain
```

依存性逆転の適用:
- `usecase` → `port`（interface） ← `adapter/gateway`（実装）
- `usecase` → `domain/model`（Repository interface） ← `adapter/persistence`（実装）

## 主要インターフェース

### ポート

| ポート | 責務 | 実装 |
|---|---|---|
| IRClient | MD5指定でLR2IRメタデータ取得（レートリミット対応） | adapter/gateway/lr2ir_client.go |

### ドメインリポジトリ（domain/model/repository.go）

| インターフェース | 責務 | 実装 |
|---|---|---|
| SongRepository | songdata.dbからの楽曲・譜面読み取り（読み取り専用） | adapter/persistence/songdata_reader.go |
| MetaRepository | elsa.dbのメタデータCRUD（chart_meta, rewrite_rule, event_mapping等） | adapter/persistence/elsa_repository.go |

### ドメインモデル

| モデル | 説明 |
|---|---|
| Song | 曲フォルダ（FolderHash, Charts[], 代表Title/Artist, ReleaseYear, EventName） |
| Chart | 譜面ファイル（MD5, SHA256, Title, Subtitle, Artist, SubArtist, Path, DifficultyLabels） |
| ChartIRMeta | LR2IR取得結果 + 動作URL（WorkingBodyURL, WorkingDiffURL） |
| EventMapping | URLパターン→イベント名・リリース年マッピング |
| RewriteRule | URL書き換えルール（replace/regex型、優先度付き） |

### ドメインサービス（domain/similarity/）

| サービス | 責務 |
|---|---|
| similarity.go | Levenshtein距離によるタイトル・アーティスト類似度判定 |
| grouping.go | 類似度に基づく重複グループ化・スコアリング |

## Wailsバインディング層

### ハンドラー → フロントエンドに公開するAPI

| ハンドラー | 主なメソッド | 説明 |
|---|---|---|
| SongHandler | ListSongs, GetSongDetail, UpdateSongMeta | 楽曲一覧・詳細・メタ更新 |
| ChartHandler | ListCharts, GetChartDetailByMD5, GetChartMetaByMD5 | 譜面一覧・詳細 |
| IRHandler | LookupByMD5, StartBulkFetch, StartDifficultyTableBulkFetch | LR2IR照合・一括取得 |
| DifficultyTableHandler | ListDifficultyTables, AddDifficultyTable, RefreshAll, ListEntries | 難易度表CRUD・更新 |
| InferenceHandler | InferMeta, ListEventMappings | メタデータ推測・マッピング管理 |
| RewriteHandler | InferWorkingURLs, ListRewriteRules | URL書き換えルール管理・一括適用 |
| App | GetConfig, SaveConfig, OpenFolder, OpenURL, ScanDuplicates | 設定・システム操作 |

### Wailsイベント

| イベント名 | データ | タイミング |
|---|---|---|
| `ir:progress` | {current, total} | IR一括取得の進捗更新 |
| `ir:done` | {total, fetched, notFound, failed, cancelled} | IR一括取得完了 |

## SQLiteスキーマ

### songdata.db（beatoraja管理、読み取り専用）

beatorajaが管理するsongdata.dbからsong・chartデータを読み取る。ELSAはこのDBを変更しない。

### elsa.db（ELSA専用）

| テーブル | 用途 |
|---|---|
| chart_meta | 譜面ごとのLR2IRメタ・動作URL |
| song_meta | 楽曲ごとのイベント名・リリース年 |
| url_rewrite_rule | URL書き換えルール（replace/regex、優先度付き） |
| event_mapping | URLパターン→イベント名・年マッピング |
| difficulty_table | 難易度表メタデータ（URL・更新日時） |
| difficulty_entry | 難易度表エントリ（レベル・曲情報） |

## 設計判断

| 項目 | 決定 | 理由 |
|---|---|---|
| 永続化 | SQLite（`modernc.org/sqlite`） | 純Go実装でCGO不要。2回目以降の起動が高速 |
| データソース | beatoraja songdata.db読み取り | 自前でBMSパース・フォルダ走査せず既存データを活用 |
| DI | 手動（app.go Init()） | 依存ゼロ。起動オーバーヘッドゼロ |
| LR2IRアクセス | HTMLスクレイピング | 公式APIなし。adapter層で閉じるので将来差し替え可 |
| 進捗通知 | Wailsイベント（push型） | ポーリング不要。リアルタイム更新 |
| 大量データ表示 | 仮想スクロール + フロントエンドフィルタ | JSON転送量とUI再描画コスト制御 |
| フロントエンド公開 | DTOのみ | ドメインモデルを隠蔽 |
