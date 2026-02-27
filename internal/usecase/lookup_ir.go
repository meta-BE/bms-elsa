package usecase

import (
	"context"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// LookupIRUseCase はLR2IRから譜面情報を取得し、登録済みならDBに保存するユースケース
type LookupIRUseCase struct {
	irClient port.IRClient
	metaRepo model.MetaRepository
}

func NewLookupIRUseCase(irClient port.IRClient, metaRepo model.MetaRepository) *LookupIRUseCase {
	return &LookupIRUseCase{irClient: irClient, metaRepo: metaRepo}
}

func (u *LookupIRUseCase) Execute(ctx context.Context, md5, sha256 string) (*port.IRResponse, error) {
	resp, err := u.irClient.LookupByMD5(ctx, md5)
	if err != nil {
		return nil, err
	}
	// 未登録の場合はDBに保存しない
	if !resp.Registered {
		return resp, nil
	}
	now := time.Now()
	meta := model.ChartIRMeta{
		MD5:          md5,
		SHA256:       sha256,
		Tags:         resp.Tags,
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
		FetchedAt:    &now,
	}
	if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
		return nil, err
	}
	return resp, nil
}
