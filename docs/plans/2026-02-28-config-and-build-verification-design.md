# config.json によるパス設定とビルド動作確認

## 目的

`wails dev` / `wails build` でアプリを起動し、任意のsongdata.dbを読み込んで楽曲データの表示・操作を確認できるようにする。

## 設計

### config.json

`~/.config/bms-elsa/config.json` に設定を保存する。elsa.db と同じディレクトリ。

```json
{
  "songdataDBPath": "/path/to/songdata.db"
}
```

### パス解決の優先順位

1. config.json の `songdataDBPath`（空文字でなければ使用）
2. `~/.beatoraja/songdata.db`（既存の自動検索）
3. `~/beatoraja/songdata.db`（既存の自動検索）
4. 空文字（楽曲一覧は空、起動は継続）

### 変更箇所

**`app.go`**:
- `Config` 構造体を定義（`SongdataDBPath string`）
- `loadConfig()` 関数を追加: config.json を読み込み、存在しなければデフォルト値を返す
- `songdataDBPath()` を修正: config.json の値を最優先で使用

### やらないこと

- UI設定画面（将来スコープ）
- config.json のバリデーションUI
- testdata/songdata.db の自動検索追加

## 動作確認手順

1. config.json にsongdata.dbのパスを設定
2. `wails dev` で開発サーバー起動、楽曲一覧が表示されることを確認
3. `wails build` でバイナリ生成、起動して同様に確認
