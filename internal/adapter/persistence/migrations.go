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

//go:embed events.csv
var eventsCSV []byte

//go:embed rewrite_rules.csv
var rewriteRulesCSV []byte

// RunMigrations はelsa.dbのスキーマを作成する。冪等。
func RunMigrations(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS event (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			bms_search_id  TEXT UNIQUE,
			name           TEXT NOT NULL,
			short_name     TEXT NOT NULL,
			release_year   INTEGER NOT NULL,
			url            TEXT NOT NULL DEFAULT '',
			created_at     TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS song_meta (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_hash     TEXT NOT NULL UNIQUE,
			release_year    INTEGER,
			event_id        TEXT,
			bms_search_id   TEXT,
			created_at      TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
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
			sort_order  INTEGER NOT NULL DEFAULT 0,
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
		`CREATE TABLE IF NOT EXISTS url_rewrite_rule (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_type   TEXT NOT NULL CHECK(rule_type IN ('replace', 'regex')),
			pattern     TEXT NOT NULL,
			replacement TEXT NOT NULL,
			priority    INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(rule_type, pattern)
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	// events.csvからシードデータを投入（冪等）
	if err := seedEvents(db); err != nil {
		return err
	}

	// 旧event_mappingテーブルを削除（冪等）
	if _, err := db.Exec(`DROP TABLE IF EXISTS event_mapping`); err != nil {
		return fmt.Errorf("drop event_mapping: %w", err)
	}

	// song_metaスキーマ移行: event_nameカラムがあればevent_id+bms_search_idに移行
	if err := migrateSongMeta(db); err != nil {
		return err
	}

	// URL書き換えルールのシードデータ投入（冪等）
	rwRecords, err := csv.NewReader(bytes.NewReader(rewriteRulesCSV)).ReadAll()
	if err != nil {
		return fmt.Errorf("rewrite_rules.csv パース失敗: %w", err)
	}
	for i, rec := range rwRecords {
		if i == 0 {
			continue // ヘッダー行スキップ
		}
		if len(rec) != 4 {
			return fmt.Errorf("rewrite_rules.csv 行%d: 列数が不正 (%d)", i+1, len(rec))
		}
		priority, err := strconv.Atoi(rec[3])
		if err != nil {
			return fmt.Errorf("rewrite_rules.csv 行%d: priority変換失敗: %w", i+1, err)
		}
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO url_rewrite_rule (rule_type, pattern, replacement, priority) VALUES (?, ?, ?, ?)`,
			rec[0], rec[1], rec[2], priority,
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

	// wav_minhashカラムの追加（冪等）
	var hasWavMinhash int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chart_meta') WHERE name='wav_minhash'`).Scan(&hasWavMinhash)
	if hasWavMinhash == 0 {
		if _, err := db.Exec(`ALTER TABLE chart_meta ADD COLUMN wav_minhash BLOB`); err != nil {
			return fmt.Errorf("add wav_minhash column: %w", err)
		}
	}

	// sort_orderカラムの追加（冪等）
	var hasSortOrder int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('difficulty_table') WHERE name='sort_order'`).Scan(&hasSortOrder)
	if hasSortOrder == 0 {
		if _, err := db.Exec(`ALTER TABLE difficulty_table ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add sort_order column: %w", err)
		}
		// 既存行にはid順でsort_orderを振る
		if _, err := db.Exec(`UPDATE difficulty_table SET sort_order = id`); err != nil {
			return fmt.Errorf("init sort_order: %w", err)
		}
	}

	// song_meta.event_id: INTEGER(event.id参照) → TEXT(event.bms_search_id)への移行（冪等）
	if err := migrateSongMetaEventIDToText(db); err != nil {
		return err
	}

	// event.urlカラムの追加（冪等）
	var hasEventURL int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('event') WHERE name='url'`).Scan(&hasEventURL)
	if hasEventURL == 0 {
		if _, err := db.Exec(`ALTER TABLE event ADD COLUMN url TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add event url column: %w", err)
		}
	}

	return nil
}

// seedEvents はevents.csvからeventテーブルにシードデータを投入する（冪等）
func seedEvents(db *sql.DB) error {
	records, err := csv.NewReader(bytes.NewReader(eventsCSV)).ReadAll()
	if err != nil {
		return fmt.Errorf("events.csv パース失敗: %w", err)
	}
	for i, rec := range records {
		if i == 0 {
			continue // ヘッダー行スキップ
		}
		if len(rec) != 4 && len(rec) != 5 {
			return fmt.Errorf("events.csv 行%d: 列数が不正 (%d)", i+1, len(rec))
		}
		bmsSearchID := rec[0]
		name := rec[1]
		shortName := rec[2]
		year, err := strconv.Atoi(rec[3])
		if err != nil {
			return fmt.Errorf("events.csv 行%d: release_year変換失敗: %w", i+1, err)
		}
		url := ""
		if len(rec) == 5 {
			url = rec[4]
		}

		// bms_search_idが空文字列の場合はNULLとして挿入
		var searchIDParam interface{}
		if bmsSearchID != "" {
			searchIDParam = bmsSearchID
		}

		if _, err := db.Exec(
			`INSERT OR IGNORE INTO event (bms_search_id, name, short_name, release_year, url) VALUES (?, ?, ?, ?, ?)`,
			searchIDParam, name, shortName, year, url,
		); err != nil {
			return err
		}
	}
	return nil
}

// migrateSongMeta は旧song_meta（event_nameカラムあり）を新スキーマに移行する
func migrateSongMeta(db *sql.DB) error {
	var hasEventName int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('song_meta') WHERE name='event_name'`).Scan(&hasEventName)
	if hasEventName == 0 {
		// 新スキーマ（event_id, bms_search_id）または新規インストール — 移行不要
		return nil
	}

	// 旧スキーマからの移行
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS song_meta_new (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_hash     TEXT NOT NULL UNIQUE,
			release_year    INTEGER,
			event_id        TEXT,
			bms_search_id   TEXT,
			created_at      TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create song_meta_new: %w", err)
	}

	if _, err := db.Exec(`
		INSERT OR IGNORE INTO song_meta_new (id, folder_hash, release_year, created_at, updated_at)
		SELECT id, folder_hash, release_year, created_at, updated_at FROM song_meta
	`); err != nil {
		return fmt.Errorf("copy song_meta data: %w", err)
	}

	if _, err := db.Exec(`DROP TABLE song_meta`); err != nil {
		return fmt.Errorf("drop old song_meta: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE song_meta_new RENAME TO song_meta`); err != nil {
		return fmt.Errorf("rename song_meta_new: %w", err)
	}

	return nil
}

// migrateSongMetaEventIDToText はsong_meta.event_idをINTEGER(event.id参照)からTEXT(event.bms_search_id)に移行する
func migrateSongMetaEventIDToText(db *sql.DB) error {
	row := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='song_meta'`)
	var ddl string
	if err := row.Scan(&ddl); err != nil {
		return nil
	}
	// DDLに "INTEGER REFERENCES event" か "event_id        INTEGER" が含まれていれば旧スキーマ
	upper := strings.ToUpper(ddl)
	if !strings.Contains(upper, "EVENT_ID") {
		return nil
	}
	// 既にTEXT型の場合はスキップ（"EVENT_ID        TEXT" or "event_id TEXT"）
	if !strings.Contains(upper, "INTEGER REFERENCES EVENT") && !strings.Contains(upper, "EVENT_ID        INTEGER") && !strings.Contains(upper, "EVENT_ID INTEGER") {
		return nil
	}

	if _, err := db.Exec(`
		CREATE TABLE song_meta_new (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_hash     TEXT NOT NULL UNIQUE,
			release_year    INTEGER,
			event_id        TEXT,
			bms_search_id   TEXT,
			created_at      TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create song_meta_new for event_id text migration: %w", err)
	}

	// 旧INTEGER event_id → eventテーブル経由でbms_search_idに変換して移行
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO song_meta_new (id, folder_hash, release_year, event_id, bms_search_id, created_at, updated_at)
		SELECT sm.id, sm.folder_hash, sm.release_year, ev.bms_search_id, sm.bms_search_id, sm.created_at, sm.updated_at
		FROM song_meta sm
		LEFT JOIN event ev ON sm.event_id = ev.id
	`); err != nil {
		return fmt.Errorf("copy song_meta for event_id text migration: %w", err)
	}

	if _, err := db.Exec(`DROP TABLE song_meta`); err != nil {
		return fmt.Errorf("drop song_meta for event_id text migration: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE song_meta_new RENAME TO song_meta`); err != nil {
		return fmt.Errorf("rename song_meta_new for event_id text migration: %w", err)
	}

	return nil
}
