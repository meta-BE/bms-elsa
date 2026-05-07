# release-notes スキル 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** GitHub Releases の body にコミットログから生成したリリースノートを埋めるリポジトリスキル `release-notes` を実装する。

**Architecture:** リポジトリスキル単体（`SKILL.md` のみ）。LLM が `gh` / `git` コマンドを順に実行し、コミット分類と文章生成も LLM が担う。スクリプト化はしない（質問4で LLM 判断を採用したためデータ収集と判定のレイヤを分離する意義が薄い）。

**Tech Stack:** Claude Code Skill（Markdown frontmatter）、`gh` CLI、`git`、Bash。利用モデルは Sonnet 4.6 に固定（`model: claude-sonnet-4-6`）。

---

## File Structure

| パス | 種別 | 役割 |
|---|---|---|
| `.claude/skills/release-notes/SKILL.md` | 新規 | スキル本体（frontmatter + 実行手順） |
| `docs/release-notes/` | 削除 | 過去のリリースノート蓄積。スキル実装完了後にディレクトリごと削除 |

---

## Task 1: スキルディレクトリと SKILL.md を作成

**Files:**
- Create: `.claude/skills/release-notes/SKILL.md`

- [ ] **Step 1: ディレクトリを作成**

```bash
mkdir -p .claude/skills/release-notes
```

- [ ] **Step 2: SKILL.md を作成**

`.claude/skills/release-notes/SKILL.md` の内容（Write ツールで作成）：

````markdown
---
name: release-notes
description: |
  GitHub Releases の body にリリースノートを生成・更新する。
  「リリースノート作成して」「v0.11.2 のリリースノートを作って」
  「最新リリースのノートを更新」などの指示があった場合に使用。
model: claude-sonnet-4-6
allowed-tools:
  - Bash
  - Write
---

# リリースノート生成・更新ガイドライン

GitHub Releases の body に、コミットログから生成したリリースノート（`## 新機能` / `## バグ修正` / `## その他の変更`）を埋めるスキル。

## 引数

- 引数なし → 最新 GitHub Release を対象
- 引数あり（`v0.11.2` または `0.11.2`）→ 指定リリースを対象

## 実行手順

### 1. バージョン正規化

- 引数が `v` なし（例: `0.11.2`）の場合、`v` を付けて `v0.11.2` に正規化する
- 引数なしの場合、最新リリースを取得する：

```bash
gh release list --limit 1 --json tagName --jq '.[0].tagName'
```

### 2. 対象リリース存在確認

```bash
gh release view <tag>
```

リリースが存在しない場合は「リリース <tag> が見つかりません」と表示して停止する。

### 3. 前バージョン解決

GitHub Releases の `publishedAt` 順で1つ前のリリースを「前バージョン」とする：

```bash
gh release list --limit 100 --json tagName,publishedAt \
  --jq 'sort_by(.publishedAt)'
```

- 上記の配列で対象タグの直前のエントリを採用する
- 対象が最古リリースの場合、前バージョン = リポジトリ最初のコミット：

```bash
git rev-list --max-parents=0 HEAD | head -1
```

### 4. コミット取得

```bash
git log <prev>..<target> --no-merges \
  --pretty=format:'%H%x09%s%x09%b%x1e'
```

- `<target>` は対象タグ
- `<prev>` は前バージョンのタグ、または最古リリースのときは最初のコミットハッシュ
- merge コミットは除外
- レコード区切りは `\x1e`、フィールド区切りは `\t`（コミット本文の改行を保持）

### 5. コミット分類

取得したコミット一覧を読み、次の4カテゴリに振り分ける。

| 分類 | 内容 |
|---|---|
| 新機能 | ユーザーから見える新しい機能の追加 |
| バグ修正 | ユーザーから見える不具合の修正 |
| その他の変更 | ユーザーから見える挙動変更・機能削除・破壊的変更など |
| 除外 | ドキュメント・内部リファクタ・テスト・CI・依存更新など、ユーザーに見えない変更 |

#### 判定の指針

- prefix（`feat:`/`fix:`/`refactor:` など）はヒントとして参照、最終判断はメッセージ全文の内容で行う
- 迷った場合は「ユーザー視点で挙動が変わるか」を基準にする
- 全コミットが「除外」になった場合は「ユーザー向けの変更がありません」と表示して停止（空のリリースノートで上書きしない）

### 6. 文章生成

#### スタイル規約

- 各項目は完結した文（体言止め禁止、「〜した」「〜を追加」「〜を修正」のような述語で締める）
- 背景・対象・挙動を含めて1〜2文で説明
- コードシンボルはバッククォートで囲む（例: `config.json`）
- 同じ機能領域に関する複数コミットは1項目にまとめる（バグ修正の積み重ねなど）

#### 過去文体の参照

過去リリースの body をいくつか参照して文体を揃える：

```bash
gh release view <prev_tag> --json body --jq .body
```

#### 出力フォーマット

```markdown
## 新機能
- ...

## バグ修正
- ...

## その他の変更
- ...
```

- 順序: `## 新機能` → `## バグ修正` → `## その他の変更`
- 該当項目がないセクションは出力しない

### 7. 上書き確認

```bash
gh release view <tag> --json body --jq .body
```

- 既存 body が空文字列（または空白・改行のみ）の場合 → 無確認で進む
- 何か入っている場合：
  - 既存 body と生成した新しい body をユーザーに表示する
  - 「上書きしますか？」と問い、明示的な承認（「はい」「ok」など）を待つ
  - 拒否されたら停止する

### 8. リリース更新

承認後（または既存 body が空の場合）：

1. 生成テキストを一時ファイル `/tmp/release-notes-<tag>.md` に書き出す（`Write` ツール）
   - `/tmp/` への書き込みがブロックされた場合は `save-temp-file` スキルへ誘導する
2. リリース更新：

```bash
gh release edit <tag> --notes-file /tmp/release-notes-<tag>.md
```

3. 更新後、URL を取得してユーザーに通知する：

```bash
gh release view <tag> --json url --jq .url
```

## エラーハンドリング

| 事象 | 挙動 |
|---|---|
| `gh` 未認証 | `gh auth login` を促すメッセージを表示して停止 |
| 対象タグなし | エラーメッセージで停止 |
| 対象範囲にユーザー向けコミット 0 件 | 「ユーザー向けの変更がありません」と通知して停止（空更新しない） |
| `/tmp/` への書き込みブロック | `save-temp-file` スキルへ誘導 |
````

- [ ] **Step 3: 作成内容を確認**

```bash
ls -la .claude/skills/release-notes/
cat .claude/skills/release-notes/SKILL.md | head -20
```

期待値: ファイルが存在し、frontmatter が `---` で始まる。

- [ ] **Step 4: コミット**

```bash
git add .claude/skills/release-notes/SKILL.md
git commit -m "feat: release-notes スキルを追加

GitHub Releases の body にコミットログから生成したリリースノートを
埋めるリポジトリスキルを実装。"
```

---

## Task 2: 動作確認（v0.11.2 を対象にスモークテスト）

**Files:**
- Read only: GitHub Releases v0.11.2

> v0.11.2 は body が空。これを実際の対象にして上書き挙動と生成品質を確認する。

- [ ] **Step 1: 対象リリース現状を確認**

```bash
gh release view v0.11.2 --json body --jq .body
```

期待値: 空文字列。

- [ ] **Step 2: 新規 Claude Code セッションでスキルを呼び出し**

別ターミナルで `claude` を起動し、以下を入力：

```
/release-notes v0.11.2
```

または自然言語で：

```
release-notes スキルを使って v0.11.2 のリリースノートを作成して
```

- [ ] **Step 3: スキル実行ログを観察**

確認項目：
- `gh release list` で前バージョン v0.11.1 が解決されていること
- `git log v0.11.1..v0.11.2 --no-merges` が実行されていること
- コミットが `新機能` / `バグ修正` / `その他の変更` のいずれかに分類されていること
- 「除外」のみで停止していないこと（v0.11.1..v0.11.2 にはユーザー向けコミットが含まれるはず）
- 既存 body が空のため無確認で `gh release edit --notes-file` が実行されること
- 完了後 URL が表示されること

- [ ] **Step 4: 結果を確認**

```bash
gh release view v0.11.2 --json body --jq .body
```

期待値: `## 新機能` / `## バグ修正` / `## その他の変更` のいずれかのセクションが存在し、各項目が述語で締めくくられた完結した文になっている。

- [ ] **Step 5: 不具合があれば SKILL.md を修正**

問題例と対応：

| 問題 | 修正箇所 |
|---|---|
| 体言止めが混入 | スタイル規約の表現を強化 |
| `docs:` / `chore:` が「その他の変更」に混入 | 判定指針に除外例を追記 |
| 前バージョン解決に失敗 | `--jq` クエリを再点検 |
| `/tmp/` ブロックでエラー停止 | save-temp-file スキル誘導文言を改善 |

修正後は再度 v0.11.2 を対象にスキルを実行し、上書き確認プロンプトが出ることを確認する。

- [ ] **Step 6: 修正があった場合のみコミット**

```bash
git add .claude/skills/release-notes/SKILL.md
git commit -m "fix: release-notes スキルの<対象箇所>を修正"
```

修正がなければスキップ。

---

## Task 3: 過去のリリースノート保存ディレクトリを削除

**Files:**
- Delete: `docs/release-notes/`

> 設計ドキュメントの「後始末」セクションに記載した通り、スキル動作確認後に削除する。

- [ ] **Step 1: 削除対象を確認**

```bash
ls docs/release-notes/ | wc -l
```

期待値: 35 前後（v0.0.1 〜 v0.11.1 の md ファイル群）。

- [ ] **Step 2: 削除実行**

```bash
git rm -r docs/release-notes/
```

- [ ] **Step 3: マニュアル等からの参照がないことを確認**

```bash
git grep -n "docs/release-notes" -- ':!docs/superpowers'
```

期待値: ヒットなし（設計・計画ドキュメント以外には参照がない）。

ヒットした場合は該当箇所を更新してから次のステップへ進む。

- [ ] **Step 4: コミット**

```bash
git commit -m "chore: 過去のリリースノート蓄積ディレクトリを削除

release-notes スキルで GitHub Releases body を直接更新する運用に
切り替えたため、リポジトリ管理は不要となった。"
```

---

## Self-Review

- **Spec coverage**: 設計の各セクション（概要・バージョン解決・差分取得・分類・文章生成・上書き確認・リリース更新・エラーハンドリング・SKILL.md 構造・後始末）すべてに対応するタスクあり。
- **Placeholder scan**: TBD/TODO なし。各ステップに具体コマンドまたは具体内容を記載。
- **Type consistency**: コマンド・パラメータ表記は spec と一致（`gh release edit --notes-file`、`git log <prev>..<target> --no-merges`、`model: claude-sonnet-4-6` 等）。
