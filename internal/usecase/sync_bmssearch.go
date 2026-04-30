package usecase

import (
	"context"
	"sync"
	"sync/atomic"

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
	resolver      *BMSSearchResolver
	bmssearchRepo model.BMSSearchRepository
	metaRepo      model.MetaRepository
}

func NewSyncBMSSearchUseCase(
	resolver *BMSSearchResolver,
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
) *SyncBMSSearchUseCase {
	return &SyncBMSSearchUseCase{
		resolver:      resolver,
		bmssearchRepo: bmssearchRepo,
		metaRepo:      metaRepo,
	}
}

func (u *SyncBMSSearchUseCase) Execute(
	ctx context.Context,
	folders []string,
	md5sByFolder map[string][]string,
	titleByFolder map[string]string,
	artistByFolder map[string]string,
	progressFn func(SyncBMSSearchProgress),
) (*SyncBMSSearchResult, error) {
	total := len(folders)
	var synced, notFound atomic.Int64
	var completed atomic.Int64

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

			ok := u.syncFolder(ctx, fh, md5sByFolder[fh], titleByFolder[fh], artistByFolder[fh])
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
	title, artist string,
) bool {
	bmsID, _, err := u.resolver.ResolveForFolder(ctx, folderHash, md5s, title, artist)
	if err != nil || bmsID == "" {
		return false
	}
	// exhibition_id があり、かつローカル event テーブルに対応 event があれば event_id も更新
	bms, err := u.bmssearchRepo.GetBMSByID(ctx, bmsID)
	if err != nil || bms == nil || bms.ExhibitionID == nil {
		return true
	}
	event, err := u.metaRepo.GetEventByBMSSearchID(ctx, *bms.ExhibitionID)
	if err != nil || event == nil {
		return true
	}
	_ = u.metaRepo.UpdateSongMetaEvent(ctx, folderHash, *bms.ExhibitionID, bmsID)
	return true
}
