package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DifficultyTableHandler struct {
	ctx             context.Context
	dtRepo          *persistence.DifficultyTableRepository
	dtFetcher       *gateway.DifficultyTableFetcher
	songReader      *persistence.SongdataReader
	estimateUseCase *usecase.EstimateInstallLocationUseCase

	// 非同期一括更新の状態管理
	mu         sync.Mutex
	refreshing bool
	cancelFunc context.CancelFunc
	progress   struct{ current, total int }
}

func NewDifficultyTableHandler(
	dtRepo *persistence.DifficultyTableRepository,
	dtFetcher *gateway.DifficultyTableFetcher,
	songReader *persistence.SongdataReader,
	estimateUseCase *usecase.EstimateInstallLocationUseCase,
) *DifficultyTableHandler {
	return &DifficultyTableHandler{
		dtRepo:          dtRepo,
		dtFetcher:       dtFetcher,
		songReader:      songReader,
		estimateUseCase: estimateUseCase,
	}
}

func (h *DifficultyTableHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *DifficultyTableHandler) GetDifficultyTableEntry(tableID int, md5 string) (*dto.DifficultyTableEntryDTO, error) {
	entry, err := h.dtRepo.GetEntry(h.ctx, tableID, md5)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	counts, err := h.songReader.CountChartsByMD5s(h.ctx, []string{md5})
	if err != nil {
		return nil, err
	}

	count := 0
	if counts != nil {
		count = counts[md5]
	}
	status := "not_installed"
	if count == 1 {
		status = "installed"
	} else if count > 1 {
		status = "duplicate"
	}

	result := dto.DifficultyTableEntryDTO{
		MD5: entry.MD5, Level: entry.Level, Title: entry.Title, Artist: entry.Artist,
		URL: entry.URL, URLDiff: entry.URLDiff,
		Status: status, InstalledCount: count,
	}
	return &result, nil
}

func (h *DifficultyTableHandler) ListDifficultyTableEntries(tableID int) ([]dto.DifficultyTableEntryDTO, error) {
	entries, err := h.dtRepo.ListEntries(h.ctx, tableID)
	if err != nil {
		return nil, err
	}

	md5s := make([]string, len(entries))
	for i, e := range entries {
		md5s[i] = e.MD5
	}

	counts, err := h.songReader.CountChartsByMD5s(h.ctx, md5s)
	if err != nil {
		return nil, err
	}

	result := make([]dto.DifficultyTableEntryDTO, len(entries))
	for i, e := range entries {
		count := 0
		if counts != nil {
			count = counts[e.MD5]
		}
		status := "not_installed"
		if count == 1 {
			status = "installed"
		} else if count > 1 {
			status = "duplicate"
		}
		result[i] = dto.DifficultyTableEntryDTO{
			MD5: e.MD5, Level: e.Level, Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
			Status: status, InstalledCount: count,
		}
	}
	return result, nil
}

func (h *DifficultyTableHandler) ListDifficultyTables() ([]dto.DifficultyTableDTO, error) {
	tables, err := h.dtRepo.ListTables(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.DifficultyTableDTO, len(tables))
	for i, t := range tables {
		count, _ := h.dtRepo.CountEntries(h.ctx, t.ID)
		var fetchedAt *string
		if t.FetchedAt != nil {
			s := t.FetchedAt.Local().Format("2006-01-02 15:04")
			fetchedAt = &s
		}
		result[i] = dto.DifficultyTableDTO{
			ID: t.ID, URL: t.URL, Name: t.Name, Symbol: t.Symbol,
			EntryCount: count, FetchedAt: fetchedAt,
		}
	}
	return result, nil
}

func (h *DifficultyTableHandler) AddDifficultyTable(tableURL string) error {
	headerURL, err := h.dtFetcher.FetchHeaderURL(tableURL)
	if err != nil {
		return err
	}

	header, err := h.dtFetcher.FetchHeader(headerURL)
	if err != nil {
		return err
	}

	entries, err := h.dtFetcher.FetchBody(header.DataURL)
	if err != nil {
		return err
	}

	tableID, err := h.dtRepo.InsertTable(h.ctx, persistence.DifficultyTable{
		URL: tableURL, HeaderURL: headerURL, DataURL: header.DataURL,
		Name: header.Name, Symbol: header.Symbol,
	})
	if err != nil {
		return err
	}

	dbEntries := make([]persistence.DifficultyTableEntry, len(entries))
	for i, e := range entries {
		dbEntries[i] = persistence.DifficultyTableEntry{
			TableID: tableID, MD5: e.MD5, Level: e.Level,
			Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
		}
	}
	return h.dtRepo.ReplaceEntries(h.ctx, tableID, dbEntries)
}

func (h *DifficultyTableHandler) RemoveDifficultyTable(id int) error {
	return h.dtRepo.DeleteTable(h.ctx, id)
}

func (h *DifficultyTableHandler) ReorderDifficultyTables(ids []int) error {
	return h.dtRepo.ReorderTables(h.ctx, ids)
}

func (h *DifficultyTableHandler) RefreshDifficultyTable(id int) dto.DifficultyTableRefreshResult {
	tables, err := h.dtRepo.ListTables(h.ctx)
	if err != nil {
		return dto.DifficultyTableRefreshResult{Success: false, Error: err.Error()}
	}

	var target *persistence.DifficultyTable
	for _, t := range tables {
		if t.ID == id {
			target = &t
			break
		}
	}
	if target == nil {
		return dto.DifficultyTableRefreshResult{Success: false, Error: "テーブルが見つかりません"}
	}

	return h.refreshTable(*target)
}

func (h *DifficultyTableHandler) RefreshAllDifficultyTables() []dto.DifficultyTableRefreshResult {
	tables, err := h.dtRepo.ListTables(h.ctx)
	if err != nil {
		return []dto.DifficultyTableRefreshResult{{Success: false, Error: err.Error()}}
	}

	results := make([]dto.DifficultyTableRefreshResult, len(tables))
	for i, t := range tables {
		results[i] = h.refreshTable(t)
	}
	return results
}

func (h *DifficultyTableHandler) EstimateInstallLocation(md5 string, tableID int) ([]dto.InstallCandidateDTO, error) {
	// 難易度表エントリからtitleを取得
	entry, err := h.dtRepo.GetEntry(h.ctx, tableID, md5)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	candidates, err := h.estimateUseCase.Execute(h.ctx, entry.Title, entry.Artist, md5)
	if err != nil {
		return nil, err
	}

	result := make([]dto.InstallCandidateDTO, len(candidates))
	for i, c := range candidates {
		result[i] = dto.InstallCandidateDTO{
			FolderPath: c.FolderPath,
			Title:      c.Title,
			Artist:     c.Artist,
			MatchTypes: c.MatchTypes,
			Score:      c.Score,
		}
	}
	return result, nil
}

// RefreshAllDifficultyTablesAsync は全難易度表を最大5並列で非同期更新する。
// 二重実行時はエラーを返す。進捗は dt:refresh-progress イベントで通知する。
func (h *DifficultyTableHandler) RefreshAllDifficultyTablesAsync() error {
	h.mu.Lock()
	if h.refreshing {
		h.mu.Unlock()
		return fmt.Errorf("既に更新中です")
	}
	h.refreshing = true
	h.mu.Unlock()

	tables, err := h.dtRepo.ListTables(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.refreshing = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.progress.current = 0
	h.progress.total = len(tables)
	h.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			h.mu.Lock()
			h.refreshing = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		sem := make(chan struct{}, 5)
		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make([]dto.DifficultyTableRefreshResult, len(tables))
		completed := 0

		for i, t := range tables {
			select {
			case <-ctx.Done():
				mu.Lock()
				for j := i; j < len(tables); j++ {
					results[j] = dto.DifficultyTableRefreshResult{
						TableName: tables[j].Name, Error: "キャンセルされました",
					}
				}
				mu.Unlock()
				goto done
			case sem <- struct{}{}:
			}

			wg.Add(1)
			go func(idx int, tbl persistence.DifficultyTable) {
				defer func() { <-sem; wg.Done() }()
				result := h.refreshTable(tbl)
				mu.Lock()
				results[idx] = result
				completed++
				c := completed
				mu.Unlock()

				h.mu.Lock()
				h.progress.current = c
				h.mu.Unlock()

				wailsRuntime.EventsEmit(h.ctx, "dt:refresh-progress", map[string]any{
					"current":   c,
					"total":     len(tables),
					"tableName": tbl.Name,
					"success":   result.Success,
					"error":     result.Error,
				})
			}(i, t)
		}
		wg.Wait()

	done:
		wg.Wait()
		wailsRuntime.EventsEmit(h.ctx, "dt:refresh-done", map[string]any{
			"results": results,
		})
	}()

	return nil
}

// StopDifficultyTableRefresh は実行中の一括更新をキャンセルする
func (h *DifficultyTableHandler) StopDifficultyTableRefresh() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsRefreshing は一括更新が実行中かどうかを返す
func (h *DifficultyTableHandler) IsRefreshing() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.refreshing
}

// RefreshProgress は実行中の一括更新の進捗を返す
func (h *DifficultyTableHandler) RefreshProgress() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]int{
		"current": h.progress.current,
		"total":   h.progress.total,
	}
}

func (h *DifficultyTableHandler) refreshTable(t persistence.DifficultyTable) dto.DifficultyTableRefreshResult {
	result := dto.DifficultyTableRefreshResult{TableName: t.Name}

	header, err := h.dtFetcher.FetchHeader(t.HeaderURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	if header.DataURL != t.DataURL {
		t.DataURL = header.DataURL
	}
	t.Name = header.Name
	t.Symbol = header.Symbol

	entries, err := h.dtFetcher.FetchBody(t.DataURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	if err := h.dtRepo.UpdateTable(h.ctx, t); err != nil {
		result.Error = err.Error()
		return result
	}

	dbEntries := make([]persistence.DifficultyTableEntry, len(entries))
	for i, e := range entries {
		dbEntries[i] = persistence.DifficultyTableEntry{
			TableID: t.ID, MD5: e.MD5, Level: e.Level,
			Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
		}
	}
	if err := h.dtRepo.ReplaceEntries(h.ctx, t.ID, dbEntries); err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.EntryCount = len(entries)
	return result
}
