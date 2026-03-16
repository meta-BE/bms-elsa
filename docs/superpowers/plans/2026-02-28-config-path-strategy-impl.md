# config.json / elsa.db パス戦略 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** config.jsonとelsa.dbの保存先を `os.UserConfigDir()` から `os.Executable()` 隣接に変更する

**Architecture:** `appDir()` 関数を追加し、`loadConfig()` と `elsaDBPath()` がそれを使うように書き換える。`songdataDBPath()` は変更しない。

**Tech Stack:** Go 1.24, Wails v2

---

### Task 1: appDir() 追加 + loadConfig() / elsaDBPath() 書き換え

**Files:**
- Modify: `app.go:90-122`

**Step 1: appDir() を追加し、loadConfig() と elsaDBPath() を書き換え**

`app.go` に以下の変更を加える:

1. `appDir()` 関数を追加:

```go
// appDir は実行ファイルと同じディレクトリを返す。
// config.jsonやelsa.dbの保存先として使用する。
func appDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
```

2. `loadConfig()` を書き換え:

```go
// loadConfig は実行ファイルと同じディレクトリの config.json を読み込む。
// ファイルが存在しない場合はゼロ値の Config を返す。
func loadConfig() Config {
	path := filepath.Join(appDir(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg
}
```

変更点:
- `os.UserConfigDir()` → `appDir()`
- `os.Open` + `io.ReadAll` → `os.ReadFile`（シンプル化）
- `"io"` を import から削除

3. `elsaDBPath()` を書き換え:

```go
// elsaDBPath は実行ファイルと同じディレクトリの elsa.db パスを返す
func elsaDBPath() string {
	return filepath.Join(appDir(), "elsa.db")
}
```

変更点:
- `os.UserConfigDir()` → `appDir()`
- `os.MkdirAll` を削除（実行ファイルのディレクトリは既に存在する）

**Step 2: ビルドして動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 3: 既存テストが通ることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テストPASS（app.goのパス関数にはテストがないが、他のテストが壊れていないことを確認）

**Step 4: コミット**

```bash
git add app.go
git commit -m "config.json/elsa.dbのパスを実行ファイル隣接に変更

os.UserConfigDir()ベースからos.Executable()ベースに統一。
appDir()関数でパス解決を集約。"
```

### Task 2: README更新

**Files:**
- Modify: `README.md`

**Step 1: セットアップ手順を更新**

`README.md` のセットアップセクションを書き換え:
- `~/Library/Application Support/bms-elsa/` への配置手順を削除
- 実行ファイルと同じディレクトリに `config.json` を置く手順に変更
- `config.json` 省略時の自動検出説明はそのまま（songdataDBPathは変更なし）

**Step 2: コミット**

```bash
git add README.md
git commit -m "READMEのセットアップ手順をパス戦略変更に合わせて更新"
```

### Task 3: TODO更新

**Files:**
- Modify: `docs/TODO.md`

**Step 1: TODOを完了にマーク**

`config.json / elsa.db のパス戦略` を `[x]` に変更。

**Step 2: コミット**

```bash
git add docs/TODO.md
git commit -m "TODOのパス戦略タスクを完了にマーク"
```
