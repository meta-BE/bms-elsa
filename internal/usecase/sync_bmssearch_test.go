package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// TestSyncBMSSearch_FallbackResolution は、公式 Pattern API が miss しフォールバック検索で
// BMSSearchResolver が解決したとき、bmssearch_bms_id_md5 / bmssearch_bms への書き込みと
// song_meta.bms_search_id, bms_search_source = 'unofficial' が正しく記録されることを検証する。
func TestSyncBMSSearch_FallbackResolution(t *testing.T) {
	// --- fake BMS クライアント: 公式 API は miss、タイトル検索はヒット ---
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) {
			return nil, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "Fallback Song", Artist: "FA"}, nil
		},
		searchFn: func(_ context.Context, title string, _ int) ([]gateway.BMSSearchBMS, error) {
			return []gateway.BMSSearchBMS{
				{ID: "bms-fb", Title: title, Artist: "FA"},
			}, nil
		},
	}

	bmssearchRepo := newFakeBMSSearchRepo()

	var savedFolderHash, savedBMSID, savedSource string
	metaRepo := &mockMetaRepo{
		updateSongMetaBMSSearchFn: func(_ context.Context, folderHash, bmsID, source string) error {
			savedFolderHash = folderHash
			savedBMSID = bmsID
			savedSource = source
			return nil
		},
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	uc := usecase.NewSyncBMSSearchUseCase(resolver, bmssearchRepo, metaRepo)

	folders := []string{"folder-fb"}
	md5sByFolder := map[string][]string{"folder-fb": {"md5-a", "md5-b"}}
	titleByFolder := map[string]string{"folder-fb": "Fallback Song"}
	artistByFolder := map[string]string{"folder-fb": "FA"}

	result, err := uc.Execute(
		context.Background(),
		folders,
		md5sByFolder,
		titleByFolder,
		artistByFolder,
		nil,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Execute の集計値検証
	if result.Total != 1 || result.Synced != 1 || result.NotFound != 0 {
		t.Errorf("result = %+v, want Total=1 Synced=1 NotFound=0", result)
	}

	// bmssearch_bms_id_md5（リンクテーブル）の書き込み検証
	for _, md5 := range []string{"md5-a", "md5-b"} {
		l := bmssearchRepo.links[md5]
		if l == nil {
			t.Errorf("link[%s] が書き込まれていない", md5)
			continue
		}
		if l.BMSID != "bms-fb" {
			t.Errorf("link[%s].BMSID = %q, want %q", md5, l.BMSID, "bms-fb")
		}
		if l.Source != model.BMSSearchSourceUnofficial {
			t.Errorf("link[%s].Source = %q, want unofficial", md5, l.Source)
		}
	}

	// bmssearch_bms（キャッシュテーブル）の書き込み検証
	if bmssearchRepo.bmsCache["bms-fb"] == nil {
		t.Error("bmssearch_bms に bms-fb が書き込まれていない")
	}

	// song_meta.bms_search_id, bms_search_source の書き込み検証
	if savedFolderHash != "folder-fb" {
		t.Errorf("UpdateSongMetaBMSSearch folderHash = %q, want %q", savedFolderHash, "folder-fb")
	}
	if savedBMSID != "bms-fb" {
		t.Errorf("UpdateSongMetaBMSSearch bmsID = %q, want %q", savedBMSID, "bms-fb")
	}
	if savedSource != string(model.BMSSearchSourceUnofficial) {
		t.Errorf("UpdateSongMetaBMSSearch source = %q, want %q", savedSource, model.BMSSearchSourceUnofficial)
	}
}
