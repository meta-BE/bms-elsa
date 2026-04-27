package model

import (
	"context"
	"time"
)

// BMSSearchSource は md5 ↔ bms_id リンクの確度区分
type BMSSearchSource string

const (
	BMSSearchSourceOfficial   BMSSearchSource = "official"
	BMSSearchSourceUnofficial BMSSearchSource = "unofficial"
)

// BMSSearchLink は md5 と BMS Search の bms_id のリンク
type BMSSearchLink struct {
	MD5        string
	BMSID      string
	Source     BMSSearchSource
	ResolvedAt time.Time
}

// BMSSearchBMS は BMS API の楽曲レスポンスをキャッシュしたエンティティ
type BMSSearchBMS struct {
	BMSID          string
	Title          string
	Artist         string
	SubArtist      string
	Genre          string
	ExhibitionID   *string
	ExhibitionName string
	PublishedAt    string
	Downloads      []BMSSearchURLEntry
	Previews       []BMSSearchPreview
	RelatedLinks   []BMSSearchURLEntry
	FetchedAt      time.Time
}

// BMSSearchURLEntry は URL + 説明のペア（DLリンク・関連リンク用）
type BMSSearchURLEntry struct {
	URL         string
	Description string
}

// BMSSearchPreview は再生プレビュー（YouTube/SoundCloud/NicoNico）
type BMSSearchPreview struct {
	Service   string
	Parameter string
}

// BMSSearchRepository は bmssearch_bms_id_md5 / bmssearch_bms の CRUD
type BMSSearchRepository interface {
	GetLinkByMD5(ctx context.Context, md5 string) (*BMSSearchLink, error)
	UpsertLinks(ctx context.Context, links []BMSSearchLink) error
	DeleteLinkByMD5(ctx context.Context, md5 string) error
	DeleteLinksByMD5s(ctx context.Context, md5s []string) error

	GetBMSByID(ctx context.Context, bmsID string) (*BMSSearchBMS, error)
	UpsertBMS(ctx context.Context, bms BMSSearchBMS) error
}
