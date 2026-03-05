package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type EstimateInstallLocationUseCase struct {
	songRepo model.SongRepository
	metaRepo model.MetaRepository
}

func NewEstimateInstallLocationUseCase(songRepo model.SongRepository, metaRepo model.MetaRepository) *EstimateInstallLocationUseCase {
	return &EstimateInstallLocationUseCase{songRepo: songRepo, metaRepo: metaRepo}
}

// Execute は難易度表エントリのtitleとmd5をもとに、導入先候補をfolder単位で返す。
// タイトル一致とLR2IR本体URL一致の両方で検索し、結果をマージする。
func (u *EstimateInstallLocationUseCase) Execute(ctx context.Context, title string, md5 string) ([]model.InstallCandidate, error) {
	// 1. タイトル完全一致検索
	titleCandidates, err := u.songRepo.FindChartFoldersByTitle(ctx, title)
	if err != nil {
		return nil, err
	}

	// 2. md5からchart_metaのbody_urlを取得
	var urlCandidates []model.InstallCandidate
	meta, err := u.metaRepo.GetChartMeta(ctx, md5)
	if err != nil {
		return nil, err
	}
	if meta != nil && meta.LR2IRBodyURL != "" {
		urlCandidates, err = u.songRepo.FindChartFoldersByBodyURL(ctx, meta.LR2IRBodyURL)
		if err != nil {
			return nil, err
		}
	}

	// 3. folder単位でマージ（matchTypesを統合）
	merged := make(map[string]*model.InstallCandidate)
	for i := range titleCandidates {
		c := &titleCandidates[i]
		merged[c.FolderPath] = c
	}
	for _, c := range urlCandidates {
		if existing, ok := merged[c.FolderPath]; ok {
			existing.MatchTypes = append(existing.MatchTypes, "body_url")
		} else {
			merged[c.FolderPath] = &c
		}
	}

	// 4. map→スライスに変換
	result := make([]model.InstallCandidate, 0, len(merged))
	for _, c := range merged {
		result = append(result, *c)
	}

	return result, nil
}
