# ユーザーマニュアル＆ZIP配布 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** ユーザー向け簡易マニュアル（docs/manual.md）を作成し、GitHub ActionsでWindows実行ファイルとともにZIPで配布する仕組みを構築する。

**Architecture:** docs/manual.md をMarkdownで管理し、CI上でsedによる簡易整形でreadme.txtに変換。bms-elsa.exe + readme.txt をZIP圧縮してGitHub Releaseにアップロード。CLAUDE.mdに機能変更時のマニュアル更新ルールを追記。

**Tech Stack:** GitHub Actions, sed, PowerShell (Compress-Archive)

---

### Task 1: マニュアル（docs/manual.md）作成

**Files:**
- Create: `docs/manual.md`

**参考ドキュメント:**
- `README.md` — 機能一覧、セットアップ手順
- `docs/plans/2026-03-07-user-manual-design.md` — マニュアル構成

**Step 1: docs/manual.md を作成**

以下の内容でマニュアルを作成する。Markdownで記述するが、sedで整形しやすいようシンプルな記法に留める（テーブル・リンク・コードブロックは使わない）。

```markdown
# BMS ELSA ユーザーマニュアル

## はじめに

BMS ELSA（Efficient Library & Storage Agent）は、beatorajaの楽曲データベース（songdata.db）を読み込み、楽曲・譜面データの整理・管理を支援するデスクトップアプリケーションです。

**前提条件:**
- beatoraja がインストールされていること
- songdata.db が存在すること（beatoraja を一度以上起動していれば自動生成されています）

## インストール

1. ダウンロードしたZIPファイルを任意のフォルダに展開してください
2. 展開先の bms-elsa.exe を実行してください

## 初回設定

初回起動時、BMS ELSAは以下の順序でsongdata.dbを自動検出します。

- ホームフォルダ/.beatoraja/songdata.db
- ホームフォルダ/beatoraja/songdata.db

自動検出できない場合は、画面上部の歯車アイコンから設定画面を開き、songdata.dbのパスを手動で指定してください。

## 機能説明

### 楽曲一覧

フォルダ単位で楽曲を表示します。検索バーでインクリメンタル検索が可能です。
楽曲を選択すると、右側の詳細パネルにメタデータ（タイトル・アーティスト・譜面数等）が表示されます。

### 譜面一覧

MD5単位で譜面を一覧表示します。各譜面のタイトル・アーティスト・難易度が確認できます。
カラムヘッダーをクリックするとソートできます。EVENT・YEAR・STATUSカラムにはフィルタ機能があります。
IR一括取得ボタンで、LR2IRからメタデータをまとめて取得できます。

### 難易度表

BMS難易度表（Stella、発狂BMS、Solomon等）を取り込んで表示します。
難易度表の追加・削除・更新は設定画面から行えます。
未導入の譜面についてもIR情報を表示できます。

### 重複検知

タイトル・アーティストの類似度に基づくファジーマッチングで、重複する楽曲を検出します。
重複グループごとにまとめて表示され、不要なファイルの整理に役立ちます。

## 免責事項

本ソフトウェアは現状のまま提供されます。
本ソフトウェアの利用に伴ういかなるトラブル、データの破損・消失についても、作者は一切の責任を負いません。ご利用は自己責任でお願いいたします。
```

**Step 2: コミット**

```bash
git add docs/manual.md
git commit -m "docs: ユーザー向け簡易マニュアルを追加"
```

---

### Task 2: GitHub ActionsでZIP配布を構成

**Files:**
- Modify: `.github/workflows/build-windows.yml`

**参考:**
- 現在のワークフローは `build/bin/bms-elsa.exe` を単体でReleaseにアップロードしている
- Windows環境（`windows-latest`）のため、PowerShellの `Compress-Archive` を使用
- sedはGit for Windowsに同梱されているため追加インストール不要

**Step 1: build-windows.yml を修正**

Buildステップの後に以下のステップを追加し、Uploadステップを修正する:

```yaml
      - name: Convert manual to readme.txt
        run: |
          sed -e 's/^## //' -e 's/^### //' -e 's/^# //' -e 's/\*\*\([^*]*\)\*\*/\1/g' -e 's/^- /・/' docs/manual.md > build/bin/readme.txt
        shell: bash

      - name: Create ZIP
        run: |
          Compress-Archive -Path build/bin/bms-elsa.exe, build/bin/readme.txt -DestinationPath build/bin/bms-elsa.zip

      - name: Upload to Release
        uses: softprops/action-gh-release@v2
        with:
          files: build/bin/bms-elsa.zip
```

**sed変換の説明:**
- `s/^## //` — h2見出しマーカー除去
- `s/^### //` — h3見出しマーカー除去
- `s/^# //` — h1見出しマーカー除去
- `s/\*\*\([^*]*\)\*\*/\1/g` — 太字マーカー除去（`**text**` → `text`）
- `s/^- /・/` — 箇条書きを全角ビュレットに変換

**Step 2: コミット**

```bash
git add .github/workflows/build-windows.yml
git commit -m "ci: Windows配布物をZIP化（exe + readme.txt）"
```

---

### Task 3: CLAUDE.mdにマニュアル更新ルールを追記

**Files:**
- Modify: `CLAUDE.md`

**Step 1: CLAUDE.md に追記**

`## フロントエンド` セクションの後に以下を追加:

```markdown
## マニュアル
- ユーザー向けマニュアルは `docs/manual.md` に記述する
- 機能追加・変更時は、該当するマニュアルのセクションも更新すること
```

**Step 2: コミット**

```bash
git add CLAUDE.md
git commit -m "docs: CLAUDE.mdにマニュアル更新ルールを追記"
```

---

### Task 4: ビルド確認と検証

**Step 1: sed変換のローカルテスト**

```bash
sed -e 's/^## //' -e 's/^### //' -e 's/^# //' -e 's/\*\*\([^*]*\)\*\*/\1/g' -e 's/^- /・/' docs/manual.md
```

期待: Markdownマーカーが除去され、プレーンテキストとして読みやすい出力になること。

**Step 2: Goビルドが既存テストを壊していないことを確認**

```bash
go test ./...
go build ./...
```

期待: 全テスト PASS、ビルド成功（ドキュメント・CI変更のみなので壊れないはず）
