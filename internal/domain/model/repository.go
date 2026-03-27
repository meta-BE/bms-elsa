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

// ChartScanTarget はMinHashスキャン対象の譜面情報
type ChartScanTarget struct {
	MD5  string
	Path string
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
	WavMinHash []byte // 代表MinHash署名（未計算ならnil）
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
	// アーティスト完全一致（大文字小文字無視）で導入済み譜面をfolder単位で検索
	FindChartFoldersByArtist(ctx context.Context, artist string) ([]InstallCandidate, error)
	// folder単位で楽曲グループを返す（重複スキャン用）
	ListSongGroupsForDuplicateScan(ctx context.Context) ([]SongGroup, error)
}

// MinHashMatch はMinHash類似度検索の結果
type MinHashMatch struct {
	MD5        string
	FolderPath string
	Similarity float64
}

// MetaRepository はelsa.dbのメタデータCRUD
type MetaRepository interface {
	// MinHash類似度検索
	FindMostSimilarByMinHash(ctx context.Context, queryMinhash []byte, threshold float64) (*MinHashMatch, error)
	GetSongMeta(ctx context.Context, folderHash string) (*SongMeta, error)
	UpsertSongMeta(ctx context.Context, meta SongMeta) error
	GetChartMeta(ctx context.Context, md5 string) (*ChartIRMeta, error)
	UpsertChartMeta(ctx context.Context, meta ChartIRMeta) error
	BulkUpsertChartMeta(ctx context.Context, metas []ChartIRMeta) error
	UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
	ListEvents(ctx context.Context) ([]Event, error)
	GetEventByBMSSearchID(ctx context.Context, bmsSearchID string) (*Event, error)
	UpsertEventByBMSSearchID(ctx context.Context, e Event) error
	UpdateEventShortName(ctx context.Context, id int, shortName string) error
	UpdateEventReleaseYear(ctx context.Context, id int, releaseYear int) error
	ListFoldersWithoutEvent(ctx context.Context) ([]string, error)
	UpdateSongMetaEvent(ctx context.Context, folderHash string, eventID string, bmsSearchID string) error
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
	// MinHashスキャン対象の譜面リスト
	ListChartsWithoutMinhash(ctx context.Context) ([]ChartScanTarget, error)
	// wav_minhashを更新（レコードがなければINSERT）
	UpdateWavMinhash(ctx context.Context, md5 string, minhash []byte) error
}
