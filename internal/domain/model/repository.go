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
}

// MetaRepository はelsa.dbのメタデータCRUD
type MetaRepository interface {
	GetSongMeta(ctx context.Context, folderHash string) (*SongMeta, error)
	UpsertSongMeta(ctx context.Context, meta SongMeta) error
	GetChartMeta(ctx context.Context, md5, sha256 string) (*ChartIRMeta, error)
	UpsertChartMeta(ctx context.Context, meta ChartIRMeta) error
	BulkUpsertChartMeta(ctx context.Context, metas []ChartIRMeta) error
	UpdateWorkingURLs(ctx context.Context, md5, sha256, workingBodyURL, workingDiffURL string) error
	// event_mapping CRUD
	ListEventMappings(ctx context.Context) ([]EventMapping, error)
	UpsertEventMapping(ctx context.Context, m EventMapping) error
	DeleteEventMapping(ctx context.Context, id int) error
	// 推測用: 未設定曲のfolderHash + 紐づくIR本体URLを取得
	ListUnsetSongsWithIRURLs(ctx context.Context) ([]SongIRURLs, error)
}
