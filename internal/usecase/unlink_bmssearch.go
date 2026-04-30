package usecase

import (
	"context"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// FolderMD5sResolver はフォルダに含まれる md5 一覧を返す
type FolderMD5sResolver interface {
	ListMD5sByFolder(ctx context.Context, folderHash string) ([]string, error)
}

type UnlinkBMSSearchUseCase struct {
	bmssearchRepo  model.BMSSearchRepository
	metaRepo       model.MetaRepository
	folderResolver FolderMD5sResolver
}

func NewUnlinkBMSSearchUseCase(
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
	folderResolver FolderMD5sResolver,
) *UnlinkBMSSearchUseCase {
	return &UnlinkBMSSearchUseCase{
		bmssearchRepo:  bmssearchRepo,
		metaRepo:       metaRepo,
		folderResolver: folderResolver,
	}
}

// UnlinkByFolder は楽曲フォルダ単位の解除（song_meta.bms_search_id/source を NULL にし、
// フォルダ内全 md5 の bmssearch_bms_id_md5 を DELETE）
func (u *UnlinkBMSSearchUseCase) UnlinkByFolder(ctx context.Context, folderHash string) error {
	if err := u.metaRepo.ClearSongMetaBMSSearch(ctx, folderHash); err != nil {
		return fmt.Errorf("UnlinkByFolder ClearSongMetaBMSSearch: %w", err)
	}
	md5s, err := u.folderResolver.ListMD5sByFolder(ctx, folderHash)
	if err != nil {
		return fmt.Errorf("UnlinkByFolder ListMD5sByFolder: %w", err)
	}
	if len(md5s) == 0 {
		return nil
	}
	if err := u.bmssearchRepo.DeleteLinksByMD5s(ctx, md5s); err != nil {
		return fmt.Errorf("UnlinkByFolder DeleteLinksByMD5s: %w", err)
	}
	return nil
}

// UnlinkByMD5 は未所持 md5 単位の解除
func (u *UnlinkBMSSearchUseCase) UnlinkByMD5(ctx context.Context, md5 string) error {
	if err := u.bmssearchRepo.DeleteLinkByMD5(ctx, md5); err != nil {
		return fmt.Errorf("UnlinkByMD5 DeleteLinkByMD5: %w", err)
	}
	return nil
}
