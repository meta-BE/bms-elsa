# config.json パス設定とビルド動作確認 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** config.json で songdata.db のパスを指定し、wails dev / wails build でアプリの動作確認ができるようにする

**Architecture:** `app.go` に Config 構造体と loadConfig() を追加。songdataDBPath() を config.json 優先に変更。変更は app.go のみ。

**Tech Stack:** Go 1.24, encoding/json (標準ライブラリ)

**設計ドキュメント:** `docs/plans/2026-02-28-config-and-build-verification-design.md`

---

## Task 1: config.json 読み込みとパス解決の実装

**Files:**
- Modify: `app.go`

**Step 1: Config 構造体と loadConfig() を実装**

`app.go` の import に `"encoding/json"`, `"io"` を追加し、`elsaDBPath()` の直前に以下を追加:

```go
// Config はアプリケーション設定
type Config struct {
	SongdataDBPath string `json:"songdataDBPath"`
}

// loadConfig は ~/.config/bms-elsa/config.json を読み込む。
// ファイルが存在しない場合はゼロ値の Config を返す。
func loadConfig() Config {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}
	}
	path := filepath.Join(configDir, "bms-elsa", "config.json")
	f, err := os.Open(path)
	if err != nil {
		return Config{}
	}
	defer f.Close()

	var cfg Config
	data, err := io.ReadAll(f)
	if err != nil {
		return Config{}
	}
	json.Unmarshal(data, &cfg)
	return cfg
}
```

**Step 2: songdataDBPath() を修正**

既存の `songdataDBPath()` を以下に置き換え:

```go
// songdataDBPath はsongdata.dbのパスを返す。
// 優先順位: config.json → ~/.beatoraja/ → ~/beatoraja/
func songdataDBPath() string {
	cfg := loadConfig()
	if cfg.SongdataDBPath != "" {
		if _, err := os.Stat(cfg.SongdataDBPath); err == nil {
			return cfg.SongdataDBPath
		}
		fmt.Fprintf(os.Stderr, "config.json の songdataDBPath が見つかりません: %s\n", cfg.SongdataDBPath)
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".beatoraja", "songdata.db"),
		filepath.Join(home, "beatoraja", "songdata.db"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
```

**Step 3: ビルド確認**

```bash
cd /path/to/bms-elsa
go build ./...
go vet ./...
```

Expected: エラーなし

**Step 4: テスト確認**

```bash
go test ./... -count=1
```

Expected: 全テスト PASS

**Step 5: コミット**

```bash
git add app.go
git commit -m "feat: config.jsonによるsongdata.dbパス指定をサポート"
```

---

## Task 2: config.json を作成して wails dev で動作確認

**Step 1: config.json を作成**

```bash
mkdir -p ~/.config/bms-elsa
cat > ~/.config/bms-elsa/config.json << 'EOF'
{
  "songdataDBPath": "/path/to/actual/songdata.db"
}
EOF
```

パスはユーザーの環境に合わせて指定する（例: testdata/songdata.db の絶対パス）。

**Step 2: wails dev で起動**

```bash
cd /path/to/bms-elsa
wails dev
```

確認項目:
- アプリウィンドウが開く
- 楽曲一覧にデータが表示される
- ソート・検索が動作する
- 行クリックで詳細パネルが表示される
- メタデータ（Event, Year）の編集が保存される

**Step 3: wails dev を終了**

確認が完了したら Ctrl+C で終了。

---

## Task 3: wails build でバイナリ生成と動作確認

**Step 1: プロダクションビルド**

```bash
cd /path/to/bms-elsa
wails build
```

Expected: `build/bin/bms-elsa` が生成される

**Step 2: バイナリを起動して確認**

```bash
./build/bin/bms-elsa
```

確認項目:
- Task 2 と同じ確認項目

**Step 3: 確認結果をコミットメッセージに記録**

問題なければ最終コミット不要（Task 1 で完了）。問題があれば修正してコミット。

---

## タスク依存関係

```
Task 1 (config.json実装)
  └── Task 2 (wails dev確認)
        └── Task 3 (wails build確認)
```
