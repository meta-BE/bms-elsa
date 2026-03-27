package persistence

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

var _ model.MetaRepository = (*ElsaRepository)(nil)

const timeLayout = "2006-01-02 15:04:05"

type ElsaRepository struct {
	db *sql.DB
}

func NewElsaRepository(db *sql.DB) *ElsaRepository {
	return &ElsaRepository{db: db}
}

func (r *ElsaRepository) GetSongMeta(ctx context.Context, folderHash string) (*model.SongMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT folder_hash, release_year, event_id, bms_search_id FROM song_meta WHERE folder_hash = ?`,
		folderHash,
	)

	var m model.SongMeta
	if err := row.Scan(&m.FolderHash, &m.ReleaseYear, &m.EventID, &m.BMSSearchID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *ElsaRepository) UpsertSongMeta(ctx context.Context, meta model.SongMeta) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, release_year, event_id, bms_search_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   release_year  = excluded.release_year,
		   event_id      = excluded.event_id,
		   bms_search_id = excluded.bms_search_id,
		   updated_at    = datetime('now')`,
		meta.FolderHash, meta.ReleaseYear, meta.EventID, meta.BMSSearchID,
	)
	return err
}

func (r *ElsaRepository) GetChartMeta(ctx context.Context, md5 string) (*model.ChartIRMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, sha256, lr2ir_tags,
		        COALESCE(lr2ir_body_url, ''), COALESCE(lr2ir_diff_url, ''), COALESCE(lr2ir_notes, ''),
		        lr2ir_fetched_at, COALESCE(working_body_url, ''), COALESCE(working_diff_url, '')
		 FROM chart_meta WHERE md5 = ?`,
		md5,
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
		t, err := time.ParseInLocation(timeLayout, fetchedAtStr.String, time.UTC)
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
		 ON CONFLICT(md5) DO UPDATE SET
		   sha256           = COALESCE(NULLIF(excluded.sha256, ''), chart_meta.sha256),
		   lr2ir_tags       = excluded.lr2ir_tags,
		   lr2ir_body_url   = excluded.lr2ir_body_url,
		   lr2ir_diff_url   = excluded.lr2ir_diff_url,
		   lr2ir_notes      = excluded.lr2ir_notes,
		   lr2ir_fetched_at = excluded.lr2ir_fetched_at,
		   working_body_url = COALESCE(NULLIF(excluded.working_body_url, ''), chart_meta.working_body_url),
		   working_diff_url = COALESCE(NULLIF(excluded.working_diff_url, ''), chart_meta.working_diff_url),
		   updated_at       = datetime('now')`,
		meta.MD5, meta.SHA256, tagsStr,
		meta.LR2IRBodyURL, meta.LR2IRDiffURL, meta.LR2IRNotes,
		fetchedAtStr, meta.WorkingBodyURL, meta.WorkingDiffURL,
	)
	return err
}

func (r *ElsaRepository) UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE chart_meta SET
			working_body_url = ?,
			working_diff_url = ?,
			updated_at = datetime('now')
		 WHERE md5 = ?`,
		workingBodyURL, workingDiffURL, md5,
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
		 ON CONFLICT(md5) DO UPDATE SET
		   sha256           = COALESCE(NULLIF(excluded.sha256, ''), chart_meta.sha256),
		   lr2ir_tags       = excluded.lr2ir_tags,
		   lr2ir_body_url   = excluded.lr2ir_body_url,
		   lr2ir_diff_url   = excluded.lr2ir_diff_url,
		   lr2ir_notes      = excluded.lr2ir_notes,
		   lr2ir_fetched_at = excluded.lr2ir_fetched_at,
		   working_body_url = COALESCE(NULLIF(excluded.working_body_url, ''), chart_meta.working_body_url),
		   working_diff_url = COALESCE(NULLIF(excluded.working_diff_url, ''), chart_meta.working_diff_url),
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

func (r *ElsaRepository) ListEvents(ctx context.Context) ([]model.Event, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, bms_search_id, name, short_name, release_year, url FROM event ORDER BY release_year DESC, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.Event
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(&e.ID, &e.BMSSearchID, &e.Name, &e.ShortName, &e.ReleaseYear, &e.URL); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *ElsaRepository) GetEventByBMSSearchID(ctx context.Context, bmsSearchID string) (*model.Event, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, bms_search_id, name, short_name, release_year, url FROM event WHERE bms_search_id = ?`,
		bmsSearchID,
	)
	var e model.Event
	if err := row.Scan(&e.ID, &e.BMSSearchID, &e.Name, &e.ShortName, &e.ReleaseYear, &e.URL); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (r *ElsaRepository) UpsertEventByBMSSearchID(ctx context.Context, e model.Event) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO event (bms_search_id, name, short_name, release_year, url)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(bms_search_id) DO UPDATE SET
		   name         = excluded.name,
		   short_name   = excluded.short_name,
		   release_year = excluded.release_year,
		   url          = CASE WHEN excluded.url != '' THEN excluded.url ELSE event.url END,
		   updated_at   = datetime('now')`,
		e.BMSSearchID, e.Name, e.ShortName, e.ReleaseYear, e.URL,
	)
	return err
}

func (r *ElsaRepository) UpdateEventShortName(ctx context.Context, id int, shortName string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE event SET short_name = ?, updated_at = datetime('now') WHERE id = ?`,
		shortName, id,
	)
	return err
}

func (r *ElsaRepository) UpdateEventReleaseYear(ctx context.Context, id int, releaseYear int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE event SET release_year = ?, updated_at = datetime('now') WHERE id = ?`,
		releaseYear, id,
	)
	return err
}

func (r *ElsaRepository) ListFoldersWithoutEvent(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.folder
		FROM songdata.song s
		LEFT JOIN song_meta sm ON s.folder = sm.folder_hash
		WHERE sm.folder_hash IS NULL OR (sm.event_id IS NULL AND sm.bms_search_id IS NULL)
		ORDER BY s.folder`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (r *ElsaRepository) UpdateSongMetaEvent(ctx context.Context, folderHash string, eventID string, bmsSearchID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, event_id, bms_search_id)
		 VALUES (?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   event_id      = excluded.event_id,
		   bms_search_id = excluded.bms_search_id,
		   updated_at    = datetime('now')`,
		folderHash, eventID, bmsSearchID,
	)
	return err
}

func (r *ElsaRepository) ListUnfetchedChartMD5s(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.md5
		FROM songdata.song s
		LEFT JOIN chart_meta cm ON s.md5 = cm.md5
		WHERE cm.id IS NULL OR cm.lr2ir_fetched_at IS NULL
		ORDER BY s.md5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var md5s []string
	for rows.Next() {
		var md5 string
		if err := rows.Scan(&md5); err != nil {
			return nil, err
		}
		md5s = append(md5s, md5)
	}
	return md5s, rows.Err()
}

func (r *ElsaRepository) ListUnfetchedDTEntryMD5s(ctx context.Context, tableID int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dte.md5
		FROM difficulty_table_entry dte
		LEFT JOIN chart_meta cm ON dte.md5 = cm.md5
		WHERE dte.table_id = ? AND (cm.id IS NULL OR cm.lr2ir_fetched_at IS NULL)
		ORDER BY dte.md5`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var md5s []string
	for rows.Next() {
		var md5 string
		if err := rows.Scan(&md5); err != nil {
			return nil, err
		}
		md5s = append(md5s, md5)
	}
	return md5s, rows.Err()
}

func (r *ElsaRepository) ListRewriteRules(ctx context.Context) ([]model.RewriteRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, rule_type, pattern, replacement, priority FROM url_rewrite_rule ORDER BY priority DESC, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.RewriteRule
	for rows.Next() {
		var rule model.RewriteRule
		if err := rows.Scan(&rule.ID, &rule.RuleType, &rule.Pattern, &rule.Replacement, &rule.Priority); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *ElsaRepository) UpsertRewriteRule(ctx context.Context, rule model.RewriteRule) error {
	if rule.ID > 0 {
		_, err := r.db.ExecContext(ctx,
			`UPDATE url_rewrite_rule SET rule_type = ?, pattern = ?, replacement = ?, priority = ?, updated_at = datetime('now') WHERE id = ?`,
			rule.RuleType, rule.Pattern, rule.Replacement, rule.Priority, rule.ID,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO url_rewrite_rule (rule_type, pattern, replacement, priority) VALUES (?, ?, ?, ?)
		 ON CONFLICT(rule_type, pattern) DO UPDATE SET replacement = excluded.replacement, priority = excluded.priority, updated_at = datetime('now')`,
		rule.RuleType, rule.Pattern, rule.Replacement, rule.Priority,
	)
	return err
}

func (r *ElsaRepository) DeleteRewriteRule(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM url_rewrite_rule WHERE id = ?`, id)
	return err
}

func (r *ElsaRepository) ListChartsForWorkingURLInference(ctx context.Context) ([]model.ChartIRMeta, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT md5, sha256, lr2ir_body_url, lr2ir_diff_url
		 FROM chart_meta
		 WHERE (working_body_url IS NULL OR working_body_url = '')
		   AND (working_diff_url IS NULL OR working_diff_url = '')
		   AND (lr2ir_body_url IS NOT NULL AND lr2ir_body_url != ''
		        OR lr2ir_diff_url IS NOT NULL AND lr2ir_diff_url != '')`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var charts []model.ChartIRMeta
	for rows.Next() {
		var c model.ChartIRMeta
		var bodyURL, diffURL sql.NullString
		if err := rows.Scan(&c.MD5, &c.SHA256, &bodyURL, &diffURL); err != nil {
			return nil, err
		}
		c.LR2IRBodyURL = bodyURL.String
		c.LR2IRDiffURL = diffURL.String
		charts = append(charts, c)
	}
	return charts, rows.Err()
}

// ListChartsWithoutMinhash はwav_minhashが未計算の譜面リストを返す
func (r *ElsaRepository) ListChartsWithoutMinhash(ctx context.Context) ([]model.ChartScanTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.md5, s.path
		FROM songdata.song s
		LEFT JOIN chart_meta cm ON s.md5 = cm.md5
		WHERE s.md5 != ''
		  AND (cm.id IS NULL OR cm.wav_minhash IS NULL)
		ORDER BY s.md5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []model.ChartScanTarget
	for rows.Next() {
		var t model.ChartScanTarget
		if err := rows.Scan(&t.MD5, &t.Path); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// UpdateWavMinhash はchart_metaのwav_minhashを更新する。レコードが存在しない場合はINSERTする。
func (r *ElsaRepository) UpdateWavMinhash(ctx context.Context, md5 string, minhash []byte) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chart_meta (md5, wav_minhash)
		 VALUES (?, ?)
		 ON CONFLICT(md5) DO UPDATE SET
		   wav_minhash = excluded.wav_minhash,
		   updated_at  = datetime('now')`,
		md5, minhash,
	)
	return err
}

// FindMostSimilarByMinHash はクエリminhashに最も類似するレコードを返す（Go全件スキャン方式）
func (r *ElsaRepository) FindMostSimilarByMinHash(ctx context.Context, queryMinhash []byte, threshold float64) (*model.MinHashMatch, error) {
	query, err := bms.MinHashFromBytes(queryMinhash)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT cm.md5, cm.wav_minhash, s.path
		 FROM chart_meta cm
		 JOIN songdata.song s ON cm.md5 = s.md5
		 WHERE cm.wav_minhash IS NOT NULL
		 GROUP BY cm.wav_minhash`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var best *model.MinHashMatch
	for rows.Next() {
		var md5 string
		var blob []byte
		var path string
		if err := rows.Scan(&md5, &blob, &path); err != nil {
			return nil, err
		}
		sig, err := bms.MinHashFromBytes(blob)
		if err != nil {
			continue
		}
		sim := query.Similarity(sig)
		if sim >= threshold && (best == nil || sim > best.Similarity) {
			best = &model.MinHashMatch{
				MD5:        md5,
				FolderPath: ParentDirOf(path),
				Similarity: sim,
			}
		}
	}
	return best, rows.Err()
}
