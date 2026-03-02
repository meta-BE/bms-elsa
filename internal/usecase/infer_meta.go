package usecase

import (
	"context"
	"strings"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// InferSongMetaUseCase はURLパターンマッチングによる楽曲メタデータ自動推測のユースケース
type InferSongMetaUseCase struct {
	metaRepo model.MetaRepository
}

func NewInferSongMetaUseCase(metaRepo model.MetaRepository) *InferSongMetaUseCase {
	return &InferSongMetaUseCase{metaRepo: metaRepo}
}

// InferenceResult は自動推測の結果
type InferenceResult struct {
	AutoSetCount   int
	UnmatchedSongs []model.SongIRURLs
	NoIRCount      int // IR未取得の曲数（UnmatchedSongsの内数）
}

func (u *InferSongMetaUseCase) RunAutoInference(ctx context.Context) (*InferenceResult, error) {
	// 1. マッピングテーブル取得
	mappings, err := u.metaRepo.ListEventMappings(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 未設定曲＋IR URL取得
	songs, err := u.metaRepo.ListUnsetSongsWithIRURLs(ctx)
	if err != nil {
		return nil, err
	}

	// 3. マッチング
	var autoSet int
	var unmatched []model.SongIRURLs
	for _, song := range songs {
		if matched := matchURL(song.BodyURLs, mappings); matched != nil {
			year := matched.ReleaseYear
			name := matched.EventName
			u.metaRepo.UpsertSongMeta(ctx, model.SongMeta{
				FolderHash: song.FolderHash, ReleaseYear: &year, EventName: &name,
			})
			autoSet++
		} else {
			unmatched = append(unmatched, song)
		}
	}

	// NoIRCount集計
	noIR := 0
	for _, s := range unmatched {
		if s.IRCount == 0 {
			noIR++
		}
	}

	return &InferenceResult{AutoSetCount: autoSet, UnmatchedSongs: unmatched, NoIRCount: noIR}, nil
}

// matchURL: URLリストの中にマッピングのpatternを含むものがあるか
// パターンは "|" 区切りで複数条件AND対応（例: "manbow.nothing.sh|&event=104"）
func matchURL(urls []string, mappings []model.EventMapping) *model.EventMapping {
	for _, url := range urls {
		for i, m := range mappings {
			parts := strings.Split(m.URLPattern, "|")
			allMatch := true
			for _, p := range parts {
				if !strings.Contains(url, p) {
					allMatch = false
					break
				}
			}
			if allMatch {
				return &mappings[i]
			}
		}
	}
	return nil
}
