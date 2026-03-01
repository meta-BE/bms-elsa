package persistence

import (
	"context"
	"database/sql"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// DifficultyTable は難易度表マスタ
type DifficultyTable struct {
	ID        int
	URL       string
	HeaderURL string
	DataURL   string
	Name      string
	Symbol    string
	FetchedAt *time.Time
}

// DifficultyTableEntry は難易度表の譜面エントリ
type DifficultyTableEntry struct {
	TableID int
	MD5     string
	Level   string
	Title   string
	Artist  string
	URL     string
	URLDiff string
}

type DifficultyTableRepository struct {
	db *sql.DB
}

func NewDifficultyTableRepository(db *sql.DB) *DifficultyTableRepository {
	return &DifficultyTableRepository{db: db}
}

func (r *DifficultyTableRepository) ListTables(ctx context.Context) ([]DifficultyTable, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, url, header_url, data_url, name, symbol, fetched_at
		FROM difficulty_table
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []DifficultyTable
	for rows.Next() {
		var t DifficultyTable
		var fetchedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.URL, &t.HeaderURL, &t.DataURL, &t.Name, &t.Symbol, &fetchedAt); err != nil {
			return nil, err
		}
		if fetchedAt.Valid {
			parsed, _ := time.ParseInLocation(timeLayout, fetchedAt.String, time.UTC)
			t.FetchedAt = &parsed
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (r *DifficultyTableRepository) InsertTable(ctx context.Context, t DifficultyTable) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO difficulty_table (url, header_url, data_url, name, symbol, fetched_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, t.URL, t.HeaderURL, t.DataURL, t.Name, t.Symbol)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (r *DifficultyTableRepository) UpdateTable(ctx context.Context, t DifficultyTable) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE difficulty_table
		SET header_url = ?, data_url = ?, name = ?, symbol = ?, fetched_at = datetime('now'), updated_at = datetime('now')
		WHERE id = ?
	`, t.HeaderURL, t.DataURL, t.Name, t.Symbol, t.ID)
	return err
}

func (r *DifficultyTableRepository) DeleteTable(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM difficulty_table WHERE id = ?`, id)
	return err
}

func (r *DifficultyTableRepository) ReplaceEntries(ctx context.Context, tableID int, entries []DifficultyTableEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM difficulty_table_entry WHERE table_id = ?`, tableID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO difficulty_table_entry (table_id, md5, level, title, artist, url, url_diff)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, tableID, e.MD5, e.Level, e.Title, e.Artist, e.URL, e.URLDiff); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *DifficultyTableRepository) CountEntries(ctx context.Context, tableID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM difficulty_table_entry WHERE table_id = ?`, tableID).Scan(&count)
	return count, err
}

func (r *DifficultyTableRepository) ListEntries(ctx context.Context, tableID int) ([]DifficultyTableEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT table_id, md5, level, COALESCE(title, ''), COALESCE(artist, ''), COALESCE(url, ''), COALESCE(url_diff, '')
		FROM difficulty_table_entry
		WHERE table_id = ?
		ORDER BY
			CASE WHEN CAST(level AS INTEGER) = 0 AND level != '0' THEN 1 ELSE 0 END,
			CAST(level AS INTEGER),
			level,
			title, artist
	`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DifficultyTableEntry
	for rows.Next() {
		var e DifficultyTableEntry
		if err := rows.Scan(&e.TableID, &e.MD5, &e.Level, &e.Title, &e.Artist, &e.URL, &e.URLDiff); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *DifficultyTableRepository) GetLabelsByMD5(ctx context.Context, md5 string) ([]model.DifficultyLabel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dt.name, dt.symbol, dte.level
		FROM difficulty_table_entry dte
		JOIN difficulty_table dt ON dt.id = dte.table_id
		WHERE dte.md5 = ?
		ORDER BY dt.name
	`, md5)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []model.DifficultyLabel
	for rows.Next() {
		var l model.DifficultyLabel
		if err := rows.Scan(&l.TableName, &l.Symbol, &l.Level); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// GetLabelsByMD5s は複数md5の難易度ラベルをまとめて取得する（N+1回避）
func (r *DifficultyTableRepository) GetLabelsByMD5s(ctx context.Context, md5s []string) (map[string][]model.DifficultyLabel, error) {
	if len(md5s) == 0 {
		return nil, nil
	}

	// プレースホルダ構築
	placeholders := make([]string, len(md5s))
	args := make([]interface{}, len(md5s))
	for i, m := range md5s {
		placeholders[i] = "?"
		args[i] = m
	}

	query := `
		SELECT dte.md5, dt.name, dt.symbol, dte.level
		FROM difficulty_table_entry dte
		JOIN difficulty_table dt ON dt.id = dte.table_id
		WHERE dte.md5 IN (` + joinStrings(placeholders, ",") + `)
		ORDER BY dt.name
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]model.DifficultyLabel)
	for rows.Next() {
		var md5 string
		var l model.DifficultyLabel
		if err := rows.Scan(&md5, &l.TableName, &l.Symbol, &l.Level); err != nil {
			return nil, err
		}
		result[md5] = append(result[md5], l)
	}
	return result, rows.Err()
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
