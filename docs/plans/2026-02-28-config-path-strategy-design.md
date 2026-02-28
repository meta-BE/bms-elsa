# config.json / elsa.db パス戦略

## 概要

config.jsonとelsa.dbの保存先を、OS標準パス（`os.UserConfigDir()`）から実行ファイル隣接に変更する。

## 動機

現状は `~/Library/Application Support/bms-elsa/`（macOS）のようにOS固有のパスにファイルが散らばる。実行ファイルの隣に全てまとめることで、ファイルの場所が一目瞭然になる。

## 設計

### appDir() 関数

`os.Executable()` → `filepath.EvalSymlinks()` → `filepath.Dir()` で実行ファイルのディレクトリを返す共通関数を追加。`loadConfig()` と `elsaDBPath()` がこの関数を使う。

### wails dev 対策

`wails dev` は `build/bin/` にバイナリを作成するため、`appDir()` は `build/bin/` を返す。config.jsonとelsa.dbが `build/bin/` 内に作られるが、`.gitignore` で無視する。

### 変更ファイル

- `app.go`: `appDir()` 追加、`loadConfig()` / `elsaDBPath()` 書き換え
- `.gitignore`: `build/bin/config.json`, `build/bin/elsa.db` 追加

### 変更しないもの

- `songdataDBPath()`: config.json内の指定 or `~/.beatoraja/` 自動検出のまま

### 採用しなかったアプローチ

- **B: 隣接優先 + UserConfigDirフォールバック**: 2パスの探索ロジックが複雑。パスの決定が常に一意であるAのほうがシンプル。
- **C: カレントディレクトリ基準**: 起動場所依存で不安定。
