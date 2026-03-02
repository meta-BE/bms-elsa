package usecase

import (
	"context"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// LookupIRUseCase はLR2IRから譜面情報を取得し、結果をDBに保存するユースケース
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
	now := time.Now()
	meta := model.ChartIRMeta{
		MD5:       md5,
		SHA256:    sha256,
		FetchedAt: &now,
	}
	if resp.Registered {
		meta.Tags = resp.Tags
		meta.LR2IRBodyURL = resp.BodyURL
		meta.LR2IRDiffURL = resp.DiffURL
		meta.LR2IRNotes = resp.Notes
	}
	if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
		return nil, err
	}
	return resp, nil
}
