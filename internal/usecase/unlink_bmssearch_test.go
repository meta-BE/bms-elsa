package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type fakeFolderMD5sResolver struct {
	md5s map[string][]string
}

func (f *fakeFolderMD5sResolver) ListMD5sByFolder(_ context.Context, folderHash string) ([]string, error) {
	return f.md5s[folderHash], nil
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

func TestUnlinkByMD5(t *testing.T) {
	bmssearchRepo := newFakeBMSSearchRepo()
	now := time.Now()
	_ = bmssearchRepo.UpsertLinks(context.Background(), []model.BMSSearchLink{
		{MD5: "morph", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})
	metaRepo := &mockMetaRepo{}
	folderResolver := &fakeFolderMD5sResolver{}

	uc := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, metaRepo, folderResolver)
	if err := uc.UnlinkByMD5(context.Background(), "morph"); err != nil {
		t.Fatal(err)
	}
	if bmssearchRepo.links["morph"] != nil {
		t.Errorf("link should be deleted")
	}
}
