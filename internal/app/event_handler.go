package app

import (
	"context"
	"regexp"
	"strconv"
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
			URL:         e.URL,
		}
	}
	return result, nil
}

// UpdateEventShortName はイベントの短縮名を更新する
func (h *EventHandler) UpdateEventShortName(id int, shortName string) error {
	return h.metaRepo.UpdateEventShortName(h.ctx, id, shortName)
}

// UpdateEventReleaseYear はイベントのリリース年を更新する
func (h *EventHandler) UpdateEventReleaseYear(id int, releaseYear int) error {
	return h.metaRepo.UpdateEventReleaseYear(h.ctx, id, releaseYear)
}

// RefreshEventsFromBMSSearch はBMS Searchからイベント一覧を取得してDBに反映し、追加件数を返す
func (h *EventHandler) RefreshEventsFromBMSSearch() (int, error) {
	exhibitions, err := h.bmsClient.FetchAllExhibitions(h.ctx)
	if err != nil {
		return 0, err
	}

	added := 0
	for _, ex := range exhibitions {
		url := extractExhibitionURL(ex)

		existing, err := h.metaRepo.GetEventByBMSSearchID(h.ctx, ex.ID)
		if err != nil {
			continue
		}
		if existing != nil {
			// 既存イベントでもURLが未設定なら更新
			if url != "" && existing.URL == "" {
				existing.URL = url
				_ = h.metaRepo.UpsertEventByBMSSearchID(h.ctx, *existing)
			}
			continue
		}

		year := extractExhibitionYear(ex)
		err = h.metaRepo.UpsertEventByBMSSearchID(h.ctx, model.Event{
			BMSSearchID: &ex.ID,
			Name:        ex.Name,
			ShortName:   ex.Name,
			ReleaseYear: year,
			URL:         url,
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

// extractExhibitionURL はイベントのURLを抽出する
func extractExhibitionURL(ex gateway.BMSSearchExhibitionDetail) string {
	if ex.LinkedProfile != nil && len(ex.LinkedProfile.Websites) > 0 {
		return ex.LinkedProfile.Websites[0].URL
	}
	return ""
}

// extractExhibitionYear はイベントの開催年を抽出する
// 優先順: entry.startsAt → impression.startsAt → イベント名から年号抽出 → createdAt
func extractExhibitionYear(ex gateway.BMSSearchExhibitionDetail) int {
	if ex.Terms != nil {
		if ex.Terms.Entry != nil && ex.Terms.Entry.StartsAt != "" {
			if t, err := time.Parse(time.RFC3339, ex.Terms.Entry.StartsAt); err == nil {
				return t.Year()
			}
		}
		if ex.Terms.Impression != nil && ex.Terms.Impression.StartsAt != "" {
			if t, err := time.Parse(time.RFC3339, ex.Terms.Impression.StartsAt); err == nil {
				return t.Year()
			}
		}
	}
	// イベント名から4桁の年号を抽出（例: "BOFU2016" → 2016）
	if m := regexp.MustCompile(`(19|20)\d{2}`).FindString(ex.Name); m != "" {
		if y, err := strconv.Atoi(m); err == nil {
			return y
		}
	}
	// 2桁の年号（例: "BMSをたくさん作るぜ'24" → 2024）
	if m := regexp.MustCompile(`'(\d{2})`).FindStringSubmatch(ex.Name); len(m) == 2 {
		if y, err := strconv.Atoi(m[1]); err == nil {
			return 2000 + y
		}
	}
	if ex.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, ex.CreatedAt); err == nil {
			return t.Year()
		}
	}
	return 0
}
