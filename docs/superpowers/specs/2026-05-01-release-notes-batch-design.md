# 過去リリース本文一括整備 設計

## 背景・目的

bms-elsa の GitHub Releases (v0.0.1〜v0.10.1, 全25個) は、すべてリリース本文が空のまま公開されている。
GitHub UI 上で表示される文章はリリース本文ではなく、本文未設定時に自動表示されるタグコミット情報や Full Changelog リンクである。

このため、過去リリースに対して **実装された大まかな機能一覧** を本文として書き加える。
今回のスコープは過去分の整備のみとし、今後のリリース運用の自動化 (`make release-note`) は別タスク (`docs/TODO.md` 記載済み) とする。

## 全体像

1. 過去25リリースを 3個ずつ 9バッチに分割
2. 各バッチをサブエージェントが担当し、`docs/release-notes/<タグ>.md` を生成
3. 同時実行は2エージェントまで → 全5サイクル
4. 全ファイル生成後、ユーザーがレビュー・編集
5. `scripts/apply-release-notes.sh` で `gh release edit` を一括実行 (冪等)

```
[git log + 変更ファイル参照] → [サブエージェント x 9] → [docs/release-notes/v*.md]
                                                           ↓
                                                      [ユーザーレビュー]
                                                           ↓
                                              [scripts/apply-release-notes.sh]
                                                           ↓
                                                  [gh release edit (x25)]
```

## 担当範囲分割

| # | エージェント | 担当リリース |
|---|---|---|
| 1 | A | v0.0.1, v0.0.2, v0.0.3 |
| 2 | B | v0.0.4, v0.1.0, v0.2.0 |
| 3 | C | v0.2.1, v0.3.0, v0.3.1 |
| 4 | D | v0.3.2, v0.3.3, v0.4.0 |
| 5 | E | v0.5.0, v0.5.1, v0.6.0 |
| 6 | F | v0.6.1, v0.7.0, v0.7.1 |
| 7 | G | v0.8.0, v0.8.1, v0.9.0 |
| 8 | H | v0.9.1, v0.9.2, v0.10.0 |
| 9 | I | v0.10.1 |

## ディスパッチサイクル

| サイクル | 並列実行 |
|---|---|
| 1 | A + B |
| 2 | C + D |
| 3 | E + F |
| 4 | G + H |
| 5 | I (単独) |

各サイクルで2エージェントを並列ディスパッチし、両方の完了を待ってから次サイクルへ進む。

## エージェントへのプロンプト仕様

各エージェントへ渡す指示の骨子(共通テンプレート + 担当リリースの差し込み):

```
あなたは bms-elsa リポジトリの過去リリースノートを生成するサブエージェントです。

担当リリース: <タグ1>, <タグ2>, <タグ3>
作業ディレクトリ: /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa

各リリースについて以下を実行:

1. 1個前のタグから対象タグまでの git log を取得
   - 例: git log v0.0.1..v0.0.2 --no-merges --pretty=format:"%h %s%n%b%n---"
   - 最初のリリース v0.0.1 はルートから v0.0.1 までを対象
     (例: git log v0.0.1 --no-merges --pretty=format:"%h %s%n%b%n---")

2. コミットメッセージから「新機能 (feat)」と「バグ修正 (fix)」を抽出
   - プレフィックスがない古いコミットも、内容を読んで feat/fix に分類
   - refactor / docs / chore / test は除外
   - より正確な機能把握のため、各コミットの変更ファイルも確認する:
     - git log <range> --name-only --pretty=format:"%h %s" で変更ファイル一覧取得
     - docs/ 配下に追加・更新された Markdown (例: docs/superpowers/specs/*.md,
       docs/*-design.md, docs/*.md) があれば head -30〜50 で冒頭の「目的・概要」を読む
     - これにより内部実装ではなく「機能としての意図」を把握できる

3. コミット粒度ではなく「機能」粒度に集約
   - 関連する複数コミットは1項目にまとめる
   - 内部実装の細部ではなく、ユーザーから見た機能として表現
   - spec ドキュメント冒頭の「目的・背景」を踏まえると正確になる

4. docs/release-notes/<タグ>.md にMarkdownで保存

出力フォーマット (空セクションは省略):

## 新機能
- 機能Aの説明
- 機能Bの説明

## バグ修正
- 修正内容Aの説明

報告: 生成したファイルパス一覧のみを返す。
```

## 出力ファイル仕様

- パス: `docs/release-notes/<タグ>.md` (例: `docs/release-notes/v0.10.1.md`)
- フォーマット:
  - 見出しは `## 新機能` / `## バグ修正` の2種類のみ
  - 空セクションは省略
  - H1 は付けない (GitHub Release UI ではタイトルがバージョン名で表示されるため)
  - 言語は日本語

例 (`docs/release-notes/v0.10.1.md`):

```markdown
## 新機能
- BMS Search 楽曲情報表示・連携 (詳細画面に BMSSearchInfoCard を追加、md5ベースの取得・解除に対応)

## バグ修正
- 難易度表からの BMS Search 解除で所持譜面の song_meta が残るバグを修正
```

## 適用スクリプト

`scripts/apply-release-notes.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

for f in docs/release-notes/v*.md; do
  tag="$(basename "$f" .md)"
  echo "Updating release: $tag"
  gh release edit "$tag" --notes-file "$f"
done
```

- 実行権限を付与 (`chmod +x`)
- 冪等: `gh release edit --notes-file` は単純上書きのため、再実行で同じ結果
- 既存ファイルがあるリリースのみ対象 (glob で `v*.md` をなめる)

## スコープ外

- 今後のリリース運用の自動化 (`make release-note` の新設)
  - `docs/TODO.md` の「## 改善」セクションに追加済み
- リリース本文の英訳・多言語対応
- `bms-elsa.zip` 等の添付ファイルの差し替え
