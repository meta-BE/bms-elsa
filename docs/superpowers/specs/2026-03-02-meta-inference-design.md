# 楽曲メタデータ推測機能 設計

## 目的

楽曲のReleaseYearとEventNameを、LR2IR本体URLのパターンマッチングで半自動的に設定する。

## スコープ

- メタ推測機能のみ。「IR一括取得」は別機能として後日設計。
- 推測の情報源はLR2IR本体URLのみ（フォルダパスは使用しない）。

## 前提

- IR情報（LR2IR本体URL等）は事前に取得済みであること
- 楽曲と譜面は1:N、譜面とIR情報も1:1の関係

## データモデル

### 新テーブル: event_mapping（elsa.db）

| カラム | 型 | 説明 |
|--------|-----|------|
| id | INTEGER PK AUTOINCREMENT | |
| url_pattern | TEXT NOT NULL | URL部分一致パターン |
| event_name | TEXT NOT NULL | イベント名 |
| release_year | INTEGER NOT NULL | 開催年 |

マッチングロジック: 曲の全譜面のIR本体URL（lr2ir_body_url）に対して、url_patternの部分一致を確認。いずれかの譜面がマッチすればその曲に適用。

マッピングデータは事前にLLMで主要イベント（BOF系等）のURL→名称・年を収集して投入。アプリUIから追加・編集・削除可能。

## UIフロー

### 起動

楽曲一覧タブのヘッダーバーに「メタ推測」ボタンを配置。

### フェーズ1（自動）

1. 未設定曲（ReleaseYear=NULL AND EventName=NULL）を取得
2. 各曲の全譜面のIR本体URLとevent_mappingテーブルを照合
3. マッチした曲を一括保存
4. サマリー表示:「X曲を自動設定 / Y曲が未マッチ（うちIR未取得Z曲）」

### フェーズ2（手動・対話的）

- 未マッチ曲を1曲ずつモーダルで表示
- 表示内容:
  - タイトル、アーティスト、ジャンル
  - 譜面のIR URL一覧
  - IR取得状況（N件中M件取得済み）
- 入力欄: event_name、release_year
- ボタン: 「保存して次へ」「スキップ」「終了」
- 進捗表示: 「3 / 45」

### マッピングテーブル管理UI

別画面またはモーダルでevent_mappingの一覧表示・追加・編集・削除。

## バックエンド

### 新テーブル

- `event_mapping`（マイグレーションに追加）

### 新API（Wailsバインディング）

- `RunAutoInference()` → フェーズ1実行。自動設定数・未マッチ曲リストを返却
- `GetUnmatchedSongs()` → フェーズ2用の未マッチ曲リスト（IR URL・取得状況付き）
- `ListEventMappings()` / `UpsertEventMapping()` / `DeleteEventMapping()` → マッピング管理CRUD

### 既存API流用

- `UpdateSongMeta(folderHash, releaseYear, eventName)` → フェーズ2の1曲保存に使用

## 自動承認の基準

- URLがevent_mappingテーブルにマッチ → ユーザー確認なしで保存
- マッチしない → フェーズ2で手動確認

## 関連する将来機能

- IR一括取得: 未取得の譜面のLR2IR情報を逐次取得する機能（別途設計）
