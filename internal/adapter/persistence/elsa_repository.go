package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
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

func (r *ElsaRepository) GetChartMeta(ctx context.Context, md5 string) (*model.ChartIRMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
		        lr2ir_fetched_at, working_body_url, working_diff_url
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

func (r *ElsaRepository) ListEventMappings(ctx context.Context) ([]model.EventMapping, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, url_pattern, event_name, release_year FROM event_mapping ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []model.EventMapping
	for rows.Next() {
		var m model.EventMapping
		if err := rows.Scan(&m.ID, &m.URLPattern, &m.EventName, &m.ReleaseYear); err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

func (r *ElsaRepository) UpsertEventMapping(ctx context.Context, m model.EventMapping) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO event_mapping (url_pattern, event_name, release_year)
		 VALUES (?, ?, ?)
		 ON CONFLICT(url_pattern) DO UPDATE SET
		   event_name   = excluded.event_name,
		   release_year = excluded.release_year`,
		m.URLPattern, m.EventName, m.ReleaseYear,
	)
	return err
}

func (r *ElsaRepository) DeleteEventMapping(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM event_mapping WHERE id = ?`,
		id,
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

// ChartScanTarget はMinHashスキャン対象の譜面情報
type ChartScanTarget struct {
	MD5  string
	Path string
}

// ListChartsWithoutMinhash はwav_minhashが未計算の譜面リストを返す
func (r *ElsaRepository) ListChartsWithoutMinhash(ctx context.Context) ([]ChartScanTarget, error) {
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

	var targets []ChartScanTarget
	for rows.Next() {
		var t ChartScanTarget
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

func (r *ElsaRepository) ListUnsetSongsWithIRURLs(ctx context.Context) ([]model.SongIRURLs, error) {
	// songdata.db（sdスキーマ）とelsa.dbのクロスDB JOIN
	// song_metaにレコードがない or (release_year IS NULL AND event_name IS NULL)の曲が対象
	rows, err := r.db.QueryContext(ctx, `
		WITH song_groups AS (
			SELECT
				s.folder AS folder_hash,
				MIN(s.title) AS title,
				MIN(s.artist) AS artist,
				MIN(s.genre) AS genre,
				COUNT(*) AS chart_count
			FROM songdata.song s
			GROUP BY s.folder
		),
		ir_urls AS (
			SELECT
				s.folder AS folder_hash,
				cm.lr2ir_body_url,
				CASE WHEN cm.lr2ir_fetched_at IS NOT NULL THEN 1 ELSE 0 END AS has_ir
			FROM songdata.song s
			LEFT JOIN chart_meta cm ON s.md5 = cm.md5
		)
		SELECT
			sg.folder_hash, sg.title, sg.artist, sg.genre, sg.chart_count,
			GROUP_CONCAT(DISTINCT iu.lr2ir_body_url) AS body_urls,
			COALESCE(SUM(iu.has_ir), 0) AS ir_count
		FROM song_groups sg
		LEFT JOIN song_meta sm ON sg.folder_hash = sm.folder_hash
		LEFT JOIN ir_urls iu ON sg.folder_hash = iu.folder_hash
		WHERE sm.folder_hash IS NULL
		   OR (sm.release_year IS NULL AND sm.event_name IS NULL)
		GROUP BY sg.folder_hash
		ORDER BY sg.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.SongIRURLs
	for rows.Next() {
		var r model.SongIRURLs
		var bodyURLsStr sql.NullString
		if err := rows.Scan(&r.FolderHash, &r.Title, &r.Artist, &r.Genre, &r.ChartCount, &bodyURLsStr, &r.IRCount); err != nil {
			return nil, err
		}
		if bodyURLsStr.Valid && bodyURLsStr.String != "" {
			for _, u := range strings.Split(bodyURLsStr.String, ",") {
				if u != "" {
					r.BodyURLs = append(r.BodyURLs, u)
				}
			}
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// MinHashMatch はMinHash類似度検索の結果
type MinHashMatch struct {
	MD5        string
	FolderPath string
	Similarity float64
}

// FindMostSimilarByMinHash はクエリminhashに最も類似するレコードを返す（Go全件スキャン方式）
func (r *ElsaRepository) FindMostSimilarByMinHash(ctx context.Context, queryMinhash []byte, threshold float64) (*MinHashMatch, error) {
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

	var best *MinHashMatch
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
			best = &MinHashMatch{
				MD5:        md5,
				FolderPath: filepath.Dir(path),
				Similarity: sim,
			}
		}
	}
	return best, rows.Err()
}
