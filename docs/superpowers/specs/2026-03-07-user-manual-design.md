# ユーザーマニュアル＆ZIP配布 設計

## Goal

身内・知人への配布用に、ユーザー向け簡易マニュアル（readme.txt）を作成し、Windows実行ファイルとともにZIPで配布する仕組みを構築する。

## 前提

- 配布対象: Windowsのみ
- 想定読者: 身内・知人（beatorajaユーザー）
- 既存CI: `.github/workflows/build-windows.yml`（タグpush時にbms-elsa.exeをGitHub Releaseにアップロード）

## マニュアル管理

### ソースファイル

`docs/manual.md`（Markdownで管理）

### 構成

1. はじめに — BMS ELSAの概要、前提条件（beatorajaインストール済み）
2. インストール — ZIPを任意フォルダに展開、bms-elsa.exeを実行
3. 初回設定 — songdata.dbの自動検出と手動指定
4. 機能説明 — 楽曲一覧・譜面一覧・難易度表・重複検知の各タブ
5. 免責事項 — ソフトウェア利用に伴うトラブル・データ破損について一切責任を負わない

### Markdown → txt変換

CI上でsedによる簡易整形:

- `# ` → 削除（見出しマーカー除去）
- `**テキスト**` → テキスト（太字マーカー除去）
- `- ` → ・（箇条書き変換）

文字コードはUTF-8のまま（変換不要）。

## CI変更

### 現在のフロー

ビルド → bms-elsa.exe をReleaseに単体アップロード

### 変更後のフロー

1. ビルド → bms-elsa.exe 生成
2. `docs/manual.md` をsedで整形 → `readme.txt` 生成
3. bms-elsa.exe + readme.txt を ZIP 圧縮
4. ZIPファイルをGitHub Releaseにアップロード（exe単体の代わり）

## CLAUDE.md追記

```markdown
## マニュアル
- ユーザー向けマニュアルは `docs/manual.md` に記述する
- 機能追加・変更時は、該当するマニュアルのセクションも更新すること
```
