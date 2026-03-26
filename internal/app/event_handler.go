package app

import (
	"context"
	"sync"
	"time"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type EventHandler struct {
	ctx            context.Context
	bmsClient      *gateway.BMSSearchClient
	syncBMSSearch  *usecase.SyncBMSSearchUseCase
	metaRepo       model.MetaRepository
	songdataReader *persistence.SongdataReader

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewEventHandler(
	bmsClient *gateway.BMSSearchClient,
	syncBMSSearch *usecase.SyncBMSSearchUseCase,
	metaRepo model.MetaRepository,
	songdataReader *persistence.SongdataReader,
) *EventHandler {
	return &EventHandler{
		bmsClient:      bmsClient,
		syncBMSSearch:  syncBMSSearch,
		metaRepo:       metaRepo,
		songdataReader: songdataReader,
	}
}

func (h *EventHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// ListEvents はイベント一覧を返す
func (h *EventHandler) ListEvents() ([]dto.EventDTO, error) {
	events, err := h.metaRepo.ListEvents(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.EventDTO, len(events))
	for i, e := range events {
		result[i] = dto.EventDTO{
			ID:          e.ID,
			BMSSearchID: e.BMSSearchID,
			Name:        e.Name,
			ShortName:   e.ShortName,
			ReleaseYear: e.ReleaseYear,
		}
	}
	return result, nil
}

// UpdateEventShortName はイベントの短縮名を更新する
func (h *EventHandler) UpdateEventShortName(id int, shortName string) error {
	return h.metaRepo.UpdateEventShortName(h.ctx, id, shortName)
}

// RefreshEventsFromBMSSearch はBMS Searchからイベント一覧を取得してDBに反映し、追加件数を返す
func (h *EventHandler) RefreshEventsFromBMSSearch() (int, error) {
	exhibitions, err := h.bmsClient.FetchAllExhibitions(h.ctx)
	if err != nil {
		return 0, err
	}

	added := 0
	for _, ex := range exhibitions {
		existing, err := h.metaRepo.GetEventByBMSSearchID(h.ctx, ex.ID)
		if err != nil {
			continue
		}
		if existing != nil {
			continue
		}

		year := extractExhibitionYear(ex)
		err = h.metaRepo.UpsertEventByBMSSearchID(h.ctx, model.Event{
			BMSSearchID: &ex.ID,
			Name:        ex.Name,
			ShortName:   ex.Name,
			ReleaseYear: year,
		})
		if err == nil {
			added++
		}
	}
	return added, nil
}

// StartBMSSearchSync はBMS Search同期をバックグラウンドで開始する
func (h *EventHandler) StartBMSSearchSync() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	folders, err := h.metaRepo.ListFoldersWithoutEvent(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	md5sByFolder, err := h.songdataReader.ListMD5sGroupedByFolder(h.ctx, folders)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, _ := h.syncBMSSearch.Execute(ctx, folders, md5sByFolder, func(p usecase.SyncBMSSearchProgress) {
			wailsRuntime.EventsEmit(h.ctx, "bmssearch:sync-progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		doneData := map[string]any{
			"total":     0,
			"synced":    0,
			"notFound":  0,
			"failed":    0,
			"cancelled": false,
		}
		if result != nil {
			doneData["total"] = result.Total
			doneData["synced"] = result.Synced
			doneData["notFound"] = result.NotFound
			doneData["failed"] = result.Failed
			doneData["cancelled"] = result.Cancelled
		}
		wailsRuntime.EventsEmit(h.ctx, "bmssearch:sync-done", doneData)
	}()

	return nil
}

// StopBMSSearchSync は実行中のBMS Search同期を中断する
func (h *EventHandler) StopBMSSearchSync() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsSyncing はBMS Search同期が実行中かどうかを返す
func (h *EventHandler) IsSyncing() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// extractExhibitionYear はイベントの開催年を抽出する
func extractExhibitionYear(ex gateway.BMSSearchExhibitionDetail) int {
	if ex.Terms != nil && ex.Terms.Entry != nil && ex.Terms.Entry.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, ex.Terms.Entry.StartsAt)
		if err == nil {
			return t.Year()
		}
	}
	if ex.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, ex.CreatedAt)
		if err == nil {
			return t.Year()
		}
	}
	return 0
}
