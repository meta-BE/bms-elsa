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
	db       *sql.DB
	metaRepo *ElsaRepository
	dtRepo   *DifficultyTableRepository
}

func NewSongdataReader(db *sql.DB, metaRepo *ElsaRepository, dtRepo *DifficultyTableRepository) *SongdataReader {
	return &SongdataReader{db: db, metaRepo: metaRepo, dtRepo: dtRepo}
}

// AttachSongdata はsongdata.dbをelsa.db接続にATTACHする。
// ATTACH後、ListSongsのEXISTS相関サブクエリ高速化のためfolderインデックスを作成する。
func AttachSongdata(db *sql.DB, songdataPath string) error {
	_, err := db.Exec("ATTACH DATABASE ? AS songdata", songdataPath)
	if err != nil {
		return err
	}
	// folderカラムにインデックスがないとListSongsのEXISTS相関サブクエリが
	// 各グループごとにフルテーブルスキャンとなり極端に遅くなる（2.4秒→0.02秒）
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS songdata.idx_song_folder ON song(folder)")
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
		WITH
		bpm_mode AS (
			SELECT folder, minbpm AS min_bpm, maxbpm AS max_bpm
			FROM (
				SELECT folder, minbpm, maxbpm,
					ROW_NUMBER() OVER (
						PARTITION BY folder ORDER BY COUNT(*) DESC, minbpm
					) AS rn
				FROM songdata.song
				GROUP BY folder, minbpm, maxbpm
			)
			WHERE rn = 1
		),
		song_groups AS (
			SELECT
				s.folder,
				COALESCE(MIN(CASE WHEN s.title != '' THEN s.title END), '') AS title,
				COALESCE(MIN(CASE WHEN s.artist != '' THEN s.artist END), '') AS artist,
				COALESCE(MIN(CASE WHEN s.genre != '' THEN s.genre END), '') AS genre,
				MIN(bm.min_bpm) AS min_bpm,
				MIN(bm.max_bpm) AS max_bpm,
				COUNT(*) AS chart_count
			FROM songdata.song s
			JOIN bpm_mode bm ON bm.folder = s.folder
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

// ListAllSongs は楽曲一覧を全件取得する（フロントエンドフィルタ用）
func (r *SongdataReader) ListAllSongs(ctx context.Context) ([]model.Song, error) {
	query := `
		WITH
		bpm_mode AS (
			SELECT folder, minbpm AS min_bpm, maxbpm AS max_bpm
			FROM (
				SELECT folder, minbpm, maxbpm,
					ROW_NUMBER() OVER (
						PARTITION BY folder ORDER BY COUNT(*) DESC, minbpm
					) AS rn
				FROM songdata.song
				GROUP BY folder, minbpm, maxbpm
			)
			WHERE rn = 1
		),
		song_groups AS (
			SELECT
				s.folder,
				COALESCE(MIN(CASE WHEN s.title != '' THEN s.title END), '') AS title,
				COALESCE(MIN(CASE WHEN s.artist != '' THEN s.artist END), '') AS artist,
				COALESCE(MIN(CASE WHEN s.genre != '' THEN s.genre END), '') AS genre,
				MIN(bm.min_bpm) AS min_bpm,
				MIN(bm.max_bpm) AS max_bpm,
				COUNT(*) AS chart_count
			FROM songdata.song s
			JOIN bpm_mode bm ON bm.folder = s.folder
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
			) AS has_ir_meta
		FROM song_groups sg
		LEFT JOIN main.song_meta sm ON sm.folder_hash = sg.folder
		ORDER BY sg.title ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListAllSongs query: %w", err)
	}
	defer rows.Close()

	var songs []model.Song
	for rows.Next() {
		var s model.Song
		var releaseYear sql.NullInt64
		var eventName sql.NullString

		if err := rows.Scan(
			&s.FolderHash, &s.Title, &s.Artist, &s.Genre,
			&s.MinBPM, &s.MaxBPM, &s.ChartCount,
			&releaseYear, &eventName,
			&s.HasIRMeta,
		); err != nil {
			return nil, fmt.Errorf("ListAllSongs scan: %w", err)
		}

		if releaseYear.Valid {
			v := int(releaseYear.Int64)
			s.ReleaseYear = &v
		}
		if eventName.Valid {
			s.EventName = &eventName.String
		}
		songs = append(songs, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListAllSongs rows: %w", err)
	}

	return songs, nil
}

func (r *SongdataReader) GetSongByFolder(ctx context.Context, folderHash string) (*model.Song, error) {
	// 全譜面を取得
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			s.md5, s.sha256, s.title, COALESCE(s.subtitle, ''), s.artist, COALESCE(s.subartist, ''),
			s.genre, s.mode, s.difficulty, s.level,
			s.minbpm, s.maxbpm, s.path, s.notes
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
			&c.MD5, &c.SHA256, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist,
			&c.Genre, &c.Mode, &c.Difficulty, &c.Level,
			&c.MinBPM, &c.MaxBPM, &c.Path, &c.Notes,
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
		irMeta, err := r.metaRepo.GetChartMeta(ctx, charts[i].MD5)
		if err != nil {
			return nil, fmt.Errorf("GetSongByFolder GetChartMeta: %w", err)
		}
		charts[i].IRMeta = irMeta
	}

	// 難易度ラベルを一括取得（N+1回避）
	md5s := make([]string, len(charts))
	for i, c := range charts {
		md5s[i] = c.MD5
	}
	labelsMap, err := r.dtRepo.GetLabelsByMD5s(ctx, md5s)
	if err != nil {
		return nil, fmt.Errorf("GetSongByFolder GetLabelsByMD5s: %w", err)
	}
	for i := range charts {
		charts[i].DifficultyLabels = labelsMap[charts[i].MD5]
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

// CountChartsByMD5s は指定md5群がsongdata.db内に何件存在するかを返す
func (r *SongdataReader) CountChartsByMD5s(ctx context.Context, md5s []string) (map[string]int, error) {
	if len(md5s) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(md5s))
	args := make([]interface{}, len(md5s))
	for i, m := range md5s {
		placeholders[i] = "?"
		args[i] = m
	}

	query := `
		SELECT md5, COUNT(*) FROM songdata.song
		WHERE md5 IN (` + joinStrings(placeholders, ",") + `)
		GROUP BY md5
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("CountChartsByMD5s: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var md5 string
		var count int
		if err := rows.Scan(&md5, &count); err != nil {
			return nil, err
		}
		result[md5] = count
	}
	return result, rows.Err()
}

// ChartListItem は譜面一覧用の軽量モデル
type ChartListItem struct {
	MD5         string
	Title       string
	Subtitle    string
	Artist      string
	SubArtist   string
	Genre       string
	MinBPM      float64
	MaxBPM      float64
	Difficulty  int
	Notes       int
	EventName   *string
	ReleaseYear *int
	HasIRMeta   bool
}

// ListAllCharts はsongdata.dbの全譜面を個別に取得する
func (r *SongdataReader) ListAllCharts(ctx context.Context) ([]ChartListItem, error) {
	query := `
		SELECT
			s.md5,
			s.title,
			COALESCE(s.subtitle, ''),
			s.artist,
			COALESCE(s.subartist, ''),
			s.genre,
			s.minbpm,
			s.maxbpm,
			s.difficulty,
			s.notes,
			sm.event_name,
			sm.release_year,
			EXISTS(
				SELECT 1 FROM main.chart_meta cm
				WHERE cm.md5 = s.md5 AND cm.sha256 = s.sha256
			) AS has_ir_meta
		FROM songdata.song s
		LEFT JOIN main.song_meta sm ON sm.folder_hash = s.folder
		WHERE s.md5 != ''
		ORDER BY s.title ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListAllCharts: %w", err)
	}
	defer rows.Close()

	var charts []ChartListItem
	for rows.Next() {
		var c ChartListItem
		var eventName sql.NullString
		var releaseYear sql.NullInt64
		if err := rows.Scan(
			&c.MD5, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist, &c.Genre,
			&c.MinBPM, &c.MaxBPM, &c.Difficulty, &c.Notes,
			&eventName, &releaseYear, &c.HasIRMeta,
		); err != nil {
			return nil, fmt.Errorf("ListAllCharts scan: %w", err)
		}
		if eventName.Valid {
			c.EventName = &eventName.String
		}
		if releaseYear.Valid {
			v := int(releaseYear.Int64)
			c.ReleaseYear = &v
		}
		charts = append(charts, c)
	}
	return charts, rows.Err()
}

// SongGroup は重複スキャン用のfolder単位の楽曲情報
type SongGroup struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	ChartCount int
	Path       string // 代表パス（フォルダまで）
}

// ListSongGroupsForDuplicateScan はfolder単位で楽曲グループを返す（重複スキャン用）
func (r *SongdataReader) ListSongGroupsForDuplicateScan(ctx context.Context) ([]SongGroup, error) {
	query := `
		SELECT
			s.folder,
			s.title,
			s.artist,
			s.genre,
			MIN(s.minbpm) AS minbpm,
			MAX(s.maxbpm) AS maxbpm,
			COUNT(*) AS chart_count,
			MIN(s.path) AS path
		FROM songdata.song s
		WHERE s.md5 IS NOT NULL AND s.md5 != ''
		GROUP BY s.folder
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListSongGroupsForDuplicateScan: %w", err)
	}
	defer rows.Close()

	var groups []SongGroup
	for rows.Next() {
		var g SongGroup
		if err := rows.Scan(&g.FolderHash, &g.Title, &g.Artist, &g.Genre,
			&g.MinBPM, &g.MaxBPM, &g.ChartCount, &g.Path); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetChartByMD5 はmd5で譜面を1件取得し、IRメタ・難易度ラベルを付与して返す
func (r *SongdataReader) GetChartByMD5(ctx context.Context, md5 string) (*model.Chart, error) {
	var c model.Chart
	err := r.db.QueryRowContext(ctx, `
		SELECT md5, sha256, title, COALESCE(subtitle, ''), artist, COALESCE(subartist, ''),
			genre, mode, difficulty, level, minbpm, maxbpm, path, notes
		FROM songdata.song
		WHERE md5 = ?
		LIMIT 1
	`, md5).Scan(
		&c.MD5, &c.SHA256, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist,
		&c.Genre, &c.Mode, &c.Difficulty, &c.Level,
		&c.MinBPM, &c.MaxBPM, &c.Path, &c.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 query: %w", err)
	}

	// IRメタ付与
	irMeta, err := r.metaRepo.GetChartMeta(ctx, c.MD5)
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 GetChartMeta: %w", err)
	}
	c.IRMeta = irMeta

	// 難易度ラベル付与
	labels, err := r.dtRepo.GetLabelsByMD5(ctx, c.MD5)
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 GetLabelsByMD5: %w", err)
	}
	c.DifficultyLabels = labels

	return &c, nil
}

func (r *SongdataReader) FindChartFoldersByTitle(ctx context.Context, title string) ([]model.InstallCandidate, error) {
	if title == "" {
		return nil, nil
	}

	query := `
		SELECT
			s.folder,
			MIN(s.title) AS title,
			MIN(s.artist) AS artist,
			MIN(s.path) AS path
		FROM songdata.song s
		WHERE LOWER(s.title) = LOWER(?)
		GROUP BY s.folder
	`

	rows, err := r.db.QueryContext(ctx, query, title)
	if err != nil {
		return nil, fmt.Errorf("FindChartFoldersByTitle: %w", err)
	}
	defer rows.Close()

	var candidates []model.InstallCandidate
	for rows.Next() {
		var folder, t, a, path string
		if err := rows.Scan(&folder, &t, &a, &path); err != nil {
			return nil, fmt.Errorf("FindChartFoldersByTitle scan: %w", err)
		}
		candidates = append(candidates, model.InstallCandidate{
			FolderPath: path,
			Title:      t,
			Artist:     a,
			MatchTypes: []string{"title"},
		})
	}
	return candidates, rows.Err()
}

func (r *SongdataReader) FindChartFoldersByBodyURL(ctx context.Context, bodyURL string) ([]model.InstallCandidate, error) {
	if bodyURL == "" {
		return nil, nil
	}

	// chart_meta.lr2ir_body_urlが一致する譜面のmd5を取得し、songdata.songと突合
	query := `
		SELECT
			s.folder,
			MIN(s.title) AS title,
			MIN(s.artist) AS artist,
			MIN(s.path) AS path
		FROM main.chart_meta cm
		INNER JOIN songdata.song s ON s.md5 = cm.md5
		WHERE cm.lr2ir_body_url = ?
		GROUP BY s.folder
	`

	rows, err := r.db.QueryContext(ctx, query, bodyURL)
	if err != nil {
		return nil, fmt.Errorf("FindChartFoldersByBodyURL: %w", err)
	}
	defer rows.Close()

	var candidates []model.InstallCandidate
	for rows.Next() {
		var folder, t, a, path string
		if err := rows.Scan(&folder, &t, &a, &path); err != nil {
			return nil, fmt.Errorf("FindChartFoldersByBodyURL scan: %w", err)
		}
		candidates = append(candidates, model.InstallCandidate{
			FolderPath: path,
			Title:      t,
			Artist:     a,
			MatchTypes: []string{"body_url"},
		})
	}
	return candidates, rows.Err()
}

func (r *SongdataReader) FindChartFoldersByArtist(ctx context.Context, artist string) ([]model.InstallCandidate, error) {
	if artist == "" {
		return nil, nil
	}

	query := `
		SELECT
			s.folder,
			MIN(s.title) AS title,
			MIN(s.artist) AS artist,
			MIN(s.path) AS path
		FROM songdata.song s
		WHERE LOWER(s.artist) = LOWER(?)
		GROUP BY s.folder
	`

	rows, err := r.db.QueryContext(ctx, query, artist)
	if err != nil {
		return nil, fmt.Errorf("FindChartFoldersByArtist: %w", err)
	}
	defer rows.Close()

	var candidates []model.InstallCandidate
	for rows.Next() {
		var folder, t, a, path string
		if err := rows.Scan(&folder, &t, &a, &path); err != nil {
			return nil, fmt.Errorf("FindChartFoldersByArtist scan: %w", err)
		}
		candidates = append(candidates, model.InstallCandidate{
			FolderPath: path,
			Title:      t,
			Artist:     a,
			MatchTypes: []string{"artist"},
		})
	}
	return candidates, rows.Err()
}
