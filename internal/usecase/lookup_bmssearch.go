package usecase

import (
	"context"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// ChartFolderResolver は md5 から所属フォルダや難易度表エントリ情報を解決する
// （SongdataReader と DifficultyTableRepository を組み合わせる）
type ChartFolderResolver interface {
	// FindFolderInfoByMD5 は md5 が所属する楽曲フォルダ情報を返す。
	// 戻り値: folderHash, フォルダ内全 md5, 楽曲タイトル, 楽曲アーティスト, 見つかったか
	FindFolderInfoByMD5(ctx context.Context, md5 string) (string, []string, string, string, bool, error)

	// FindOrphanInfoByMD5 は未所持 md5 の難易度表エントリから title/artist を解決する。
	FindOrphanInfoByMD5(ctx context.Context, md5 string) (string, string, bool, error)
}

type LookupBMSSearchUseCase struct {
	resolver       *BMSSearchResolver
	folderResolver ChartFolderResolver
	bmssearchRepo  model.BMSSearchRepository
}

func NewLookupBMSSearchUseCase(
	resolver *BMSSearchResolver,
	folderResolver ChartFolderResolver,
	bmssearchRepo model.BMSSearchRepository,
) *LookupBMSSearchUseCase {
	return &LookupBMSSearchUseCase{
		resolver:       resolver,
		folderResolver: folderResolver,
		bmssearchRepo:  bmssearchRepo,
	}
}

func (u *LookupBMSSearchUseCase) Execute(ctx context.Context, md5 string) (*dto.BMSSearchInfoDTO, error) {
	var bmsID string
	var source model.BMSSearchSource

	folderHash, md5s, title, artist, ownedFound, err := u.folderResolver.FindFolderInfoByMD5(ctx, md5)
	if err != nil {
		return nil, fmt.Errorf("LookupBMSSearch FindFolderInfoByMD5: %w", err)
	}
	if ownedFound {
		bmsID, source, err = u.resolver.ResolveForFolder(ctx, folderHash, md5s, title, artist)
	} else {
		t, a, found, err2 := u.folderResolver.FindOrphanInfoByMD5(ctx, md5)
		if err2 != nil {
			return nil, fmt.Errorf("LookupBMSSearch FindOrphanInfoByMD5: %w", err2)
		}
		if !found {
			return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
		}
		bmsID, source, err = u.resolver.ResolveForOrphanMD5(ctx, md5, t, a)
	}
	if err != nil {
		return nil, fmt.Errorf("LookupBMSSearch resolve: %w", err)
	}
	if bmsID == "" {
		return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
	}
	bms, err := u.bmssearchRepo.GetBMSByID(ctx, bmsID)
	if err != nil {
		return nil, fmt.Errorf("LookupBMSSearch GetBMSByID: %w", err)
	}
	if bms == nil {
		return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
	}
	return dto.BMSSearchBMSToDTO(*bms, string(source)), nil
}
