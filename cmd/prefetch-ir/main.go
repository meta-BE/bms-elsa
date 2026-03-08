// LR2IRの全MD5に対してIR情報を事前取得し、elsa.dbに保存するスクリプト
// 使い方: go run ./cmd/prefetch-ir [--db build/elsa.db] [--csv cmd/prefetch-ir/bmsid-md5-map.csv] [--interval 200ms] [--start-line 1]
// 中断時に表示される --start-line の値を指定することで途中から再開できる
package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type csvRow struct {
	line int    // CSV内の行番号（ヘッダー除く、1始まり）
	md5  string
}

func main() {
	dbPath := flag.String("db", "build/elsa.db", "出力先elsa.dbのパス")
	csvPath := flag.String("csv", "cmd/prefetch-ir/bmsid-md5-map.csv", "bmsid,md5のCSVファイル")
	interval := flag.Duration("interval", 200*time.Millisecond, "リクエスト間隔")
	startLine := flag.Int("start-line", 1, "再開時の行番号（この値以降の行を処理、1始まり）")
	flag.Parse()

	// CSV読み込み（start-line以降をフィルタ）
	rows, totalLines, err := loadCSV(*csvPath, *startLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CSV読み込みエラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "CSV: 全%d行、処理対象%d行（start-line=%d〜）\n", totalLines, len(rows), *startLine)

	// DB接続（処理対象0件でもマイグレーション済みDBを作成するため、先に実行）
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

	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "処理対象の行がありません")
		return
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

	var lastLine int
	var fetchedCount int     // 実際にIRリクエストした件数
	fetchStart := time.Now() // fetch開始時刻

	for i, row := range rows {
		if ctx.Err() != nil {
			break
		}

		lastLine = row.line

		// 既存チェック: fetched_atがあればスキップ
		existing, err := repo.GetChartMeta(ctx, row.md5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n取得エラー (line=%d): %v\n", row.line, err)
			continue
		}
		if existing != nil && existing.FetchedAt != nil {
			printProgress(os.Stderr, row.line, row.md5, false, fetchedCount, i, len(rows), fetchStart)
			continue
		}

		// IR取得
		irResp, err := irClient.LookupByMD5(ctx, row.md5)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Fprintf(os.Stderr, "\nIR取得エラー (line=%d, md5=%s): %v\n", row.line, row.md5, err)
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
			fmt.Fprintf(os.Stderr, "\n保存エラー (line=%d): %v\n", row.line, err)
			continue
		}

		printProgress(os.Stderr, row.line, row.md5, irResp.Registered, fetchedCount, i, len(rows), fetchStart)
	}

	fmt.Fprintln(os.Stderr) // 最終行の改行

	if ctx.Err() != nil {
		fmt.Fprintf(os.Stderr, "中断しました。再開するには: --start-line %d\n", lastLine)
	} else {
		fmt.Fprintln(os.Stderr, "完了しました")
	}
}

// loadCSV はCSVファイルを読み込み、startLine以降の行を返す。
// 行番号はヘッダー除く1始まり。bmsidが空や不正でもmd5があれば処理対象に含める。
func loadCSV(path string, startLine int) (rows []csvRow, totalLines int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // 列数不定を許容

	// ヘッダースキップ
	if _, err := reader.Read(); err != nil {
		return nil, 0, fmt.Errorf("ヘッダー読み込みエラー: %w", err)
	}

	line := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("行%d: CSV読み込みエラー: %w", line+1, err)
		}
		line++

		// md5列（2列目）を取得。空行やmd5なしはスキップ
		md5 := ""
		if len(rec) >= 2 {
			md5 = strings.TrimSpace(rec[1])
		} else if len(rec) == 1 {
			md5 = strings.TrimSpace(rec[0])
		}
		if md5 == "" {
			continue
		}

		totalLines++
		if line >= startLine {
			rows = append(rows, csvRow{line: line, md5: md5})
		}
	}

	return rows, totalLines, nil
}

// printProgress は進捗を\rで1行上書き表示する
func printProgress(w *os.File, line int, md5 string, registered bool, fetchedCount, idx, filteredCount int, fetchStart time.Time) {
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
		fmt.Fprintf(w, "\r[%d/%d] line=%d md5=%s registered=%t (残り約%s)   ",
			idx+1, filteredCount, line, md5, registered, remaining)
	} else {
		fmt.Fprintf(w, "\r[%d/%d] line=%d md5=%s registered=%t   ",
			idx+1, filteredCount, line, md5, registered)
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
