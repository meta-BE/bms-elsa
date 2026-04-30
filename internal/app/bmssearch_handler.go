package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type BMSSearchHandler struct {
	ctx            context.Context
	lookupUC       *usecase.LookupBMSSearchUseCase
	unlinkUC       *usecase.UnlinkBMSSearchUseCase
	bmssearchRepo  model.BMSSearchRepository
	metaRepo       model.MetaRepository
	folderResolver usecase.ChartFolderResolver
}

func NewBMSSearchHandler(
	lookupUC *usecase.LookupBMSSearchUseCase,
	unlinkUC *usecase.UnlinkBMSSearchUseCase,
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
	folderResolver usecase.ChartFolderResolver,
) *BMSSearchHandler {
	return &BMSSearchHandler{
		lookupUC:       lookupUC,
		unlinkUC:       unlinkUC,
		bmssearchRepo:  bmssearchRepo,
		metaRepo:       metaRepo,
		folderResolver: folderResolver,
	}
}

func (h *BMSSearchHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// GetBMSSearchInfoByMD5 は DB のみから情報を取得する（API 呼び出しなし）。詳細画面の初期表示用。
func (h *BMSSearchHandler) GetBMSSearchInfoByMD5(md5 string) (*dto.BMSSearchInfoDTO, error) {
	folderHash, _, _, _, owned, err := h.folderResolver.FindFolderInfoByMD5(h.ctx, md5)
	if err != nil {
		return nil, err
	}
	var bmsID string
	var source string
	if owned {
		meta, err := h.metaRepo.GetSongMeta(h.ctx, folderHash)
		if err != nil {
			return nil, err
		}
		if meta == nil || meta.BMSSearchID == nil {
			return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
		}
		bmsID = *meta.BMSSearchID
		if meta.BMSSearchSource != nil {
			source = *meta.BMSSearchSource
		}
	} else {
		link, err := h.bmssearchRepo.GetLinkByMD5(h.ctx, md5)
		if err != nil {
			return nil, err
		}
		if link == nil {
			return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
		}
		bmsID = link.BMSID
		source = string(link.Source)
	}
	bms, err := h.bmssearchRepo.GetBMSByID(h.ctx, bmsID)
	if err != nil {
		return nil, err
	}
	if bms == nil {
		return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
	}
	return dto.BMSSearchBMSToDTO(*bms, source), nil
}

// LookupBMSSearchByMD5 は「取得」ボタン押下時。Resolver 経由で取得＆保存。
func (h *BMSSearchHandler) LookupBMSSearchByMD5(md5 string) (*dto.BMSSearchInfoDTO, error) {
	return h.lookupUC.Execute(h.ctx, md5)
}

// UnlinkBMSSearchByFolder は所持譜面の解除（song_meta + 全 md5 リンク削除）。
func (h *BMSSearchHandler) UnlinkBMSSearchByFolder(folderHash string) error {
	return h.unlinkUC.UnlinkByFolder(h.ctx, folderHash)
}

// UnlinkBMSSearchByMD5 は未所持 md5 の解除。
func (h *BMSSearchHandler) UnlinkBMSSearchByMD5(md5 string) error {
	return h.unlinkUC.UnlinkByMD5(h.ctx, md5)
}
