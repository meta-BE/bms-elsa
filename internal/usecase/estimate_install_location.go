package usecase

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type EstimateInstallLocationUseCase struct {
	songRepo model.SongRepository
	metaRepo model.MetaRepository
}

func NewEstimateInstallLocationUseCase(songRepo model.SongRepository, metaRepo model.MetaRepository) *EstimateInstallLocationUseCase {
	return &EstimateInstallLocationUseCase{songRepo: songRepo, metaRepo: metaRepo}
}

// スコア定義
const (
	scoreTitle     = 3
	scoreBaseTitle = 2
	scoreBodyURL   = 3
	scoreArtist    = 1
)

// 末尾の [...] (...) -...- を繰り返し除去する正規表現
var trailingSuffixRe = regexp.MustCompile(`\s*(\[[^\]]*\]|\([^)]*\)|-[^-]+-)\s*$`)

// extractBaseTitle はエントリtitleから末尾の接尾辞を除去してベースタイトルを返す。
// 例: "影縫 [闇] (集0)" → "影縫"
func extractBaseTitle(title string) string {
	base := title
	for {
		trimmed := trailingSuffixRe.ReplaceAllString(base, "")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == base || trimmed == "" {
			break
		}
		base = trimmed
	}
	return base
}

// Execute は難易度表エントリのtitle, artist, md5をもとに、導入先候補をfolder単位で返す。
// 複数のマッチング手法で検索し、スコアリングで結果をランク付けする。
func (u *EstimateInstallLocationUseCase) Execute(ctx context.Context, title, artist, md5 string) ([]model.InstallCandidate, error) {
	merged := make(map[string]*model.InstallCandidate)

	addCandidates := func(candidates []model.InstallCandidate, matchType string, score int) {
		for _, c := range candidates {
			if existing, ok := merged[c.FolderPath]; ok {
				existing.MatchTypes = append(existing.MatchTypes, matchType)
				existing.Score += score
			} else {
				c.MatchTypes = []string{matchType}
				c.Score = score
				merged[c.FolderPath] = &c
			}
		}
	}

	// 1. タイトル完全一致検索（スコア3）
	titleCandidates, err := u.songRepo.FindChartFoldersByTitle(ctx, title)
	if err != nil {
		return nil, err
	}
	addCandidates(titleCandidates, "title", scoreTitle)

	// 2. ベースタイトル一致検索（スコア2）— 元titleと異なる場合のみ
	baseTitle := extractBaseTitle(title)
	if baseTitle != title && baseTitle != "" {
		baseCandidates, err := u.songRepo.FindChartFoldersByTitle(ctx, baseTitle)
		if err != nil {
			return nil, err
		}
		addCandidates(baseCandidates, "base_title", scoreBaseTitle)
	}

	// 3. body_url一致検索（スコア3）
	meta, err := u.metaRepo.GetChartMeta(ctx, md5)
	if err != nil {
		return nil, err
	}
	if meta != nil && meta.LR2IRBodyURL != "" {
		urlCandidates, err := u.songRepo.FindChartFoldersByBodyURL(ctx, meta.LR2IRBodyURL)
		if err != nil {
			return nil, err
		}
		addCandidates(urlCandidates, "body_url", scoreBodyURL)
	}

	// 4. アーティスト一致検索（スコア1）
	if artist != "" {
		artistCandidates, err := u.songRepo.FindChartFoldersByArtist(ctx, artist)
		if err != nil {
			return nil, err
		}
		addCandidates(artistCandidates, "artist", scoreArtist)
	}

	// 5. map→スライスに変換し、スコア降順でソート
	result := make([]model.InstallCandidate, 0, len(merged))
	for _, c := range merged {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result, nil
}
