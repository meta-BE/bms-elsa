# BMS ELSA (Efficient Library & Storage Agent) クリーンアーキテクチャ設計

## 技術スタック

- バックエンド: Wails v2 + Go
- フロントエンド: WebView（詳細は別途設計）
- 永続化: SQLite（`modernc.org/sqlite` — 純Go実装、CGO不要）
- DI: 手動（main.goで組み立て）

## ディレクトリ構造

```
bms-elsa/
├── cmd/app/
│   └── main.go                         # DI組み立て + Wails起動
├── internal/
│   ├── domain/                         # ドメイン層（最内層・外部依存なし）
│   │   ├── model/
│   │   │   ├── song.go                 # 曲フォルダ エンティティ
│   │   │   ├── chart.go                # 譜面ファイル エンティティ
│   │   │   ├── bms_header.go           # ヘッダ情報 Value Object
│   │   │   ├── bms_definition.go       # WAV/BMP定義 Value Object
│   │   │   ├── ir_metadata.go          # LR2IRメタデータ
│   │   │   └── validation_result.go    # 差分検証結果
│   │   ├── repository/
│   │   │   ├── song_repository.go      # SongRepository interface
│   │   │   └── chart_repository.go     # ChartRepository interface
│   │   └── service/
│   │       ├── chart_validator.go      # 差分正当性検証
│   │       └── metadata_matcher.go     # タイトル類似度判定
│   │
│   ├── usecase/                        # ユースケース層
│   │   ├── scan_songs.go               # フォルダ走査
│   │   ├── list_songs.go               # 一覧取得（ページング）
│   │   ├── import_song.go              # 楽曲導入
│   │   ├── import_chart.go             # 差分導入
│   │   ├── validate_chart.go           # 差分検証
│   │   ├── lookup_ir.go               # LR2IR照合
│   │   ├── rename_song.go              # リネーム
│   │   └── move_song.go               # 移動
│   │
│   ├── port/                           # ポート定義（usecase層が依存するインターフェース）
│   │   ├── filesystem.go               # FileSystem interface
│   │   ├── bms_parser.go               # BMSParser interface
│   │   ├── ir_client.go                # IRClient interface
│   │   ├── hasher.go                   # Hasher interface
│   │   └── event_emitter.go            # EventEmitter interface
│   │
│   ├── adapter/                        # アダプタ層（ポート・リポジトリの実装）
│   │   ├── parser/
│   │   │   └── bms_parser.go           # BMSパーサー実装
│   │   ├── gateway/
│   │   │   └── lr2ir_client.go         # LR2IRスクレイピング実装
│   │   ├── filesystem/
│   │   │   ├── scanner.go              # ディレクトリ走査
│   │   │   ├── file_ops.go             # ファイル移動・リネーム
│   │   │   └── hasher.go               # MD5計算
│   │   └── persistence/
│   │       ├── sqlite_repository.go    # SQLiteリポジトリ実装
│   │       └── migrations.go           # スキーマ定義・マイグレーション
│   │
│   └── app/                            # Wailsバインディング層（最外層）
│       ├── scan_handler.go             # 走査API
│       ├── song_handler.go             # 楽曲API
│       ├── chart_handler.go            # 譜面/差分API
│       ├── ir_handler.go               # LR2IR API
│       ├── dto/                        # フロントエンド向けDTO群
│       └── event/
│           └── wails_emitter.go        # Wailsイベント実装
│
├── frontend/                           # フロントエンド（別途設計）
├── go.mod
└── wails.json
```

## レイヤー構成と依存方向

依存は常に外側から内側への一方向。

```
cmd/app (DI組み立て・全層を参照)
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
- `usecase` → `port`（interface） ← `adapter`（実装）
- `usecase` → `domain/repository`（interface） ← `adapter/persistence`（実装）

## 主要インターフェース

### ポート

| ポート | 責務 | 実装 |
|---|---|---|
| FileSystem | フォルダ走査、ファイル存在確認、移動、リネーム、読み込み | adapter/filesystem |
| BMSParser | `[]byte` → BMSHeader + BMSDefinitions への変換（I/Oなし） | adapter/parser |
| IRClient | MD5指定でLR2IRメタデータ取得（レートリミット・バッチ対応） | adapter/gateway |
| Hasher | ファイルパス → MD5ハッシュ | adapter/filesystem |
| EventEmitter | `Emit(eventName, data)` でフロントエンドへプッシュ通知 | app/event |

### ドメインモデル

| モデル | 説明 |
|---|---|
| Song | 曲フォルダ（DirPath, Charts[], 代表Title/Artist） |
| Chart | 譜面ファイル（FilePath, BMSHeader, BMSDefinitions, MD5） |
| BMSHeader | #TITLE, #SUBTITLE, #ARTIST, #SUBARTIST, #GENRE, #BPM, #PLAYLEVEL, #DIFFICULTY, #RANK, #TOTAL等 |
| BMSDefinitions | WAV定義一覧 + BMP定義一覧 |
| IRMetadata | LR2IRから取得した情報（BMSID, Title, Artist, Tags, BodyURL等） |
| ValidationResult | 検証合否, メタデータ一致, 欠損ファイル一覧, 参照ファイル存在率 |

### ドメインサービス

| サービス | 責務 |
|---|---|
| ChartValidator | メタデータ一致判定 + 参照ファイル存在確認 + 総合判定 |
| MetadataMatcher | タイトルからサブタイトル除去、ベース部分の一致判定 |

## Wailsバインディング層

### ハンドラー → フロントエンドに公開するAPI

| ハンドラー | メソッド | 説明 |
|---|---|---|
| ScanHandler | StartScan(rootPath) | フォルダ走査開始（非同期、進捗はイベント通知） |
| ScanHandler | CancelScan() | 走査キャンセル |
| SongHandler | ListSongs(PageRequest) | ページング付き楽曲一覧 |
| SongHandler | ImportSong(sourcePath, targetDir) | 楽曲導入 |
| SongHandler | RenameSong(dirPath, newName) | リネーム |
| SongHandler | MoveSong(dirPath, targetDir) | 移動 |
| ChartHandler | ValidateChart(chartPath, targetSongDir) | 差分正当性検証 |
| ChartHandler | ImportChart(chartPath, targetSongDir) | 差分導入 |
| IRHandler | LookupByMD5(md5) | LR2IRメタデータ取得 |
| IRHandler | LookupByChart(chartPath) | ローカル譜面からLR2IR照合 |

### Wailsイベント

| イベント名 | データ | タイミング |
|---|---|---|
| `scan:progress` | ScanProgressDTO | 走査進捗更新 |
| `scan:complete` | nil | 走査完了 |
| `scan:error` | {message} | 走査エラー |

## SQLiteスキーマ

```sql
CREATE TABLE songs (
    dir_path TEXT PRIMARY KEY,
    dir_name TEXT NOT NULL,
    title    TEXT,
    artist   TEXT
);

CREATE TABLE charts (
    file_path  TEXT PRIMARY KEY,
    song_dir   TEXT NOT NULL REFERENCES songs(dir_path),
    file_name  TEXT NOT NULL,
    md5        TEXT NOT NULL,
    title      TEXT,
    subtitle   TEXT,
    artist     TEXT,
    subartist  TEXT,
    genre      TEXT,
    bpm        REAL,
    play_level INTEGER,
    difficulty INTEGER,
    rank       INTEGER,
    total      REAL,
    file_size  INTEGER
);

CREATE TABLE chart_refs (
    chart_path TEXT NOT NULL REFERENCES charts(file_path),
    ref_type   TEXT NOT NULL,  -- 'wav' or 'bmp'
    ref_index  TEXT NOT NULL,
    filename   TEXT NOT NULL,
    PRIMARY KEY (chart_path, ref_type, ref_index)
);

CREATE TABLE ir_cache (
    md5        TEXT PRIMARY KEY,
    bms_id     INTEGER,
    title      TEXT,
    artist     TEXT,
    genre      TEXT,
    tags       TEXT,  -- JSON配列
    body_url   TEXT,
    fetched_at DATETIME NOT NULL
);

CREATE INDEX idx_charts_song_dir ON charts(song_dir);
CREATE INDEX idx_charts_md5 ON charts(md5);
CREATE INDEX idx_songs_title ON songs(title);
```

- `ir_cache` でLR2IR取得結果をキャッシュし、再問い合わせを回避
- 走査時は既存レコードとファイルシステムの差分を検出し、差分のみ更新（増分走査）

## 設計判断

| 項目 | 決定 | 理由 |
|---|---|---|
| 永続化 | SQLite（`modernc.org/sqlite`） | 純Go実装でCGO不要。2回目以降の起動が高速 |
| DI | 手動（main.goで組み立て） | 依存ゼロ。起動オーバーヘッドゼロ |
| BMSパーサー入力 | `[]byte` | ファイルI/Oと分離。テスト容易 |
| LR2IRアクセス | HTMLスクレイピング | 公式APIなし。adapter層で閉じるので将来差し替え可 |
| 走査並列化 | goroutine worker pool | 数千〜数万フォルダに対応。contextキャンセル対応 |
| 進捗通知 | Wailsイベント（push型） | ポーリング不要。リアルタイム更新 |
| 大量データ表示 | ページングAPI（offset/limit） | JSON転送量とUI再描画コスト制御 |
| フロントエンド公開 | DTOのみ | ドメインモデルを隠蔽 |
