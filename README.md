# BMS ELSA

**Efficient Library & Storage Agent** — BMSファイルの整理・導入・検証を支援するデスクトップアプリケーション。

**[ユーザーマニュアル](docs/manual.md)** — インストール・設定・機能説明はこちら

**[ダウンロード](https://github.com/meta-BE/bms-elsa/releases/latest)** — 最新版のZIPファイル

## 機能

- 5タブ構成：楽曲一覧・譜面一覧・難易度表・重複検知・差分導入
- beatoraja の `songdata.db` を読み込み、楽曲・譜面データを表示
- 仮想スクロールによる大量データの高速表示（2,600曲以上対応）
- 各タブにインクリメンタル検索機能
- カラムヘッダークリックによるソート（難易度表はレベル数値順ソート対応）
- 左右分割レイアウト（ドラッグリサイズ対応）で選択項目の詳細を表示
- LR2IR からのメタデータ取得（個別・一括取得対応、進捗表示・中断対応）
- 難易度表からの未導入譜面IR一括取得
- BMS Search API連携によるイベント情報自動取得（楽曲→イベント紐付けの一括同期、3並列・進捗表示・中断対応）
- イベントマスター管理（393件のイベントデータ同梱、BMS Searchからの更新、短縮名編集）
- 楽曲詳細からBMS Search・イベント本家ページへの直接リンク
- イベント名・リリース年の手動設定（オートコンプリート付きイベント選択）
- 動作URLの編集・保存
- BMS難易度表の取り込み・管理（Stella, 発狂BMS, Solomon等に対応、ドラッグ&ドロップで並び替え可能、個別更新・一括並列更新対応）
- 難易度表の未導入譜面でもIR情報を表示
- 譜面詳細に難易度ラベルをバッジ表示
- 上下キーによるキーボードナビゲーション（楽曲・譜面・難易度表・重複検知の各タブ対応）
- GUIからの設定編集（songdata.dbパス、難易度表の追加・削除・更新・並び替え、イベントマスター管理）
- 難易度表の並列一括更新（最大5並列、進捗表示、キャンセル対応）と個別更新ボタン
- URL書き換えルール（replace/regex対応、優先度付きルール適用で動作URLを自動推定）
- 重複検知（MD5完全一致検出 + WAV定義MinHash類似度・メタデータファジーマッチングの2段階検出、専用タブで一覧・詳細表示）
- フォルダマージ（重複検知から選択した2フォルダのファイルを1フォルダに統合、競合時は作成日時で判定、移動元は自動削除）
- フォルダ移動（楽曲詳細から移動先ディレクトリを選択してフォルダごと移動、同一FS時はrename・クロスFS時はコピー＋削除、移動済み行の黄色表示）
- カラム幅リサイズ（全テーブルでヘッダードラッグによるカラム幅変更、設定を自動保存・復元、ウィンドウリサイズ時の比例再計算、設定画面からテーブル別リセット）
- 差分導入（BMS/BME/BMLファイルをD&Dで導入先自動推定、WAV定義MinHash・IR・タイトル一致のスコアリング統合）
- 起動時バックグラウンドタスク自動実行（MinHashスキャン・重複検知スキャン・難易度表一括更新・動作URL推定を自動実行、設定画面で進捗・結果確認）
- BMSパーサー + MinHash計算（WAV定義からMinHash署名を計算・保存、譜面の音声類似度マッチングに使用）
- フォルダを開く（楽曲詳細・譜面詳細・難易度表エントリ・重複詳細からインストール先フォルダをファイルマネージャーで表示）
- カラムフィルタ（EVENT/YEAR/STATUSカラムでドロップダウンフィルタリング）
- パス検索（楽曲一覧・譜面一覧でフォルダパスによる検索、トグルで切り替え）
- IR情報プリフェッチ済み elsa.db 同梱（初回起動時から約33万譜面分のIR情報を利用可能）
- 詳細ビューからLR2IRページへの直接リンク
- 外部リンクのシステムブラウザ表示（macOS/Windows/Linux対応）
- LR2IR備考の改行表示・URL自動リンク化
- カスタムコンテキストメニュー（カット/コピー/ペースト/削除 + リンク上で「開く」「URLをコピー」対応）

## セットアップ

`songdata.db` のパスはアプリの設定画面（歯車アイコン）から設定できる。
設定は実行ファイルと同じディレクトリの `config.json` に保存される。

未設定の場合は `~/.beatoraja/songdata.db` → `~/beatoraja/songdata.db` の順で自動検出する。

## 技術スタック

| レイヤー | 技術 |
|---|---|
| バックエンド | Go 1.24 + Wails v2 |
| フロントエンド | Svelte 4 + TypeScript + Vite 5 |
| UI | TailwindCSS + DaisyUI 5 |
| 永続化 | SQLite（`modernc.org/sqlite` — 純Go実装、CGO不要） |
| テーブル表示 | TanStack Table + TanStack Virtual |
| DnD | svelte-dnd-action |

## 前提条件

- Go 1.24+
- Node.js 16+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 開発

```bash
# 開発サーバー起動（HMR + DevTools有効）
wails dev

# プロダクションビルド
wails build

# DevTools付きプロダクションビルド
wails build -devtools
```

ビルド成果物は `build/bin/` に出力される。

## ディレクトリ構成

```
bms-elsa/
├── main.go                     # Wailsエントリポイント
├── app.go                      # App構造体 + 設定・難易度表API
├── internal/
│   ├── domain/model/           # エンティティ・値オブジェクト
│   ├── usecase/                # ユースケース層
│   ├── adapter/
│   │   ├── gateway/            # LR2IRクライアント・BMS Searchクライアント・難易度表フェッチャー
│   │   └── persistence/        # SQLiteリポジトリ（elsa.db + songdata.db）
│   └── app/                    # Wailsバインディング層（ハンドラー + DTO）
├── frontend/                   # Svelte + TypeScript
│   └── src/
│       ├── components/         # 共有UIコンポーネント
│       ├── views/              # タブ画面・詳細パネル
│       ├── settings/           # 設定系モーダル
│       └── utils/              # ユーティリティ関数
├── cmd/                        # 開発用CLIコマンド
│   ├── prefetch-ir/            # LR2IR情報の事前取得
│   ├── merge-db/               # elsa.db統合ツール
│   └── gen-testdata/           # テスト用songdata.db生成
├── build/                      # Wailsビルド設定 + elsa.db
├── testdata/                   # テスト用データ（songdata.db等）
└── docs/                       # ドキュメント
    ├── TODO.md                 # 開発タスク一覧
    └── superpowers/            # 設計・実装計画ドキュメント
        ├── specs/              # 設計ドキュメント
        └── plans/              # 実装計画
```

## 注意事項

### songdata.db への書き込み

本アプリは起動時に beatoraja の `songdata.db` に対してインデックス（`idx_song_folder`）を作成する。これは楽曲一覧の表示速度を実用的な水準にするために必要な処理であり、テーブルのデータ自体には一切変更を加えない。インデックスは `CREATE INDEX IF NOT EXISTS` で冪等に作成されるため、既に存在する場合は何も行わない。beatoraja の動作に影響はないが、`songdata.db` のファイルサイズがインデックス分だけ増加する。

## 設計ドキュメント

- [アーキテクチャ設計](docs/architecture.md)
- [BMSドメイン知識・モチベーション](docs/bms-domain-and-motivation.md)
- [フロントエンド技術調査](docs/frontend-research.md)
- [Wails + Go 設計引き継ぎ](docs/wails_go_design_handoff.md)
- [BMS難易度表フォーマット](docs/bms-difficulty-table-format.md)
- [BMS Search API仕様](docs/bmssearch-api.md)
- [LR2IRレスポンス構造](docs/lr2ir-structure.md)
