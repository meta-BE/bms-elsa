package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

var _ model.BMSSearchRepository = (*BMSSearchRepository)(nil)

type BMSSearchRepository struct {
	db *sql.DB
}

func NewBMSSearchRepository(db *sql.DB) *BMSSearchRepository {
	return &BMSSearchRepository{db: db}
}

func (r *BMSSearchRepository) GetLinkByMD5(ctx context.Context, md5 string) (*model.BMSSearchLink, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, bms_id, source, resolved_at FROM bmssearch_bms_id_md5 WHERE md5 = ?`,
		md5,
	)
	var l model.BMSSearchLink
	var src string
	var resolvedUnix int64
	if err := row.Scan(&l.MD5, &l.BMSID, &src, &resolvedUnix); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	l.Source = model.BMSSearchSource(src)
	l.ResolvedAt = time.Unix(resolvedUnix, 0)
	return &l, nil
}

func (r *BMSSearchRepository) UpsertLinks(ctx context.Context, links []model.BMSSearchLink) error {
	if len(links) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO bmssearch_bms_id_md5 (md5, bms_id, source, resolved_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(md5) DO UPDATE SET
		   bms_id      = excluded.bms_id,
		   source      = excluded.source,
		   resolved_at = excluded.resolved_at`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, l := range links {
		if _, err := stmt.ExecContext(ctx, l.MD5, l.BMSID, string(l.Source), l.ResolvedAt.Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *BMSSearchRepository) DeleteLinkByMD5(ctx context.Context, md5 string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bmssearch_bms_id_md5 WHERE md5 = ?`, md5)
	return err
}

func (r *BMSSearchRepository) DeleteLinksByMD5s(ctx context.Context, md5s []string) error {
	if len(md5s) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(md5s))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(md5s))
	for i, m := range md5s {
		args[i] = m
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM bmssearch_bms_id_md5 WHERE md5 IN (`+placeholders+`)`,
		args...,
	)
	return err
}

func (r *BMSSearchRepository) GetBMSByID(ctx context.Context, bmsID string) (*model.BMSSearchBMS, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT bms_id, title, artist, subartist, genre,
		       exhibition_id, exhibition_name, published_at,
		       downloads_json, previews_json, related_links_json, fetched_at
		FROM bmssearch_bms WHERE bms_id = ?`, bmsID)
	var b model.BMSSearchBMS
	var exID sql.NullString
	var dlsJSON, prevsJSON, relJSON string
	var fetchedUnix int64
	if err := row.Scan(
		&b.BMSID, &b.Title, &b.Artist, &b.SubArtist, &b.Genre,
		&exID, &b.ExhibitionName, &b.PublishedAt,
		&dlsJSON, &prevsJSON, &relJSON, &fetchedUnix,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if exID.Valid {
		s := exID.String
		b.ExhibitionID = &s
	}
	b.FetchedAt = time.Unix(fetchedUnix, 0)
	if err := json.Unmarshal([]byte(dlsJSON), &b.Downloads); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(prevsJSON), &b.Previews); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(relJSON), &b.RelatedLinks); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BMSSearchRepository) UpsertBMS(ctx context.Context, bms model.BMSSearchBMS) error {
	dlsJSON, err := json.Marshal(emptyIfNilURLEntries(bms.Downloads))
	if err != nil {
		return err
	}
	prevsJSON, err := json.Marshal(emptyIfNilPreviews(bms.Previews))
	if err != nil {
		return err
	}
	relJSON, err := json.Marshal(emptyIfNilURLEntries(bms.RelatedLinks))
	if err != nil {
		return err
	}
	var exIDParam any
	if bms.ExhibitionID != nil {
		exIDParam = *bms.ExhibitionID
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO bmssearch_bms
		  (bms_id, title, artist, subartist, genre, exhibition_id, exhibition_name,
		   published_at, downloads_json, previews_json, related_links_json, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bms_id) DO UPDATE SET
		  title              = excluded.title,
		  artist             = excluded.artist,
		  subartist          = excluded.subartist,
		  genre              = excluded.genre,
		  exhibition_id      = excluded.exhibition_id,
		  exhibition_name    = excluded.exhibition_name,
		  published_at       = excluded.published_at,
		  downloads_json     = excluded.downloads_json,
		  previews_json      = excluded.previews_json,
		  related_links_json = excluded.related_links_json,
		  fetched_at         = excluded.fetched_at`,
		bms.BMSID, bms.Title, bms.Artist, bms.SubArtist, bms.Genre,
		exIDParam, bms.ExhibitionName, bms.PublishedAt,
		string(dlsJSON), string(prevsJSON), string(relJSON), bms.FetchedAt.Unix(),
	)
	return err
}

func emptyIfNilURLEntries(v []model.BMSSearchURLEntry) []model.BMSSearchURLEntry {
	if v == nil {
		return []model.BMSSearchURLEntry{}
	}
	return v
}

func emptyIfNilPreviews(v []model.BMSSearchPreview) []model.BMSSearchPreview {
	if v == nil {
		return []model.BMSSearchPreview{}
	}
	return v
}
