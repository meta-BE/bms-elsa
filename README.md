# BMS ELSA

**Efficient Library & Storage Agent** — BMSファイルの整理・導入・検証を支援するデスクトップアプリケーション。

**[ユーザーマニュアル](docs/manual.md)** — インストール・設定・機能説明はこちら

**[ダウンロード](https://github.com/meta-BE/bms-elsa/releases/latest)** — 最新版のZIPファイル

## 機能

- 4タブ構成：楽曲一覧・譜面一覧・難易度表・重複検知
- beatoraja の `songdata.db` を読み込み、楽曲・譜面データを表示
- 仮想スクロールによる大量データの高速表示（2,600曲以上対応）
- 各タブにインクリメンタル検索機能
- カラムヘッダークリックによるソート（難易度表はレベル数値順ソート対応）
- 左右分割レイアウト（ドラッグリサイズ対応）で選択項目の詳細を表示
- LR2IR からのメタデータ取得（個別・一括取得対応、進捗表示・中断対応）
- 難易度表からの未導入譜面IR一括取得
- 楽曲メタデータ推測（URLパターンマッチングによるイベント名・リリース年の自動設定 + 手動確認フロー）
- Event名・リリース年・動作URLの編集・保存
- BMS難易度表の取り込み・管理（Stella, 発狂BMS, Solomon等に対応）
- 難易度表の未導入譜面でもIR情報を表示
- 譜面詳細に難易度ラベルをバッジ表示
- GUIからの設定編集（songdata.dbパス、難易度表の追加・削除・更新、URLパターンマッピング管理）
- URL書き換えルール（replace/regex対応、優先度付きルール適用で動作URLを自動推定）
- 重複検知（タイトル・アーティスト類似度によるファジーマッチング、専用タブで一覧・詳細表示）
- カラムフィルタ（EVENT/YEAR/STATUSカラムでドロップダウンフィルタリング）
- 詳細ビューからLR2IRページへの直接リンク
- 外部リンクのシステムブラウザ表示（macOS/Windows/Linux対応）

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
│   │   ├── gateway/            # LR2IRクライアント・難易度表フェッチャー
│   │   └── persistence/        # SQLiteリポジトリ（elsa.db + songdata.db）
│   └── app/                    # Wailsバインディング層（ハンドラー + DTO）
├── frontend/                   # Svelte + TypeScript
│   └── src/
│       ├── components/         # 共有UIコンポーネント
│       ├── views/              # タブ画面・詳細パネル
│       ├── settings/           # 設定系モーダル
│       └── utils/              # ユーティリティ関数
├── build/                      # Wailsビルド設定
├── testdata/                   # テスト用データ（songdata.db等）
└── docs/                       # ドキュメント
    ├── TODO.md                 # 開発タスク一覧
    └── plans/                  # 設計・実装計画ドキュメント
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
