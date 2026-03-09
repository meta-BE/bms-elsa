package usecase

import (
	"context"
	"fmt"
	"os"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// ScanMinHashProgress はスキャン進捗
type ScanMinHashProgress struct {
	Current int
	Total   int
}

// ScanMinHashResult はスキャン結果
type ScanMinHashResult struct {
	Total     int
	Computed  int
	Skipped   int
	Failed    int
	Cancelled bool
}

type ScanMinHashUseCase struct {
	metaRepo model.MetaRepository
}

func NewScanMinHashUseCase(metaRepo model.MetaRepository) *ScanMinHashUseCase {
	return &ScanMinHashUseCase{metaRepo: metaRepo}
}

func (u *ScanMinHashUseCase) Execute(ctx context.Context, targets []model.ChartScanTarget, progressFn func(ScanMinHashProgress)) *ScanMinHashResult {
	result := &ScanMinHashResult{Total: len(targets)}

	for i, tgt := range targets {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result
		default:
		}

		if _, err := os.Stat(tgt.Path); err != nil {
			result.Skipped++
			if progressFn != nil {
				progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
			}
			continue
		}

		parsed, err := bms.ParseBMSFile(tgt.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan: parse error %s: %v\n", tgt.Path, err)
			result.Failed++
			if progressFn != nil {
				progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
			}
			continue
		}

		sig := bms.ComputeMinHash(parsed.WAVFiles)
		if err := u.metaRepo.UpdateWavMinhash(ctx, tgt.MD5, sig.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "scan: db error %s: %v\n", tgt.MD5, err)
			result.Failed++
			if progressFn != nil {
				progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
			}
			continue
		}

		result.Computed++
		if progressFn != nil {
			progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
		}
	}

	return result
}
