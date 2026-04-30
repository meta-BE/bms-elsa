package usecase

import (
	"context"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// FolderMD5sResolver はフォルダに含まれる md5 一覧、および md5 から所属フォルダ情報を返す
type FolderMD5sResolver interface {
	ListMD5sByFolder(ctx context.Context, folderHash string) ([]string, error)
	// FindFolderInfoByMD5 は md5 が所属する楽曲フォルダ情報を返す。
	// 戻り値: folderHash, フォルダ内全 md5, 楽曲タイトル, 楽曲アーティスト, 所持されているか
	FindFolderInfoByMD5(ctx context.Context, md5 string) (string, []string, string, string, bool, error)
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

// UnlinkByMD5 は md5 単位の解除。
// md5 が所持譜面に属する場合は UnlinkByFolder と同等の処理（song_meta クリア + フォルダ全 md5 のリンク削除）を行い、
// 未所持の場合は当該 md5 のリンクのみ削除する。
func (u *UnlinkBMSSearchUseCase) UnlinkByMD5(ctx context.Context, md5 string) error {
	folderHash, _, _, _, owned, err := u.folderResolver.FindFolderInfoByMD5(ctx, md5)
	if err != nil {
		return fmt.Errorf("UnlinkByMD5 FindFolderInfoByMD5: %w", err)
	}
	if owned {
		return u.UnlinkByFolder(ctx, folderHash)
	}
	if err := u.bmssearchRepo.DeleteLinkByMD5(ctx, md5); err != nil {
		return fmt.Errorf("UnlinkByMD5 DeleteLinkByMD5: %w", err)
	}
	return nil
}
