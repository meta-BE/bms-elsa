package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type fakeFolderMD5sResolver struct {
	md5s       map[string][]string                // folderHash -> md5s
	folderInfo map[string]fakeFolderInfoForUnlink // md5 -> folder info
}

type fakeFolderInfoForUnlink struct {
	folderHash string
	md5s       []string
	title      string
	artist     string
	owned      bool
}

func (f *fakeFolderMD5sResolver) ListMD5sByFolder(_ context.Context, folderHash string) ([]string, error) {
	return f.md5s[folderHash], nil
}

func (f *fakeFolderMD5sResolver) FindFolderInfoByMD5(_ context.Context, md5 string) (string, []string, string, string, bool, error) {
	info, ok := f.folderInfo[md5]
	if !ok {
		return "", nil, "", "", false, nil
	}
	return info.folderHash, info.md5s, info.title, info.artist, info.owned, nil
}

func TestUnlinkByFolder(t *testing.T) {
	bmssearchRepo := newFakeBMSSearchRepo()
	now := time.Now()
	_ = bmssearchRepo.UpsertLinks(context.Background(), []model.BMSSearchLink{
		{MD5: "m1", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "m2", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "m9", BMSID: "z", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})

	var clearedFolder string
	metaRepo := &mockMetaRepo{
		clearSongMetaBMSSearchFn: func(_ context.Context, fh string) error {
			clearedFolder = fh
			return nil
		},
	}
	folderResolver := &fakeFolderMD5sResolver{
		md5s: map[string][]string{"folder1": {"m1", "m2"}},
	}

	uc := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, metaRepo, folderResolver)
	if err := uc.UnlinkByFolder(context.Background(), "folder1"); err != nil {
		t.Fatal(err)
	}
	if clearedFolder != "folder1" {
		t.Errorf("metaRepo folderHash wrong: %q", clearedFolder)
	}
	if bmssearchRepo.links["m1"] != nil || bmssearchRepo.links["m2"] != nil {
		t.Errorf("links not deleted")
	}
	if bmssearchRepo.links["m9"] == nil {
		t.Errorf("m9 should remain")
	}
}

func TestUnlinkByMD5_Orphan(t *testing.T) {
	bmssearchRepo := newFakeBMSSearchRepo()
	now := time.Now()
	_ = bmssearchRepo.UpsertLinks(context.Background(), []model.BMSSearchLink{
		{MD5: "morph", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})
	var cleared bool
	metaRepo := &mockMetaRepo{
		clearSongMetaBMSSearchFn: func(_ context.Context, _ string) error {
			cleared = true
			return nil
		},
	}
	folderResolver := &fakeFolderMD5sResolver{}

	uc := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, metaRepo, folderResolver)
	if err := uc.UnlinkByMD5(context.Background(), "morph"); err != nil {
		t.Fatal(err)
	}
	if bmssearchRepo.links["morph"] != nil {
		t.Errorf("link should be deleted")
	}
	if cleared {
		t.Errorf("ClearSongMetaBMSSearch should NOT be called for orphan md5")
	}
}

// 難易度表からの解除（所持譜面）でも song_meta を確実にクリアし、
// フォルダ内全 md5 のリンクを削除することを検証する
func TestUnlinkByMD5_Owned(t *testing.T) {
	bmssearchRepo := newFakeBMSSearchRepo()
	now := time.Now()
	_ = bmssearchRepo.UpsertLinks(context.Background(), []model.BMSSearchLink{
		{MD5: "m1", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "m2", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "m9", BMSID: "z", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})

	var clearedFolder string
	metaRepo := &mockMetaRepo{
		clearSongMetaBMSSearchFn: func(_ context.Context, fh string) error {
			clearedFolder = fh
			return nil
		},
	}
	folderResolver := &fakeFolderMD5sResolver{
		md5s: map[string][]string{"folder1": {"m1", "m2"}},
		folderInfo: map[string]fakeFolderInfoForUnlink{
			"m1": {folderHash: "folder1", md5s: []string{"m1", "m2"}, owned: true},
		},
	}

	uc := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, metaRepo, folderResolver)
	if err := uc.UnlinkByMD5(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
	if clearedFolder != "folder1" {
		t.Errorf("song_meta should be cleared for folder1, got %q", clearedFolder)
	}
	if bmssearchRepo.links["m1"] != nil || bmssearchRepo.links["m2"] != nil {
		t.Errorf("all md5 links in folder should be deleted")
	}
	if bmssearchRepo.links["m9"] == nil {
		t.Errorf("m9 (other folder) should remain")
	}
}
