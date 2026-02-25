# BMS ELSA

**Efficient Library & Storage Agent** — BMSファイルの整理・導入・検証を支援するデスクトップアプリケーション。

## 技術スタック

| レイヤー | 技術 |
|---|---|
| バックエンド | Go + Wails v2 |
| フロントエンド | Svelte 4 + TypeScript + Vite 5 |
| 永続化 | SQLite（`modernc.org/sqlite` — 純Go実装、CGO不要） |
| テーブル表示 | TanStack Table + TanStack Virtual |

## 前提条件

- Go 1.23+
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
├── main.go                     # Wailsエントリポイント + DI組み立て
├── app.go                      # App構造体（将来ハンドラーに分割）
├── internal/
│   ├── domain/                 # ドメイン層（最内層・外部依存なし）
│   │   ├── model/              # エンティティ・値オブジェクト
│   │   ├── repository/         # リポジトリインターフェース
│   │   └── service/            # ドメインサービス
│   ├── usecase/                # ユースケース層
│   ├── port/                   # ポート定義（外部I/Oの抽象）
│   ├── adapter/                # アダプタ層（ポート・リポジトリの実装）
│   │   ├── parser/             # BMSパーサー
│   │   ├── gateway/            # LR2IRクライアント
│   │   ├── filesystem/         # ファイル操作・MD5計算
│   │   └── persistence/        # SQLiteリポジトリ
│   └── app/                    # Wailsバインディング層
│       ├── dto/                # フロントエンド向けDTO
│       └── event/              # Wailsイベント実装
├── frontend/                   # Svelte + TypeScript
├── build/                      # Wailsビルド設定
└── docs/                       # 設計ドキュメント
```

## 設計ドキュメント

- [アーキテクチャ設計](docs/architecture.md)
- [BMSドメイン知識・モチベーション](docs/bms-domain-and-motivation.md)
- [フロントエンド技術調査](docs/frontend-research.md)
- [Wails + Go 設計引き継ぎ](docs/wails_go_design_handoff.md)
