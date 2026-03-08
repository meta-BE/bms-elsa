// IR情報入りelsa.dbをユーザーのelsa.dbに統合するスクリプト
// 使い方: go run ./cmd/merge-db --source build/elsa.db --target path/to/user/elsa.db
// ターゲットに既にIR情報があるmd5はスキップし、ないmd5のみ補完する
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	source := flag.String("source", "build/elsa.db", "IR情報を持つソースelsa.dbのパス")
	target := flag.String("target", "", "統合先のユーザーelsa.dbのパス（必須）")
	flag.Parse()

	if *target == "" {
		fmt.Fprintln(os.Stderr, "エラー: --target は必須です")
		flag.Usage()
		os.Exit(1)
	}

	if _, err := os.Stat(*source); err != nil {
		fmt.Fprintf(os.Stderr, "ソースDB が見つかりません: %s\n", *source)
		os.Exit(1)
	}
	if _, err := os.Stat(*target); err != nil {
		fmt.Fprintf(os.Stderr, "ターゲットDB が見つかりません: %s\n", *target)
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ターゲットDB接続エラー: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	// ソースDBをATTACH
	if _, err := db.Exec("ATTACH DATABASE ? AS source", *source); err != nil {
		fmt.Fprintf(os.Stderr, "ソースDB ATTACH エラー: %v\n", err)
		os.Exit(1)
	}

	// マージ前の統計を取得
	var sourceCount int
	db.QueryRow("SELECT COUNT(*) FROM source.chart_meta WHERE lr2ir_fetched_at IS NOT NULL").Scan(&sourceCount)

	var beforeCount int
	db.QueryRow("SELECT COUNT(*) FROM main.chart_meta").Scan(&beforeCount)

	// ターゲットに既にIR情報がある件数（スキップ対象）
	var skipped int
	db.QueryRow(`
		SELECT COUNT(*) FROM main.chart_meta m
		INNER JOIN source.chart_meta s ON s.md5 = m.md5
		WHERE m.lr2ir_fetched_at IS NOT NULL AND s.lr2ir_fetched_at IS NOT NULL
	`).Scan(&skipped)

	// ターゲットにIR情報がないmd5のみ補完
	_, err = db.Exec(`
		INSERT INTO main.chart_meta (md5, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes, lr2ir_fetched_at)
		SELECT md5, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes, lr2ir_fetched_at
		FROM source.chart_meta
		WHERE lr2ir_fetched_at IS NOT NULL
		ON CONFLICT(md5) DO UPDATE SET
		  lr2ir_tags       = CASE WHEN main.chart_meta.lr2ir_fetched_at IS NULL
		                     THEN excluded.lr2ir_tags ELSE main.chart_meta.lr2ir_tags END,
		  lr2ir_body_url   = CASE WHEN main.chart_meta.lr2ir_fetched_at IS NULL
		                     THEN excluded.lr2ir_body_url ELSE main.chart_meta.lr2ir_body_url END,
		  lr2ir_diff_url   = CASE WHEN main.chart_meta.lr2ir_fetched_at IS NULL
		                     THEN excluded.lr2ir_diff_url ELSE main.chart_meta.lr2ir_diff_url END,
		  lr2ir_notes      = CASE WHEN main.chart_meta.lr2ir_fetched_at IS NULL
		                     THEN excluded.lr2ir_notes ELSE main.chart_meta.lr2ir_notes END,
		  lr2ir_fetched_at = CASE WHEN main.chart_meta.lr2ir_fetched_at IS NULL
		                     THEN excluded.lr2ir_fetched_at ELSE main.chart_meta.lr2ir_fetched_at END
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "マージエラー: %v\n", err)
		os.Exit(1)
	}

	var afterCount int
	db.QueryRow("SELECT COUNT(*) FROM main.chart_meta").Scan(&afterCount)

	inserted := afterCount - beforeCount
	updated := sourceCount - inserted - skipped

	fmt.Printf("ソース: %d件、新規追加: %d件、IR補完: %d件、スキップ（既存IR情報あり）: %d件\n",
		sourceCount, inserted, updated, skipped)
}
