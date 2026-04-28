package usecase

import (
	"context"
	"time"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// BMSSearchAPI は BMSSearchClient のうち Resolver が使う部分のインターフェース
type BMSSearchAPI interface {
	LookupPatternByMD5(ctx context.Context, md5 string) (*gateway.BMSSearchPattern, error)
	LookupBMS(ctx context.Context, bmsID string) (*gateway.BMSSearchBMS, error)
	SearchBMSesByTitle(ctx context.Context, title string, limit int) ([]gateway.BMSSearchBMS, error)
}

const (
	fallbackSearchLimit    = 20
	fallbackScoreThreshold = 50
)

type BMSSearchResolver struct {
	bmsClient     BMSSearchAPI
	bmssearchRepo model.BMSSearchRepository
	metaRepo      model.MetaRepository
}

func NewBMSSearchResolver(
	bmsClient BMSSearchAPI,
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
) *BMSSearchResolver {
	return &BMSSearchResolver{
		bmsClient:     bmsClient,
		bmssearchRepo: bmssearchRepo,
		metaRepo:      metaRepo,
	}
}

// ResolveForFolder は楽曲フォルダ単位の解決。
// 公式ヒット時 + フォールバック採用時に bmssearch_bms_id_md5 / bmssearch_bms / song_meta を書き込む。
// 未紐付け時は ("", "", nil)。
func (r *BMSSearchResolver) ResolveForFolder(
	ctx context.Context,
	folderHash string,
	md5s []string,
	title, artist string,
) (string, model.BMSSearchSource, error) {
	bmsID, hit, err := r.tryOfficial(ctx, md5s)
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, md5s, bmsID, model.BMSSearchSourceOfficial); err != nil {
			return "", "", err
		}
		if err := r.metaRepo.UpdateSongMetaBMSSearch(ctx, folderHash, bmsID, string(model.BMSSearchSourceOfficial)); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceOfficial, nil
	}

	bmsID, hit, err = r.tryFallback(ctx, title, artist)
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, md5s, bmsID, model.BMSSearchSourceUnofficial); err != nil {
			return "", "", err
		}
		if err := r.metaRepo.UpdateSongMetaBMSSearch(ctx, folderHash, bmsID, string(model.BMSSearchSourceUnofficial)); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceUnofficial, nil
	}
	return "", "", nil
}

// ResolveForOrphanMD5 は未所持 md5 単位の解決。song_meta は触らない。
func (r *BMSSearchResolver) ResolveForOrphanMD5(
	ctx context.Context,
	md5, title, artist string,
) (string, model.BMSSearchSource, error) {
	bmsID, hit, err := r.tryOfficial(ctx, []string{md5})
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, []string{md5}, bmsID, model.BMSSearchSourceOfficial); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceOfficial, nil
	}
	bmsID, hit, err = r.tryFallback(ctx, title, artist)
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, []string{md5}, bmsID, model.BMSSearchSourceUnofficial); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceUnofficial, nil
	}
	return "", "", nil
}

// tryOfficial は公式 md5 ヒットを試み、最初に見つかった bmsID を返す
func (r *BMSSearchResolver) tryOfficial(ctx context.Context, md5s []string) (string, bool, error) {
	for _, md5 := range md5s {
		p, err := r.bmsClient.LookupPatternByMD5(ctx, md5)
		if err != nil {
			continue
		}
		if p == nil {
			continue
		}
		return p.BMS.ID, true, nil
	}
	return "", false, nil
}

// tryFallback はテキスト検索で候補を取得しスコアリング採用する
func (r *BMSSearchResolver) tryFallback(ctx context.Context, title, artist string) (string, bool, error) {
	if title == "" {
		return "", false, nil
	}
	cands, err := r.bmsClient.SearchBMSesByTitle(ctx, title, fallbackSearchLimit)
	if err != nil {
		return "", false, err
	}
	if len(cands) == 0 {
		return "", false, nil
	}
	refs := make([]ScoreCandidateRef, len(cands))
	for i, c := range cands {
		refs[i] = ScoreCandidateRef{Title: c.Title, Artist: c.Artist}
	}
	idx, ok := PickBestCandidate(refs, title, artist, fallbackScoreThreshold)
	if !ok {
		return "", false, nil
	}
	return cands[idx].ID, true, nil
}

// persist は md5s に対するリンク UPSERT と bmssearch_bms 取得・UPSERT を行う
func (r *BMSSearchResolver) persist(
	ctx context.Context,
	md5s []string,
	bmsID string,
	source model.BMSSearchSource,
) error {
	now := time.Now()
	links := make([]model.BMSSearchLink, len(md5s))
	for i, m := range md5s {
		links[i] = model.BMSSearchLink{MD5: m, BMSID: bmsID, Source: source, ResolvedAt: now}
	}
	if err := r.bmssearchRepo.UpsertLinks(ctx, links); err != nil {
		return err
	}
	// bmssearch_bms キャッシュ確認 → 未取得なら API
	cached, err := r.bmssearchRepo.GetBMSByID(ctx, bmsID)
	if err != nil {
		return err
	}
	if cached != nil {
		return nil
	}
	apiBMS, err := r.bmsClient.LookupBMS(ctx, bmsID)
	if err != nil {
		return err
	}
	if apiBMS == nil {
		return nil
	}
	return r.bmssearchRepo.UpsertBMS(ctx, gatewayBMSToModel(*apiBMS, now))
}

func gatewayBMSToModel(g gateway.BMSSearchBMS, fetchedAt time.Time) model.BMSSearchBMS {
	var exID *string
	exName := ""
	if g.Exhibition != nil {
		s := g.Exhibition.ID
		exID = &s
		exName = g.Exhibition.Name
	}
	dls := make([]model.BMSSearchURLEntry, len(g.Downloads))
	for i, d := range g.Downloads {
		dls[i] = model.BMSSearchURLEntry{URL: d.URL, Description: d.Description}
	}
	prevs := make([]model.BMSSearchPreview, len(g.Previews))
	for i, p := range g.Previews {
		prevs[i] = model.BMSSearchPreview{Service: p.Service, Parameter: p.Parameter}
	}
	rels := make([]model.BMSSearchURLEntry, len(g.RelatedLinks))
	for i, rl := range g.RelatedLinks {
		rels[i] = model.BMSSearchURLEntry{URL: rl.URL, Description: rl.Description}
	}
	return model.BMSSearchBMS{
		BMSID:          g.ID,
		Title:          g.Title,
		Artist:         g.Artist,
		SubArtist:      g.SubArtist,
		Genre:          g.Genre,
		ExhibitionID:   exID,
		ExhibitionName: exName,
		PublishedAt:    g.PublishedAt,
		Downloads:      dls,
		Previews:       prevs,
		RelatedLinks:   rels,
		FetchedAt:      fetchedAt,
	}
}
