# 設定画面

## 概要

config.jsonをGUIから編集できる設定画面を追加する。

## 要件

- 設定項目: songdataDBPath のみ（将来追加可能な構造にする）
- アクセス: ナビバーの歯車アイコンからモーダルを開く
- 入力: テキスト入力 + 参照ボタン（OSネイティブのファイル選択ダイアログ）
- 保存後: 「再起動してください」メッセージを表示（ホットリロードはしない）

## 設計

### バックエンド（app.go）

Appに3つのメソッドを追加し、App自体をWails Bindに追加する:

- `GetConfig() Config`: config.jsonを読んで返す
- `SaveConfig(cfg Config) error`: config.jsonに書き込む
- `SelectFile() string`: `runtime.OpenFileDialog`でファイル選択ダイアログを開く

### フロントエンド

**Settings.svelte（新規）**: DaisyUI `<dialog>` モーダル。songdataDBPathのテキスト入力 + 参照ボタン、保存/キャンセルボタン。

**App.svelte**: ナビバーに歯車アイコンボタンを追加。クリックでSettingsモーダルを開く。

### データフロー

```
歯車クリック → GetConfig() → モーダルに現在値を表示
参照ボタン  → SelectFile() → テキスト入力に反映
保存ボタン  → SaveConfig() → 「再起動してください」表示
```

### 採用しなかったアプローチ

- **ドロワー（サイドパネル）**: 設定項目1つに対して大げさ。将来項目が増えたら検討。
- **ホットリロード**: DETACH→再ATTACH→楽曲一覧再取得の実装コストに対し、頻繁に変更する設定ではないため不要。
