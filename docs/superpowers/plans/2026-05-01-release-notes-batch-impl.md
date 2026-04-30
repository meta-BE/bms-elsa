# 過去リリース本文一括整備 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** GitHub Releases v0.0.1〜v0.10.1 (全25個) のリリース本文を、サブエージェント並列ディスパッチで生成し `gh release edit` で一括適用する。

**Architecture:** 25リリースを 3個ずつ 9バッチに分割し、サブエージェント並列ディスパッチ (同時実行2まで、計5サイクル) で `docs/release-notes/<タグ>.md` を生成。ユーザーレビュー後、`scripts/apply-release-notes.sh` で一括反映。

**Tech Stack:** bash, gh CLI, git, Agent tool (サブエージェントディスパッチ)

**Spec:** `docs/superpowers/specs/2026-05-01-release-notes-batch-design.md`

---

## File Structure

| パス | 役割 | 種別 |
|---|---|---|
| `scripts/apply-release-notes.sh` | リリース本文一括適用スクリプト (gh release edit) | 新規 |
| `docs/release-notes/.gitkeep` | ディレクトリ確保用 | 新規 |
| `docs/release-notes/v0.0.1.md` 〜 `v0.10.1.md` | リリース本文ドラフト (25ファイル) | サブエージェント生成 |

---

## Common: サブエージェントへのプロンプトテンプレート

各サイクルで使う共通テンプレート。`<TAGS>` 部分のみ各エージェントで差し替える。

```
あなたは bms-elsa リポジトリの過去リリースノートを生成するサブエージェントです。

担当リリース: <TAGS>
作業ディレクトリ: /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa

各リリースについて以下を実行してください:

1. 1個前のタグから対象タグまでの git log を取得
   - 例: git log v0.0.1..v0.0.2 --no-merges --pretty=format:"%h %s%n%b%n---"
   - 最初のリリース v0.0.1 のみ、ルートから v0.0.1 までを対象
     (例: git log v0.0.1 --no-merges --pretty=format:"%h %s%n%b%n---")

2. コミットメッセージから「新機能 (feat)」と「バグ修正 (fix)」を抽出
   - プレフィックスがない古いコミットも、内容を読んで feat/fix に分類
   - refactor / docs / chore / test は除外
   - より正確な機能把握のため、各コミットの変更ファイルも確認:
     - git log <range> --name-only --pretty=format:"%h %s" で変更ファイル一覧取得
     - docs/ 配下に追加・更新された Markdown (例: docs/superpowers/specs/*.md,
       docs/*-design.md, docs/*.md) があれば head -50 で冒頭の「目的・概要」を読む
     - 内部実装ではなく「機能としての意図」を把握する

3. コミット粒度ではなく「機能」粒度に集約
   - 関連する複数コミットは1項目にまとめる
   - ユーザーから見た機能として表現
   - spec ドキュメント冒頭の「目的・背景」を踏まえると正確になる

4. docs/release-notes/<タグ>.md にMarkdownで保存
   - 見出しは ## 新機能 / ## バグ修正 のみ
   - 空セクションは省略
   - H1 は付けない
   - 言語は日本語

出力フォーマット例:

## 新機能
- 機能Aの説明
- 機能Bの説明

## バグ修正
- 修正内容Aの説明

報告: 生成したファイルパス一覧のみを返してください。
```

---

### Task 1: 適用スクリプト作成

**Files:**
- Create: `scripts/apply-release-notes.sh`

- [ ] **Step 1: scripts/ ディレクトリ作成**

Run: `mkdir -p scripts`
Expected: エラーなし

- [ ] **Step 2: スクリプトファイル作成**

ファイル `scripts/apply-release-notes.sh` を以下の内容で新規作成:

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -d docs/release-notes ]; then
  echo "ERROR: docs/release-notes/ ディレクトリが存在しません" >&2
  exit 1
fi

shopt -s nullglob
files=(docs/release-notes/v*.md)
if [ ${#files[@]} -eq 0 ]; then
  echo "ERROR: docs/release-notes/v*.md にファイルがありません" >&2
  exit 1
fi

echo "適用対象: ${#files[@]} 件"
for f in "${files[@]}"; do
  tag="$(basename "$f" .md)"
  echo "  - $tag"
done

read -r -p "実行しますか？ [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "キャンセルしました。"
  exit 0
fi

for f in "${files[@]}"; do
  tag="$(basename "$f" .md)"
  echo "Updating release: $tag"
  gh release edit "$tag" --notes-file "$f"
done

echo "完了"
```

- [ ] **Step 3: 実行権限付与**

Run: `chmod +x scripts/apply-release-notes.sh`
Expected: エラーなし

- [ ] **Step 4: シンタックスチェック**

Run: `bash -n scripts/apply-release-notes.sh`
Expected: 無出力で exit 0

- [ ] **Step 5: 異常系の動作確認 (docs/release-notes/ 未作成のため ERROR となること)**

Run: `scripts/apply-release-notes.sh`
Expected: stderr に "ERROR: docs/release-notes/ ディレクトリが存在しません" が出力され exit 1

- [ ] **Step 6: コミット**

```bash
git add scripts/apply-release-notes.sh
git commit -m "feat: リリース本文一括適用スクリプト追加

- docs/release-notes/v*.md を gh release edit --notes-file で一括反映
- 実行前に対象タグを表示して y/N 確認"
```

---

### Task 2: docs/release-notes/ ディレクトリ確保

**Files:**
- Create: `docs/release-notes/.gitkeep`

- [ ] **Step 1: ディレクトリと .gitkeep 作成**

Run:
```bash
mkdir -p docs/release-notes
touch docs/release-notes/.gitkeep
```
Expected: エラーなし

- [ ] **Step 2: 適用スクリプトの正常パス動作確認 (ディレクトリは存在するがファイルなし → ERROR)**

Run: `scripts/apply-release-notes.sh`
Expected: stderr に "ERROR: docs/release-notes/v*.md にファイルがありません" が出力され exit 1

- [ ] **Step 3: コミット**

```bash
git add docs/release-notes/.gitkeep
git commit -m "chore: docs/release-notes/ ディレクトリ追加"
```

---

### Task 3: サイクル1 - エージェント A + B を並列ディスパッチ

担当:
- A: v0.0.1, v0.0.2, v0.0.3
- B: v0.0.4, v0.1.0, v0.2.0

**Files:**
- Create (by agents): `docs/release-notes/v0.0.1.md`, `v0.0.2.md`, `v0.0.3.md`, `v0.0.4.md`, `v0.1.0.md`, `v0.2.0.md`

- [ ] **Step 1: エージェント A と B を 1メッセージ内で並列ディスパッチ**

Common テンプレートの `<TAGS>` を以下に差し替えた2つのプロンプトを Agent tool で同時呼び出し:
- A: `v0.0.1, v0.0.2, v0.0.3`
- B: `v0.0.4, v0.1.0, v0.2.0`

`subagent_type` は `general-purpose` を指定。両方の完了を待ってから次ステップへ。

- [ ] **Step 2: 6ファイル生成確認**

Run: `ls docs/release-notes/v0.0.1.md docs/release-notes/v0.0.2.md docs/release-notes/v0.0.3.md docs/release-notes/v0.0.4.md docs/release-notes/v0.1.0.md docs/release-notes/v0.2.0.md`
Expected: 6ファイル全部存在 (No such file エラーなし)

- [ ] **Step 3: フォーマット目視確認**

Run: `for f in docs/release-notes/v0.0.1.md docs/release-notes/v0.0.2.md docs/release-notes/v0.0.3.md docs/release-notes/v0.0.4.md docs/release-notes/v0.1.0.md docs/release-notes/v0.2.0.md; do echo "=== $f ==="; cat "$f"; echo; done`
Expected: 各ファイルが `## 新機能` または `## バグ修正` を含む / H1なし / 日本語

- [ ] **Step 4: コミット**

```bash
git add docs/release-notes/v0.0.1.md docs/release-notes/v0.0.2.md docs/release-notes/v0.0.3.md docs/release-notes/v0.0.4.md docs/release-notes/v0.1.0.md docs/release-notes/v0.2.0.md
git commit -m "docs: リリースノートドラフト生成 (v0.0.1〜v0.2.0)"
```

---

### Task 4: サイクル2 - エージェント C + D を並列ディスパッチ

担当:
- C: v0.2.1, v0.3.0, v0.3.1
- D: v0.3.2, v0.3.3, v0.4.0

**Files:**
- Create (by agents): `docs/release-notes/v0.2.1.md`, `v0.3.0.md`, `v0.3.1.md`, `v0.3.2.md`, `v0.3.3.md`, `v0.4.0.md`

- [ ] **Step 1: エージェント C と D を 1メッセージ内で並列ディスパッチ**

Common テンプレートの `<TAGS>` を以下に差し替えた2つのプロンプトを Agent tool で同時呼び出し:
- C: `v0.2.1, v0.3.0, v0.3.1`
- D: `v0.3.2, v0.3.3, v0.4.0`

`subagent_type` は `general-purpose`。両方完了を待ってから次ステップへ。

- [ ] **Step 2: 6ファイル生成確認**

Run: `ls docs/release-notes/v0.2.1.md docs/release-notes/v0.3.0.md docs/release-notes/v0.3.1.md docs/release-notes/v0.3.2.md docs/release-notes/v0.3.3.md docs/release-notes/v0.4.0.md`
Expected: 6ファイル全部存在

- [ ] **Step 3: フォーマット目視確認**

Run: `for f in docs/release-notes/v0.2.1.md docs/release-notes/v0.3.0.md docs/release-notes/v0.3.1.md docs/release-notes/v0.3.2.md docs/release-notes/v0.3.3.md docs/release-notes/v0.4.0.md; do echo "=== $f ==="; cat "$f"; echo; done`
Expected: 各ファイルが `## 新機能` または `## バグ修正` を含む / H1なし / 日本語

- [ ] **Step 4: コミット**

```bash
git add docs/release-notes/v0.2.1.md docs/release-notes/v0.3.0.md docs/release-notes/v0.3.1.md docs/release-notes/v0.3.2.md docs/release-notes/v0.3.3.md docs/release-notes/v0.4.0.md
git commit -m "docs: リリースノートドラフト生成 (v0.2.1〜v0.4.0)"
```

---

### Task 5: サイクル3 - エージェント E + F を並列ディスパッチ

担当:
- E: v0.5.0, v0.5.1, v0.6.0
- F: v0.6.1, v0.7.0, v0.7.1

**Files:**
- Create (by agents): `docs/release-notes/v0.5.0.md`, `v0.5.1.md`, `v0.6.0.md`, `v0.6.1.md`, `v0.7.0.md`, `v0.7.1.md`

- [ ] **Step 1: エージェント E と F を 1メッセージ内で並列ディスパッチ**

Common テンプレートの `<TAGS>` を以下に差し替えた2つのプロンプトを Agent tool で同時呼び出し:
- E: `v0.5.0, v0.5.1, v0.6.0`
- F: `v0.6.1, v0.7.0, v0.7.1`

`subagent_type` は `general-purpose`。両方完了を待ってから次ステップへ。

- [ ] **Step 2: 6ファイル生成確認**

Run: `ls docs/release-notes/v0.5.0.md docs/release-notes/v0.5.1.md docs/release-notes/v0.6.0.md docs/release-notes/v0.6.1.md docs/release-notes/v0.7.0.md docs/release-notes/v0.7.1.md`
Expected: 6ファイル全部存在

- [ ] **Step 3: フォーマット目視確認**

Run: `for f in docs/release-notes/v0.5.0.md docs/release-notes/v0.5.1.md docs/release-notes/v0.6.0.md docs/release-notes/v0.6.1.md docs/release-notes/v0.7.0.md docs/release-notes/v0.7.1.md; do echo "=== $f ==="; cat "$f"; echo; done`
Expected: 各ファイルが `## 新機能` または `## バグ修正` を含む / H1なし / 日本語

- [ ] **Step 4: コミット**

```bash
git add docs/release-notes/v0.5.0.md docs/release-notes/v0.5.1.md docs/release-notes/v0.6.0.md docs/release-notes/v0.6.1.md docs/release-notes/v0.7.0.md docs/release-notes/v0.7.1.md
git commit -m "docs: リリースノートドラフト生成 (v0.5.0〜v0.7.1)"
```

---

### Task 6: サイクル4 - エージェント G + H を並列ディスパッチ

担当:
- G: v0.8.0, v0.8.1, v0.9.0
- H: v0.9.1, v0.9.2, v0.10.0

**Files:**
- Create (by agents): `docs/release-notes/v0.8.0.md`, `v0.8.1.md`, `v0.9.0.md`, `v0.9.1.md`, `v0.9.2.md`, `v0.10.0.md`

- [ ] **Step 1: エージェント G と H を 1メッセージ内で並列ディスパッチ**

Common テンプレートの `<TAGS>` を以下に差し替えた2つのプロンプトを Agent tool で同時呼び出し:
- G: `v0.8.0, v0.8.1, v0.9.0`
- H: `v0.9.1, v0.9.2, v0.10.0`

`subagent_type` は `general-purpose`。両方完了を待ってから次ステップへ。

- [ ] **Step 2: 6ファイル生成確認**

Run: `ls docs/release-notes/v0.8.0.md docs/release-notes/v0.8.1.md docs/release-notes/v0.9.0.md docs/release-notes/v0.9.1.md docs/release-notes/v0.9.2.md docs/release-notes/v0.10.0.md`
Expected: 6ファイル全部存在

- [ ] **Step 3: フォーマット目視確認**

Run: `for f in docs/release-notes/v0.8.0.md docs/release-notes/v0.8.1.md docs/release-notes/v0.9.0.md docs/release-notes/v0.9.1.md docs/release-notes/v0.9.2.md docs/release-notes/v0.10.0.md; do echo "=== $f ==="; cat "$f"; echo; done`
Expected: 各ファイルが `## 新機能` または `## バグ修正` を含む / H1なし / 日本語

- [ ] **Step 4: コミット**

```bash
git add docs/release-notes/v0.8.0.md docs/release-notes/v0.8.1.md docs/release-notes/v0.9.0.md docs/release-notes/v0.9.1.md docs/release-notes/v0.9.2.md docs/release-notes/v0.10.0.md
git commit -m "docs: リリースノートドラフト生成 (v0.8.0〜v0.10.0)"
```

---

### Task 7: サイクル5 - エージェント I を単独ディスパッチ

担当:
- I: v0.10.1

**Files:**
- Create (by agent): `docs/release-notes/v0.10.1.md`

- [ ] **Step 1: エージェント I を単独ディスパッチ**

Common テンプレートの `<TAGS>` を `v0.10.1` に差し替えたプロンプトを Agent tool で 1個呼び出し。
`subagent_type` は `general-purpose`。

- [ ] **Step 2: ファイル生成確認**

Run: `ls docs/release-notes/v0.10.1.md`
Expected: ファイル存在

- [ ] **Step 3: フォーマット目視確認**

Run: `cat docs/release-notes/v0.10.1.md`
Expected: `## 新機能` または `## バグ修正` を含む / H1なし / 日本語

- [ ] **Step 4: 全25ファイル揃ったことを確認**

Run: `ls docs/release-notes/v*.md | wc -l`
Expected: `25`

- [ ] **Step 5: コミット**

```bash
git add docs/release-notes/v0.10.1.md
git commit -m "docs: リリースノートドラフト生成 (v0.10.1)"
```

---

### Task 8: ユーザーレビュー (チェックポイント)

**Files:**
- Modify (by user): `docs/release-notes/v*.md` (任意)

- [ ] **Step 1: ユーザーへのレビュー依頼**

ユーザーに以下を依頼:
- `docs/release-notes/v*.md` (25ファイル) を確認
- 不適切な表現・粒度・抜け漏れがあれば直接編集
- 編集後にOKを返す

実装エージェントは **ユーザーから明示的に「OK」「適用して」等の許可が出るまで Task 9 に進まない**。

- [ ] **Step 2: ユーザー編集分があればコミット**

Run: `git status docs/release-notes/`

ユーザーが手で編集していた場合のみ:
```bash
git add docs/release-notes/
git commit -m "docs: リリースノートドラフト ユーザーレビュー反映"
```

変更がなければスキップ。

---

### Task 9: 適用スクリプト実行 (gh release edit 一括)

**Files:** なし (リモートGitHub Releases更新のみ)

- [ ] **Step 1: 事前確認 - gh CLI 認証状態**

Run: `gh auth status`
Expected: ログイン済み (`Logged in to github.com as ...`)

未ログインの場合、ユーザーに `gh auth login` を促してから次へ。

- [ ] **Step 2: 適用スクリプト実行**

Run: `scripts/apply-release-notes.sh`

確認プロンプトで `y` を入力。
Expected:
- `適用対象: 25 件` 表示
- 25個の `Updating release: vX.Y.Z` 行
- 末尾に `完了`
- exit 0

- [ ] **Step 3: 抜き打ちで2〜3個のリリースを GitHub 上で確認**

Run: `gh release view v0.10.1 --json body,tagName | head -20`
Expected: `body` フィールドに生成した内容が反映されている (空文字列ではない)

別の例:
Run: `gh release view v0.5.0 --json body,tagName | head -20`
Expected: 同上

- [ ] **Step 4: 完了報告**

ユーザーに「全25リリースの本文反映が完了しました」と報告。
最終的にこのフィーチャーブランチを main へマージするかは別判断 (PR作成・直接マージはユーザー指示待ち)。

---

## Self-Review

**1. Spec coverage:**
- 全25リリースの3個ずつ9バッチ分割 → Tasks 3〜7 でカバー
- 同時実行2並列・5サイクル → Tasks 3〜7 のサイクル構成
- `docs/release-notes/<タグ>.md` 出力 → Tasks 3〜7 の Step 1
- ユーザーレビュー → Task 8
- `scripts/apply-release-notes.sh` で一括適用 → Task 1, Task 9
- 冪等 (再実行で同結果) → スクリプトは `gh release edit --notes-file` の単純上書き

**2. Placeholder scan:**
- TBD/TODO/「適切な」等の曖昧表現なし
- 各 Step に具体的なコマンド・コードあり

**3. Type consistency:**
- ファイルパス・タグ名・コミットメッセージ規則は全タスクで一貫
- サブエージェントの `subagent_type` は全サイクル `general-purpose` で統一
