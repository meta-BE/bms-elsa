package persistence

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

const timeLayout = "2006-01-02 15:04:05"

type ElsaRepository struct {
	db *sql.DB
}

func NewElsaRepository(db *sql.DB) *ElsaRepository {
	return &ElsaRepository{db: db}
}

func (r *ElsaRepository) GetSongMeta(ctx context.Context, folderHash string) (*model.SongMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT folder_hash, release_year, event_name FROM song_meta WHERE folder_hash = ?`,
		folderHash,
	)

	var m model.SongMeta
	if err := row.Scan(&m.FolderHash, &m.ReleaseYear, &m.EventName); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *ElsaRepository) UpsertSongMeta(ctx context.Context, meta model.SongMeta) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, release_year, event_name)
		 VALUES (?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   release_year = excluded.release_year,
		   event_name   = excluded.event_name,
		   updated_at   = datetime('now')`,
		meta.FolderHash, meta.ReleaseYear, meta.EventName,
	)
	return err
}

func (r *ElsaRepository) GetChartMeta(ctx context.Context, md5, sha256 string) (*model.ChartIRMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
		        lr2ir_fetched_at, working_body_url, working_diff_url
		 FROM chart_meta WHERE md5 = ? AND sha256 = ?`,
		md5, sha256,
	)

	var m model.ChartIRMeta
	var tagsStr sql.NullString
	var fetchedAtStr sql.NullString

	if err := row.Scan(
		&m.MD5, &m.SHA256, &tagsStr,
		&m.LR2IRBodyURL, &m.LR2IRDiffURL, &m.LR2IRNotes,
		&fetchedAtStr, &m.WorkingBodyURL, &m.WorkingDiffURL,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if tagsStr.Valid && tagsStr.String != "" {
		m.Tags = strings.Split(tagsStr.String, ",")
	}
	if fetchedAtStr.Valid && fetchedAtStr.String != "" {
		t, err := time.Parse(timeLayout, fetchedAtStr.String)
		if err != nil {
			return nil, err
		}
		m.FetchedAt = &t
	}

	return &m, nil
}

func (r *ElsaRepository) UpsertChartMeta(ctx context.Context, meta model.ChartIRMeta) error {
	tagsStr := strings.Join(meta.Tags, ",")

	var fetchedAtStr *string
	if meta.FetchedAt != nil {
		s := meta.FetchedAt.UTC().Format(timeLayout)
		fetchedAtStr = &s
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chart_meta (md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes, lr2ir_fetched_at, working_body_url, working_diff_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(md5, sha256) DO UPDATE SET
		   lr2ir_tags       = excluded.lr2ir_tags,
		   lr2ir_body_url   = excluded.lr2ir_body_url,
		   lr2ir_diff_url   = excluded.lr2ir_diff_url,
		   lr2ir_notes      = excluded.lr2ir_notes,
		   lr2ir_fetched_at = excluded.lr2ir_fetched_at,
		   working_body_url = excluded.working_body_url,
		   working_diff_url = excluded.working_diff_url,
		   updated_at       = datetime('now')`,
		meta.MD5, meta.SHA256, tagsStr,
		meta.LR2IRBodyURL, meta.LR2IRDiffURL, meta.LR2IRNotes,
		fetchedAtStr, meta.WorkingBodyURL, meta.WorkingDiffURL,
	)
	return err
}

func (r *ElsaRepository) BulkUpsertChartMeta(ctx context.Context, metas []model.ChartIRMeta) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO chart_meta (md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes, lr2ir_fetched_at, working_body_url, working_diff_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(md5, sha256) DO UPDATE SET
		   lr2ir_tags       = excluded.lr2ir_tags,
		   lr2ir_body_url   = excluded.lr2ir_body_url,
		   lr2ir_diff_url   = excluded.lr2ir_diff_url,
		   lr2ir_notes      = excluded.lr2ir_notes,
		   lr2ir_fetched_at = excluded.lr2ir_fetched_at,
		   working_body_url = excluded.working_body_url,
		   working_diff_url = excluded.working_diff_url,
		   updated_at       = datetime('now')`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, meta := range metas {
		tagsStr := strings.Join(meta.Tags, ",")
		var fetchedAtStr *string
		if meta.FetchedAt != nil {
			s := meta.FetchedAt.UTC().Format(timeLayout)
			fetchedAtStr = &s
		}

		if _, err := stmt.ExecContext(ctx,
			meta.MD5, meta.SHA256, tagsStr,
			meta.LR2IRBodyURL, meta.LR2IRDiffURL, meta.LR2IRNotes,
			fetchedAtStr, meta.WorkingBodyURL, meta.WorkingDiffURL,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
