# LR2IR事前取得CLIコマンド 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** LR2IRの全MD5のIR情報を事前取得するCLIコマンドを作成し、初期化済みelsa.dbをビルドに同梱する。

**Architecture:** `cmd/prefetch-ir/main.go` にスタンドアロンCLIを作成。既存の `gateway.LR2IRClient`（interval設定可能に小改修）と `persistence.ElsaRepository` を再利用し、CSV読み込み→IR取得→DB保存を逐次実行する。SIGINT受信で安全に中断し、再開IDを表示する。

**Tech Stack:** Go, `modernc.org/sqlite`, `encoding/csv`, `os/signal`

---

### Task 1: LR2IRClient にリクエスト間隔設定メソッドを追加

**Files:**
- Modify: `internal/adapter/gateway/lr2ir_client.go`

**Step 1: `SetInterval` メソッドを追加**

`lr2ir_client.go` の `LR2IRClient` 構造体に `interval` フィールドを追加し、デフォルトで `minRequestInterval` を使用するようにする。

```go
type LR2IRClient struct {
	client   *http.Client
	baseURL  string
	mu       sync.Mutex
	lastReq  time.Time
	interval time.Duration // 追加
}
```

コンストラクタ `NewLR2IRClient()` と `NewLR2IRClientWithBaseURL()` で `interval: minRequestInterval` を初期化。

`SetInterval` メソッドを追加:
```go
// SetInterval はリクエスト間隔を設定する（デフォルト: 500ms）
func (c *LR2IRClient) SetInterval(d time.Duration) {
	c.interval = d
}
```

`LookupByMD5` 内の `minRequestInterval` 参照を `c.interval` に置換（2箇所: `time.Since(c.lastReq) < minRequestInterval` と `time.NewTimer(minRequestInterval - elapsed)`）。

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: SUCCESS

**Step 3: 既存テスト確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/gateway/...`
Expected: PASS（既存テストがあれば）

**Step 4: コミット**

```bash
git add internal/adapter/gateway/lr2ir_client.go
git commit -m "feat: LR2IRClientにリクエスト間隔設定メソッドを追加"
```

---

### Task 2: CSVファイルをリポジトリに配置

**Files:**
- Create: `cmd/prefetch-ir/bmsid-md5-map.csv`（ダウンロードフォルダからコピー）

**Step 1: ディレクトリ作成とCSVコピー**

```bash
mkdir -p cmd/prefetch-ir
cp /Users/yudai.kuroki/Downloads/bmsid-md5-map.csv cmd/prefetch-ir/
```

**Step 2: コミット**

```bash
git add cmd/prefetch-ir/bmsid-md5-map.csv
git commit -m "data: LR2IR MD5リストCSVを追加"
```

---

### Task 3: prefetch-ir CLIコマンドの実装

**Files:**
- Create: `cmd/prefetch-ir/main.go`

**Step 1: main.go を作成**

```go
package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type entry struct {
	bmsid int
	md5   string
}

func main() {
	dbPath := flag.String("db", "build/elsa.db", "出力先elsa.dbパス")
	csvPath := flag.String("csv", "cmd/prefetch-ir/bmsid-md5-map.csv", "bmsid,md5のCSVファイルパス")
	interval := flag.Duration("interval", 200*time.Millisecond, "リクエスト間隔")
	startID := flag.Int("start-id", 0, "再開時のbmsid（この値以降から処理）")
	flag.Parse()

	entries, err := loadCSV(*csvPath, *startID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CSV読み込みエラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("CSV読み込み完了: %d件（start-id=%d以降）\n", len(entries), *startID)

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := persistence.RunMigrations(db); err != nil {
		fmt.Fprintf(os.Stderr, "migration: %v\n", err)
		os.Exit(1)
	}

	repo := persistence.NewElsaRepository(db)
	irClient := gateway.NewLR2IRClient()
	irClient.SetInterval(*interval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SIGINT受信で安全に停止
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	total := len(entries)
	startTime := time.Now()
	var lastBmsID int
	skipped := 0
	fetched := 0
	notFound := 0
	failed := 0

	for i, e := range entries {
		if ctx.Err() != nil {
			break
		}

		lastBmsID = e.bmsid

		// fetched_at済みならスキップ
		existing, err := repo.GetChartMeta(ctx, e.md5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nDB読み取りエラー bmsid=%d: %v\n", e.bmsid, err)
			failed++
			continue
		}
		if existing != nil && existing.FetchedAt != nil {
			skipped++
			continue
		}

		// IR取得
		resp, err := irClient.LookupByMD5(ctx, e.md5)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Fprintf(os.Stderr, "\nIR取得エラー bmsid=%d md5=%s: %v\n", e.bmsid, e.md5, err)
			failed++
			continue
		}

		// DB保存
		now := time.Now()
		meta := model.ChartIRMeta{
			MD5:       e.md5,
			FetchedAt: &now,
		}
		if resp.Registered {
			meta.Tags = resp.Tags
			meta.LR2IRBodyURL = resp.BodyURL
			meta.LR2IRDiffURL = resp.DiffURL
			meta.LR2IRNotes = resp.Notes
			fetched++
		} else {
			notFound++
		}
		if err := repo.UpsertChartMeta(ctx, meta); err != nil {
			fmt.Fprintf(os.Stderr, "\nDB保存エラー bmsid=%d: %v\n", e.bmsid, err)
			failed++
			continue
		}

		// 進捗表示
		processed := i + 1
		elapsed := time.Since(startTime)
		rate := float64(processed-skipped) / elapsed.Seconds()
		remaining := time.Duration(0)
		if rate > 0 {
			remaining = time.Duration(float64(total-processed) / rate * float64(time.Second))
		}
		status := "registered"
		if !resp.Registered {
			status = "not_found"
		}
		fmt.Fprintf(os.Stderr, "\r[%d/%d] bmsid=%d md5=%s %s (残り約%s)    ",
			processed, total, e.bmsid, e.md5[:12], status, remaining.Truncate(time.Second))
	}

	fmt.Fprintln(os.Stderr)
	fmt.Printf("完了: fetched=%d not_found=%d skipped=%d failed=%d\n", fetched, notFound, skipped, failed)
	if ctx.Err() != nil {
		fmt.Printf("中断しました。再開するには: --start-id %d\n", lastBmsID)
	}
}

func loadCSV(path string, startID int) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	// ヘッダースキップ
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("ヘッダー読み込みエラー: %w", err)
	}

	var entries []entry
	for {
		record, err := r.Read()
		if err != nil {
			break // io.EOF含む
		}
		if len(record) < 2 {
			continue
		}
		id, err := strconv.Atoi(record[0])
		if err != nil {
			continue
		}
		if id < startID {
			continue
		}
		entries = append(entries, entry{bmsid: id, md5: record[1]})
	}
	return entries, nil
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./cmd/prefetch-ir/`
Expected: SUCCESS

**Step 3: ヘルプ表示確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go run ./cmd/prefetch-ir/ --help`
Expected: フラグのヘルプが表示される

**Step 4: 小規模テスト（最初の5件だけ取得）**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go run ./cmd/prefetch-ir/ --db /tmp/test-prefetch.db --interval 500ms --start-id 1`
Expected: 数件取得して進捗が表示される。Ctrl+Cで中断IDが表示される。

**Step 5: コミット**

```bash
git add cmd/prefetch-ir/main.go
git commit -m "feat: LR2IR事前取得CLIコマンドを追加"
```

---

### Task 4: CI設定にelsa.db同梱を追加

**Files:**
- Modify: `.github/workflows/build-windows.yml`

**Step 1: elsa.dbコピーステップとzip対象を追加**

`Copy manual as manual.txt` ステップの後に追加:
```yaml
      - name: Copy elsa.db
        run: Copy-Item build/elsa.db build/bin/elsa.db
```

`Create ZIP` ステップのパスにelsa.dbを追加:
```yaml
      - name: Create ZIP
        run: |
          Compress-Archive -Path build/bin/bms-elsa.exe, build/bin/manual.txt, build/bin/elsa.db -DestinationPath build/bin/bms-elsa.zip
```

**Step 2: コミット**

```bash
git add .github/workflows/build-windows.yml
git commit -m "ci: ビルド成果物にelsa.dbを同梱"
```

---

### Task 5: .gitignore確認とelsa.db配置

**Files:**
- Modify: `.gitignore`（必要に応じて）

**Step 1: .gitignoreでbuild/elsa.dbが除外されていないか確認**

もし `*.db` や `build/` がgitignoreされている場合、`!build/elsa.db` の例外を追加する。

**Step 2: 空のelsa.dbを生成してコミット（実際のprefetch実行前のプレースホルダ）**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
go run ./cmd/prefetch-ir/ --db build/elsa.db --csv cmd/prefetch-ir/bmsid-md5-map.csv --start-id 999999999
```

これにより、マイグレーション済みの空elsa.dbが `build/elsa.db` に作成される（start-idが大きすぎるので0件取得）。

```bash
git add build/elsa.db .gitignore
git commit -m "chore: マイグレーション済み空elsa.dbを追加"
```

> **注意**: 実際のIR取得（約18.7時間）は手動で別途実行し、完了後に `build/elsa.db` を再コミットする。
