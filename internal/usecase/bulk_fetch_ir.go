package usecase

import (
	"context"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// BulkFetchProgress は進捗通知用
type BulkFetchProgress struct {
	Current int
	Total   int
}

// BulkFetchResult は一括取得の結果
type BulkFetchResult struct {
	Total     int
	Fetched   int // 登録済み
	NotFound  int // LR2IR未登録
	Failed    int // エラー（スキップ）
	Cancelled bool
}

type BulkFetchIRUseCase struct {
	irClient port.IRClient
	metaRepo model.MetaRepository
}

func NewBulkFetchIRUseCase(irClient port.IRClient, metaRepo model.MetaRepository) *BulkFetchIRUseCase {
	return &BulkFetchIRUseCase{irClient: irClient, metaRepo: metaRepo}
}

func (u *BulkFetchIRUseCase) Execute(ctx context.Context, md5s []string, progressFn func(BulkFetchProgress)) (*BulkFetchResult, error) {
	result := &BulkFetchResult{Total: len(md5s)}

	for i, md5 := range md5s {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result, nil
		default:
		}

		resp, err := u.irClient.LookupByMD5(ctx, md5)
		if err != nil {
			if ctx.Err() != nil {
				result.Cancelled = true
				return result, nil
			}
			result.Failed++
			if progressFn != nil {
				progressFn(BulkFetchProgress{Current: i + 1, Total: len(md5s)})
			}
			continue
		}

		now := time.Now()
		meta := model.ChartIRMeta{
			MD5:       md5,
			FetchedAt: &now,
		}
		if resp.Registered {
			meta.Tags = resp.Tags
			meta.LR2IRBodyURL = resp.BodyURL
			meta.LR2IRDiffURL = resp.DiffURL
			meta.LR2IRNotes = resp.Notes
			result.Fetched++
		} else {
			result.NotFound++
		}

		if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
			result.Failed++
		}

		if progressFn != nil {
			progressFn(BulkFetchProgress{Current: i + 1, Total: len(md5s)})
		}
	}

	return result, nil
}
