package usecase

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type SyncBMSSearchProgress struct {
	Current int
	Total   int
}

type SyncBMSSearchResult struct {
	Total     int
	Synced    int
	NotFound  int
	Failed    int
	Cancelled bool
}

type SyncBMSSearchUseCase struct {
	bmsClient *gateway.BMSSearchClient
	metaRepo  model.MetaRepository
}

func NewSyncBMSSearchUseCase(
	bmsClient *gateway.BMSSearchClient,
	metaRepo model.MetaRepository,
) *SyncBMSSearchUseCase {
	return &SyncBMSSearchUseCase{bmsClient: bmsClient, metaRepo: metaRepo}
}

func (u *SyncBMSSearchUseCase) Execute(
	ctx context.Context,
	folders []string,
	md5sByFolder map[string][]string,
	progressFn func(SyncBMSSearchProgress),
) (*SyncBMSSearchResult, error) {
	total := len(folders)
	var synced, notFound atomic.Int64
	var completed atomic.Int64

	// BMS詳細のキャッシュ（並列安全）
	var bmsCacheMu sync.Mutex
	bmsCache := make(map[string]*gateway.BMSSearchBMS)

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, folderHash := range folders {
		select {
		case <-ctx.Done():
			wg.Wait()
			return &SyncBMSSearchResult{
				Total: total, Synced: int(synced.Load()), NotFound: int(notFound.Load()),
				Cancelled: true,
			}, nil
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(fh string) {
			defer func() { <-sem; wg.Done() }()

			md5s := md5sByFolder[fh]
			ok := u.syncFolder(ctx, fh, md5s, bmsCache, &bmsCacheMu)
			if ok {
				synced.Add(1)
			} else {
				notFound.Add(1)
			}

			c := int(completed.Add(1))
			if progressFn != nil {
				progressFn(SyncBMSSearchProgress{Current: c, Total: total})
			}
		}(folderHash)
	}
	wg.Wait()

	return &SyncBMSSearchResult{
		Total: total, Synced: int(synced.Load()), NotFound: int(notFound.Load()),
	}, nil
}

func (u *SyncBMSSearchUseCase) syncFolder(
	ctx context.Context,
	folderHash string,
	md5s []string,
	bmsCache map[string]*gateway.BMSSearchBMS,
	bmsCacheMu *sync.Mutex,
) bool {
	for _, md5 := range md5s {
		pattern, err := u.bmsClient.LookupPatternByMD5(ctx, md5)
		if err != nil || pattern == nil {
			continue
		}

		bmsID := pattern.BMS.ID

		// キャッシュ確認（並列安全）
		bmsCacheMu.Lock()
		bms, cached := bmsCache[bmsID]
		bmsCacheMu.Unlock()

		if !cached {
			bms, err = u.bmsClient.LookupBMS(ctx, bmsID)
			if err != nil {
				continue
			}
			bmsCacheMu.Lock()
			bmsCache[bmsID] = bms
			bmsCacheMu.Unlock()
		}
		if bms == nil {
			continue
		}

		if bms.Exhibition != nil {
			event, err := u.metaRepo.GetEventByBMSSearchID(ctx, bms.Exhibition.ID)
			if err == nil && event != nil {
				u.metaRepo.UpdateSongMetaEvent(ctx, folderHash, bms.Exhibition.ID, bmsID)
				return true
			}
		}

		// exhibitionがなくてもbms_search_idは保存
		u.metaRepo.UpsertSongMeta(ctx, model.SongMeta{
			FolderHash:  folderHash,
			BMSSearchID: &bmsID,
		})
		return true
	}
	return false
}
