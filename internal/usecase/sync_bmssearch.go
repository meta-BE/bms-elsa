package usecase

import (
	"context"

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
	result := &SyncBMSSearchResult{Total: len(folders)}
	bmsCache := make(map[string]*gateway.BMSSearchBMS)

	for i, folderHash := range folders {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result, nil
		default:
		}

		if progressFn != nil {
			progressFn(SyncBMSSearchProgress{Current: i + 1, Total: len(folders)})
		}

		md5s := md5sByFolder[folderHash]
		synced := u.syncFolder(ctx, folderHash, md5s, bmsCache)
		if synced {
			result.Synced++
		} else {
			result.NotFound++
		}
	}
	return result, nil
}

func (u *SyncBMSSearchUseCase) syncFolder(
	ctx context.Context,
	folderHash string,
	md5s []string,
	bmsCache map[string]*gateway.BMSSearchBMS,
) bool {
	for _, md5 := range md5s {
		pattern, err := u.bmsClient.LookupPatternByMD5(ctx, md5)
		if err != nil || pattern == nil {
			continue
		}

		bmsID := pattern.BMS.ID
		bms, cached := bmsCache[bmsID]
		if !cached {
			bms, err = u.bmsClient.LookupBMS(ctx, bmsID)
			if err != nil {
				continue
			}
			bmsCache[bmsID] = bms
		}
		if bms == nil {
			continue
		}

		if bms.Exhibition != nil {
			event, err := u.metaRepo.GetEventByBMSSearchID(ctx, bms.Exhibition.ID)
			if err == nil && event != nil {
				u.metaRepo.UpdateSongMetaEvent(ctx, folderHash, event.ID, bmsID)
				return true
			}
		}

		// exhibitionがなくてもbms_search_idは保存（event_id=0は設定しない）
		u.metaRepo.UpsertSongMeta(ctx, model.SongMeta{
			FolderHash:  folderHash,
			BMSSearchID: &bmsID,
		})
		return true
	}
	return false
}
