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

type csvRow struct {
	bmsID int
	md5   string
}

func main() {
	dbPath := flag.String("db", "build/elsa.db", "出力先elsa.dbのパス")
	csvPath := flag.String("csv", "cmd/prefetch-ir/bmsid-md5-map.csv", "bmsid,md5のCSVファイル")
	interval := flag.Duration("interval", 200*time.Millisecond, "リクエスト間隔")
	startID := flag.Int("start-id", 0, "再開時のbmsid（この値以上の行を処理）")
	flag.Parse()

	// CSV読み込み
	allRows, err := loadCSV(*csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CSV読み込みエラー: %v\n", err)
		os.Exit(1)
	}
	// start-id でフィルタ
	var rows []csvRow
	for _, r := range allRows {
		if r.bmsID >= *startID {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "処理対象の行がありません")
		return
	}

	// DB接続
	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB接続エラー: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(1) // ATTACH DATABASE互換

	if err := persistence.RunMigrations(db); err != nil {
		fmt.Fprintf(os.Stderr, "マイグレーションエラー: %v\n", err)
		os.Exit(1)
	}

	repo := persistence.NewElsaRepository(db)
	irClient := gateway.NewLR2IRClient()
	irClient.SetInterval(*interval)

	// SIGINT対応
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	var lastBmsID int
	var fetchedCount int     // 実際にIRリクエストした件数
	fetchStart := time.Now() // fetch開始時刻

	for i, row := range rows {
		if ctx.Err() != nil {
			break
		}

		lastBmsID = row.bmsID

		// 既存チェック: fetched_atがあればスキップ
		existing, err := repo.GetChartMeta(ctx, row.md5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n取得エラー (bmsid=%d): %v\n", row.bmsID, err)
			continue
		}
		if existing != nil && existing.FetchedAt != nil {
			printProgress(os.Stderr, row.bmsID, row.md5, false, fetchedCount, i, len(rows), fetchStart)
			continue
		}

		// IR取得
		irResp, err := irClient.LookupByMD5(ctx, row.md5)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Fprintf(os.Stderr, "\nIR取得エラー (bmsid=%d, md5=%s): %v\n", row.bmsID, row.md5, err)
			continue
		}
		fetchedCount++

		now := time.Now().UTC()
		meta := model.ChartIRMeta{
			MD5:       row.md5,
			FetchedAt: &now,
		}

		if irResp.Registered {
			meta.Tags = irResp.Tags
			meta.LR2IRBodyURL = irResp.BodyURL
			meta.LR2IRDiffURL = irResp.DiffURL
			meta.LR2IRNotes = irResp.Notes
		}

		if err := repo.UpsertChartMeta(ctx, meta); err != nil {
			fmt.Fprintf(os.Stderr, "\n保存エラー (bmsid=%d): %v\n", row.bmsID, err)
			continue
		}

		printProgress(os.Stderr, row.bmsID, row.md5, irResp.Registered, fetchedCount, i, len(rows), fetchStart)
	}

	fmt.Fprintln(os.Stderr) // 最終行の改行

	if ctx.Err() != nil {
		fmt.Fprintf(os.Stderr, "中断しました。再開するには: --start-id %d\n", lastBmsID)
	} else {
		fmt.Fprintln(os.Stderr, "完了しました")
	}
}

// loadCSV はCSVファイルの全行を読み込む（ヘッダー除く）
func loadCSV(path string) ([]csvRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var rows []csvRow
	for i, rec := range records {
		if i == 0 {
			continue // ヘッダースキップ
		}
		if len(rec) != 2 {
			return nil, fmt.Errorf("行%d: 列数が不正 (%d)", i+1, len(rec))
		}
		bmsID, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, fmt.Errorf("行%d: bmsid変換失敗: %w", i+1, err)
		}
		rows = append(rows, csvRow{bmsID: bmsID, md5: rec[1]})
	}

	return rows, nil
}

// printProgress は進捗を\rで1行上書き表示する
// 残り時間は実際にIRリクエストした件数の処理速度で推定する
func printProgress(w *os.File, bmsID int, md5 string, registered bool, fetchedCount, idx, filteredCount int, fetchStart time.Time) {
	remaining := ""
	if fetchedCount > 0 {
		elapsed := time.Since(fetchStart)
		avgPerFetch := elapsed / time.Duration(fetchedCount)
		remainCount := filteredCount - idx - 1
		if remainCount > 0 {
			est := avgPerFetch * time.Duration(remainCount)
			remaining = formatDuration(est)
		}
	}

	if remaining != "" {
		fmt.Fprintf(w, "\r[%d/%d] bmsid=%d md5=%s registered=%t (残り約%s)   ",
			idx+1, filteredCount, bmsID, md5, registered, remaining)
	} else {
		fmt.Fprintf(w, "\r[%d/%d] bmsid=%d md5=%s registered=%t   ",
			idx+1, filteredCount, bmsID, md5, registered)
	}
}

// formatDuration は時間を "XhYYm" 形式にフォーマットする
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
