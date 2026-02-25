# フロントエンド技術調査

## 前提

- Wails v2のWebViewベース。フロントエンドはViteでビルドし `embed.FS` でGoバイナリに埋め込まれる
- バンドルサイズ最小化が最重要
- 数千〜数万行のテーブル表示（フィルタリング・ソート付き）が主要な要件

## テーブル: TanStack Table + TanStack Virtual

| 項目 | 値 |
|---|---|
| バンドルサイズ | Table: ~10-15KB, Virtual: ~10-15KB（gzip後） |
| 機能 | フィルタリング（グローバル/カラム/ファジー）、マルチカラムソート内蔵 |
| 仮想スクロール | 50,000行でも60FPS |
| 設計 | ヘッドレスUI。マークアップ・スタイルは自由 |
| 対応FW | React, Vue, Solid, Svelte |

ヘッドレスなのでUIライブラリ側のテーブルコンポーネントは不要。

## フレームワーク選定

### 候補比較

| FW | gzipサイズ | TanStack対応 | Wailsテンプレート |
|---|---|---|---|
| Svelte 5 | ~2-3 KB | `@tanstack/svelte-table` | 公式あり |
| Svelte 4 | ~2-3 KB | `@tanstack/svelte-table` | 公式あり |
| Preact | ~4 KB | 非公式（React互換で動く可能性） | 公式あり |
| Solid.js | ~6-7 KB | `@tanstack/solid-table` | なし |
| Vue 3 | ~34 KB | `@tanstack/vue-table` | 公式あり |
| React | ~45 KB | `@tanstack/react-table` | 公式あり |

### 選定: Svelte

- バンドルサイズ最小（コンパイル型でランタイムがほぼゼロ）
- TanStack Table公式対応
- Wails公式テンプレートあり
- 使用経験あり
- Svelte 4を選択肢に含める（安定性重視の場合）

## UIライブラリ

### TanStack Tableとの組み合わせで必要なもの

テーブル本体はTanStack Tableが担うため、UIライブラリからは以下が必要:
- ボタン、入力フォーム、セレクト、ダイアログ等の基本コンポーネント
- レイアウト（サイドバー、ヘッダー等）

### 候補

| ライブラリ | 方式 | Svelte 4/5 | 特徴 |
|---|---|---|---|
| shadcn-svelte | CLIでソースコード生成 | 両対応 | Bits UI + Tailwind。必要なコンポーネントだけ生成。未使用分のバンドル影響ゼロ |
| DaisyUI | Tailwindプラグイン | FW無関係 | Tailwindクラスだけで完結。最軽量。コンポーネントロジックは自前 |
| Skeleton UI | プリビルトデザインシステム | v2=Svelte4, v3=Svelte5 | 完成度高い。テーマ付き。カスタマイズに制限あり |
| Melt UI + Tailwind | ヘッドレスUI | 両対応 | 最も自由度が高い。学習コストやや高い |

### 推奨構成

**TanStack Table + shadcn-svelte + Tailwind CSS**

- shadcn-svelteは必要なコンポーネントだけCLIで生成し、ソースコードが自分のプロジェクトに入る
- DaisyUIはさらに軽量な代替（Tailwindクラスのみでコンポーネントロジックは自前）

## Wailsフロントエンドビルドの仕組み

- ViteでビルドしたフロントエンドがGoの `embed.FS` でバイナリに埋め込まれる
- 開発時は `wails dev` でViteのHMRが使える
- `wails.json` でフロントエンドのビルドコマンドを設定

```json
{
  "frontend:dir": "frontend",
  "frontend:install": "npm install",
  "frontend:build": "npm run build"
}
```

## 参考

- TanStack Table: https://tanstack.com/table/latest
- TanStack Virtual: https://tanstack.com/virtual/latest
- shadcn-svelte: https://www.shadcn-svelte.com/
- Wails Templates: https://wails.io/docs/community/templates/
