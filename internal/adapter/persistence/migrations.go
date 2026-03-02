package persistence

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

//go:embed event_mappings.csv
var eventMappingsCSV []byte

// RunMigrations はelsa.dbのスキーマを作成する。冪等。
func RunMigrations(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS song_meta (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_hash   TEXT NOT NULL UNIQUE,
			release_year  INTEGER,
			event_name    TEXT,
			created_at    TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS chart_meta (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			md5              TEXT NOT NULL UNIQUE,
			sha256           TEXT NOT NULL DEFAULT '',
			lr2ir_tags       TEXT,
			lr2ir_body_url   TEXT,
			lr2ir_diff_url   TEXT,
			lr2ir_notes      TEXT,
			lr2ir_fetched_at TEXT,
			working_body_url TEXT,
			working_diff_url TEXT,
			created_at       TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_song_meta_folder_hash ON song_meta(folder_hash)`,
		`CREATE TABLE IF NOT EXISTS difficulty_table (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			url         TEXT NOT NULL UNIQUE,
			header_url  TEXT NOT NULL,
			data_url    TEXT NOT NULL,
			name        TEXT NOT NULL,
			symbol      TEXT NOT NULL,
			fetched_at  TEXT,
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS difficulty_table_entry (
			table_id    INTEGER NOT NULL REFERENCES difficulty_table(id) ON DELETE CASCADE,
			md5         TEXT NOT NULL,
			level       TEXT NOT NULL,
			title       TEXT,
			artist      TEXT,
			url         TEXT,
			url_diff    TEXT,
			PRIMARY KEY (table_id, md5)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dte_md5 ON difficulty_table_entry(md5)`,
		`CREATE TABLE IF NOT EXISTS event_mapping (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			url_pattern  TEXT NOT NULL UNIQUE,
			event_name   TEXT NOT NULL,
			release_year INTEGER NOT NULL
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	// 埋め込みCSVからシードデータを投入（冪等）
	records, err := csv.NewReader(bytes.NewReader(eventMappingsCSV)).ReadAll()
	if err != nil {
		return fmt.Errorf("event_mappings.csv パース失敗: %w", err)
	}
	for i, rec := range records {
		if i == 0 {
			continue // ヘッダー行スキップ
		}
		if len(rec) != 3 {
			return fmt.Errorf("event_mappings.csv 行%d: 列数が不正 (%d)", i+1, len(rec))
		}
		year, err := strconv.Atoi(rec[2])
		if err != nil {
			return fmt.Errorf("event_mappings.csv 行%d: release_year変換失敗: %w", i+1, err)
		}
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO event_mapping (url_pattern, event_name, release_year) VALUES (?, ?, ?)`,
			rec[0], rec[1], year,
		); err != nil {
			return err
		}
	}

	// chart_meta: (md5, sha256) UNIQUE → md5 UNIQUE に変更
	var hasOldSchema int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chart_meta') WHERE name='sha256'`).Scan(&hasOldSchema)
	if hasOldSchema > 0 {
		// 旧スキーマの場合のみマイグレーション
		row := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='chart_meta'`)
		var ddl string
		_ = row.Scan(&ddl)
		if strings.Contains(ddl, "UNIQUE(md5, sha256)") {
			if _, err := db.Exec(`
				CREATE TABLE chart_meta_new (
					id               INTEGER PRIMARY KEY AUTOINCREMENT,
					md5              TEXT NOT NULL UNIQUE,
					sha256           TEXT NOT NULL DEFAULT '',
					lr2ir_tags       TEXT,
					lr2ir_body_url   TEXT,
					lr2ir_diff_url   TEXT,
					lr2ir_notes      TEXT,
					lr2ir_fetched_at TEXT,
					working_body_url TEXT,
					working_diff_url TEXT,
					created_at       TEXT NOT NULL DEFAULT (datetime('now')),
					updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
				);
				INSERT OR IGNORE INTO chart_meta_new
					(md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
					 lr2ir_fetched_at, working_body_url, working_diff_url, created_at, updated_at)
				SELECT md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
					lr2ir_fetched_at, working_body_url, working_diff_url, created_at, updated_at
				FROM chart_meta
				GROUP BY md5
				HAVING id = MAX(id);
				DROP TABLE chart_meta;
				ALTER TABLE chart_meta_new RENAME TO chart_meta;
			`); err != nil {
				return fmt.Errorf("chart_meta migration: %w", err)
			}
		}
	}

	return nil
}
