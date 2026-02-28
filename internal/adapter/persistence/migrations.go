package persistence

import "database/sql"

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
			md5              TEXT NOT NULL,
			sha256           TEXT NOT NULL,
			lr2ir_tags       TEXT,
			lr2ir_body_url   TEXT,
			lr2ir_diff_url   TEXT,
			lr2ir_notes      TEXT,
			lr2ir_fetched_at TEXT,
			working_body_url TEXT,
			working_diff_url TEXT,
			created_at       TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(md5, sha256)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_song_meta_folder_hash ON song_meta(folder_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_chart_meta_md5 ON chart_meta(md5)`,
		`CREATE INDEX IF NOT EXISTS idx_chart_meta_sha256 ON chart_meta(sha256)`,
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
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
