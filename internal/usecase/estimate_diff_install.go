package usecase

import (
	"context"
	"path/filepath"
	"sort"
	"time"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
)

const (
	minhashScoreMultiplier = 10.0 // MinHash類似度 * 10 = MinHashスコア（最大10点）
	irSkipThreshold        = 8.0  // MinHashスコアがこの値以上ならIR問い合わせをスキップ
)

// ImportCandidate は差分導入の推定結果
type ImportCandidate struct {
	FilePath    string
	FileName    string
	Title       string
	Subtitle    string
	Artist      string
	Subartist   string
	Genre       string
	MD5         string
	DestFolder  string  // 推定先フォルダ（空なら未推定）
	Score       float64 // 統合スコア（MinHashスコア + メタデータスコア）
	MatchMethod string  // 最もスコアに寄与した手段: "minhash" / "ir" / "title" / ""
}

type EstimateDiffInstallUseCase struct {
	elsaRepo        *persistence.ElsaRepository
	songRepo        model.SongRepository
	metaRepo        model.MetaRepository
	irClient        port.IRClient
	estimateUseCase *EstimateInstallLocationUseCase
}

func NewEstimateDiffInstallUseCase(
	elsaRepo *persistence.ElsaRepository,
	songRepo model.SongRepository,
	metaRepo model.MetaRepository,
	irClient port.IRClient,
	estimateUseCase *EstimateInstallLocationUseCase,
) *EstimateDiffInstallUseCase {
	return &EstimateDiffInstallUseCase{
		elsaRepo:        elsaRepo,
		songRepo:        songRepo,
		metaRepo:        metaRepo,
		irClient:        irClient,
		estimateUseCase: estimateUseCase,
	}
}

// folderScore はフォルダ単位のスコア集約用
type folderScore struct {
	FolderPath    string
	MinHashScore  float64
	MetadataScore float64
	BestMethod    string // 最もスコアに寄与した手段
}

func (fs folderScore) Total() float64 {
	return fs.MinHashScore + fs.MetadataScore
}

// EstimateOne は1ファイルの導入先を統一スコア方式で推定する
func (u *EstimateDiffInstallUseCase) EstimateOne(ctx context.Context, filePath string) (*ImportCandidate, error) {
	parsed, err := bms.ParseBMSFile(filePath)
	if err != nil {
		return nil, err
	}

	candidate := &ImportCandidate{
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		Title:     parsed.Title,
		Subtitle:  parsed.Subtitle,
		Artist:    parsed.Artist,
		Subartist: parsed.Subartist,
		Genre:     parsed.Genre,
		MD5:       parsed.MD5,
	}

	// フォルダごとのスコア集約map
	scores := make(map[string]*folderScore)

	// Step 1: WAV MinHash類似度検索
	sig := bms.ComputeMinHash(parsed.WAVFiles)
	match, err := u.elsaRepo.FindMostSimilarByMinHash(ctx, sig.Bytes(), 0.0)
	if err != nil {
		return nil, err
	}
	if match != nil && match.Similarity > 0 {
		mhScore := match.Similarity * minhashScoreMultiplier
		scores[match.FolderPath] = &folderScore{
			FolderPath:   match.FolderPath,
			MinHashScore: mhScore,
			BestMethod:   "minhash",
		}
	}

	// Step 2: MinHashスコアが閾値未満ならIR問い合わせ
	bestMinHash := 0.0
	for _, fs := range scores {
		if fs.MinHashScore > bestMinHash {
			bestMinHash = fs.MinHashScore
		}
	}

	title := parsed.Title
	artist := parsed.Artist

	if bestMinHash < irSkipThreshold {
		irResp, err := u.irClient.LookupByMD5(ctx, parsed.MD5)
		if err == nil && irResp != nil && irResp.Registered {
			u.saveIRResponse(ctx, parsed.MD5, irResp)
			// IR情報でEstimateInstallLocationを実行
			title = irResp.Title
			artist = irResp.Artist
		}
	}

	// Step 3: EstimateInstallLocation（IR情報またはパースしたtitle/artist）
	if title != "" {
		metaCandidates, err := u.estimateUseCase.Execute(ctx, title, artist, parsed.MD5)
		if err == nil {
			for _, mc := range metaCandidates {
				fs, ok := scores[mc.FolderPath]
				if ok {
					fs.MetadataScore = float64(mc.Score)
					// MinHash + メタデータ両方あればbestMethodはスコアが高い方
					if fs.MetadataScore > fs.MinHashScore {
						fs.BestMethod = bestMethodFromMatchTypes(mc.MatchTypes)
					}
				} else {
					scores[mc.FolderPath] = &folderScore{
						FolderPath:    mc.FolderPath,
						MetadataScore: float64(mc.Score),
						BestMethod:    bestMethodFromMatchTypes(mc.MatchTypes),
					}
				}
			}
		}
	}

	// Step 4: 統合スコア最上位を選択
	if len(scores) == 0 {
		return candidate, nil
	}

	var all []*folderScore
	for _, fs := range scores {
		all = append(all, fs)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Total() > all[j].Total()
	})

	best := all[0]
	candidate.DestFolder = best.FolderPath
	candidate.Score = best.Total()
	candidate.MatchMethod = best.BestMethod
	return candidate, nil
}

func bestMethodFromMatchTypes(matchTypes []string) string {
	for _, mt := range matchTypes {
		if mt == "body_url" {
			return "ir"
		}
	}
	return "title"
}

func (u *EstimateDiffInstallUseCase) saveIRResponse(ctx context.Context, md5 string, resp *port.IRResponse) {
	now := time.Now()
	meta := model.ChartIRMeta{
		MD5:          md5,
		Tags:         resp.Tags,
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
		FetchedAt:    &now,
	}
	u.metaRepo.UpsertChartMeta(ctx, meta)
}
