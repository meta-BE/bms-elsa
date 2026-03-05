package model

import "context"

// ListOptions は楽曲一覧取得のオプション
type ListOptions struct {
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
	Search   string // title, artist, genreを横断検索
}

// DuplicateGroup は同一md5の譜面グループ
type DuplicateGroup struct {
	MD5    string
	Charts []Chart
}

// SongRepository はsongdata.dbから楽曲・譜面を読み取る（読み取り専用）
type SongRepository interface {
	ListSongs(ctx context.Context, opts ListOptions) ([]Song, int, error)
	ListAllSongs(ctx context.Context) ([]Song, error)
	GetSongByFolder(ctx context.Context, folderHash string) (*Song, error)
	// タイトル完全一致（大文字小文字無視）で導入済み譜面をfolder単位で検索
	FindChartFoldersByTitle(ctx context.Context, title string) ([]InstallCandidate, error)
	// LR2IR本体URLが一致する導入済み譜面をfolder単位で検索
	FindChartFoldersByBodyURL(ctx context.Context, bodyURL string) ([]InstallCandidate, error)
}

// MetaRepository はelsa.dbのメタデータCRUD
type MetaRepository interface {
	GetSongMeta(ctx context.Context, folderHash string) (*SongMeta, error)
	UpsertSongMeta(ctx context.Context, meta SongMeta) error
	GetChartMeta(ctx context.Context, md5 string) (*ChartIRMeta, error)
	UpsertChartMeta(ctx context.Context, meta ChartIRMeta) error
	BulkUpsertChartMeta(ctx context.Context, metas []ChartIRMeta) error
	UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
	ListEventMappings(ctx context.Context) ([]EventMapping, error)
	UpsertEventMapping(ctx context.Context, m EventMapping) error
	DeleteEventMapping(ctx context.Context, id int) error
	ListUnsetSongsWithIRURLs(ctx context.Context) ([]SongIRURLs, error)
	// IR未取得の譜面md5一覧（songdata.songベース）
	ListUnfetchedChartMD5s(ctx context.Context) ([]string, error)
	// 難易度表の未取得エントリmd5一覧
	ListUnfetchedDTEntryMD5s(ctx context.Context, tableID int) ([]string, error)
	// URL書き換えルール
	ListRewriteRules(ctx context.Context) ([]RewriteRule, error)
	UpsertRewriteRule(ctx context.Context, rule RewriteRule) error
	DeleteRewriteRule(ctx context.Context, id int) error
	// 動作URL未設定の譜面（lr2ir URLあり）を取得
	ListChartsForWorkingURLInference(ctx context.Context) ([]ChartIRMeta, error)
}
