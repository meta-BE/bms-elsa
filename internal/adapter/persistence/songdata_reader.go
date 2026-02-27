package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

var _ model.SongRepository = (*SongdataReader)(nil)

const defaultPageSize = 50

// SongdataReader はsongdata.db（ATTACHされた）から楽曲を読み取る
type SongdataReader struct {
	db       *sql.DB // elsa.db接続（songdata.dbがATTACH済み）
	metaRepo *ElsaRepository
}

func NewSongdataReader(db *sql.DB, metaRepo *ElsaRepository) *SongdataReader {
	return &SongdataReader{db: db, metaRepo: metaRepo}
}

// AttachSongdata はsongdata.dbをelsa.db接続にATTACHする
func AttachSongdata(db *sql.DB, songdataPath string) error {
	_, err := db.Exec("ATTACH DATABASE ? AS songdata", songdataPath)
	return err
}

// sortColumn はSortByフィールド名をSQL列名に変換する
func sortColumn(sortBy string) string {
	switch sortBy {
	case "artist":
		return "sg.artist"
	case "genre":
		return "sg.genre"
	case "bpm":
		return "sg.max_bpm"
	case "chartCount":
		return "sg.chart_count"
	case "eventName":
		return "sm.event_name"
	case "releaseYear":
		return "sm.release_year"
	default:
		return "sg.title"
	}
}

func (r *SongdataReader) ListSongs(ctx context.Context, opts model.ListOptions) ([]model.Song, int, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	col := sortColumn(opts.SortBy)
	dir := "ASC"
	if opts.SortDesc {
		dir = "DESC"
	}
	// SQLインジェクション防止: sortColumnは固定文字列のみ返す
	orderClause := fmt.Sprintf("%s %s", col, dir)

	query := fmt.Sprintf(`
		WITH song_groups AS (
			SELECT
				s.folder,
				COALESCE(MIN(CASE WHEN s.title != '' THEN s.title END), '') AS title,
				COALESCE(MIN(CASE WHEN s.artist != '' THEN s.artist END), '') AS artist,
				COALESCE(MIN(CASE WHEN s.genre != '' THEN s.genre END), '') AS genre,
				MIN(s.minbpm) AS min_bpm,
				MAX(s.maxbpm) AS max_bpm,
				COUNT(*) AS chart_count
			FROM songdata.song s
			GROUP BY s.folder
		)
		SELECT
			sg.folder, sg.title, sg.artist, sg.genre,
			sg.min_bpm, sg.max_bpm, sg.chart_count,
			sm.release_year, sm.event_name,
			EXISTS(
				SELECT 1 FROM songdata.song ss
				INNER JOIN main.chart_meta cm ON cm.md5 = ss.md5 AND cm.sha256 = ss.sha256
				WHERE ss.folder = sg.folder
			) AS has_ir_meta,
			COUNT(*) OVER() AS total_count
		FROM song_groups sg
		LEFT JOIN main.song_meta sm ON sm.folder_hash = sg.folder
		WHERE (? = '' OR sg.title LIKE '%%' || ? || '%%'
		       OR sg.artist LIKE '%%' || ? || '%%'
		       OR sg.genre LIKE '%%' || ? || '%%')
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, orderClause)

	search := opts.Search
	rows, err := r.db.QueryContext(ctx, query,
		search, search, search, search,
		pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("ListSongs query: %w", err)
	}
	defer rows.Close()

	var songs []model.Song
	var totalCount int

	for rows.Next() {
		var s model.Song
		var releaseYear sql.NullInt64
		var eventName sql.NullString
		var total int

		if err := rows.Scan(
			&s.FolderHash, &s.Title, &s.Artist, &s.Genre,
			&s.MinBPM, &s.MaxBPM, &s.ChartCount,
			&releaseYear, &eventName,
			&s.HasIRMeta,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("ListSongs scan: %w", err)
		}

		if releaseYear.Valid {
			v := int(releaseYear.Int64)
			s.ReleaseYear = &v
		}
		if eventName.Valid {
			s.EventName = &eventName.String
		}

		totalCount = total
		songs = append(songs, s)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ListSongs rows: %w", err)
	}

	return songs, totalCount, nil
}

func (r *SongdataReader) GetSongByFolder(ctx context.Context, folderHash string) (*model.Song, error) {
	// 全譜面を取得
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			s.md5, s.sha256, s.title, s.artist, COALESCE(s.subartist, ''),
			s.genre, s.mode, s.difficulty, s.level,
			s.minbpm, s.maxbpm, s.path
		FROM songdata.song s
		WHERE s.folder = ?
		ORDER BY s.difficulty, s.level
	`, folderHash)
	if err != nil {
		return nil, fmt.Errorf("GetSongByFolder query: %w", err)
	}
	defer rows.Close()

	var charts []model.Chart
	for rows.Next() {
		var c model.Chart
		if err := rows.Scan(
			&c.MD5, &c.SHA256, &c.Title, &c.Artist, &c.SubArtist,
			&c.Genre, &c.Mode, &c.Difficulty, &c.Level,
			&c.MinBPM, &c.MaxBPM, &c.Path,
		); err != nil {
			return nil, fmt.Errorf("GetSongByFolder scan chart: %w", err)
		}
		charts = append(charts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetSongByFolder rows: %w", err)
	}

	// rowsを閉じた後にchart_metaを付与（コネクション競合を回避）
	for i := range charts {
		irMeta, err := r.metaRepo.GetChartMeta(ctx, charts[i].MD5, charts[i].SHA256)
		if err != nil {
			return nil, fmt.Errorf("GetSongByFolder GetChartMeta: %w", err)
		}
		charts[i].IRMeta = irMeta
	}

	if len(charts) == 0 {
		return nil, nil
	}

	// 代表譜面から楽曲情報を構築
	rep := charts[0]
	song := &model.Song{
		FolderHash: folderHash,
		Title:      rep.Title,
		Artist:     rep.Artist,
		Genre:      rep.Genre,
		Charts:     charts,
	}

	// BPMはフォルダ内全譜面の最小/最大
	song.MinBPM = rep.MinBPM
	song.MaxBPM = rep.MaxBPM
	for _, c := range charts[1:] {
		if c.MinBPM < song.MinBPM {
			song.MinBPM = c.MinBPM
		}
		if c.MaxBPM > song.MaxBPM {
			song.MaxBPM = c.MaxBPM
		}
	}

	// song_metaがあれば付与
	meta, err := r.metaRepo.GetSongMeta(ctx, folderHash)
	if err != nil {
		return nil, fmt.Errorf("GetSongByFolder GetSongMeta: %w", err)
	}
	if meta != nil {
		song.ReleaseYear = meta.ReleaseYear
		song.EventName = meta.EventName
	}

	return song, nil
}
