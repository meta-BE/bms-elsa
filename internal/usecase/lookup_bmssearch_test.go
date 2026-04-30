package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type fakeChartFolderResolver struct {
	folderFn func(md5 string) (folderHash string, md5sInFolder []string, title, artist string, found bool)
	entryFn  func(md5 string) (title, artist string, found bool)
}

func (f *fakeChartFolderResolver) FindFolderInfoByMD5(_ context.Context, md5 string) (string, []string, string, string, bool, error) {
	fh, m, t, a, ok := f.folderFn(md5)
	return fh, m, t, a, ok, nil
}

func (f *fakeChartFolderResolver) FindOrphanInfoByMD5(_ context.Context, md5 string) (string, string, bool, error) {
	t, a, ok := f.entryFn(md5)
	return t, a, ok, nil
}

func TestLookupBMSSearch_OwnedChart(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, md5 string) (*gateway.BMSSearchPattern, error) {
			if md5 == "m1" {
				p := &gateway.BMSSearchPattern{}
				p.BMS.ID = "bms-1"
				return p, nil
			}
			return nil, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "Owned"}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}
	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)

	folderResolver := &fakeChartFolderResolver{
		folderFn: func(_ string) (string, []string, string, string, bool) {
			return "folder1", []string{"m0", "m1"}, "Owned", "Artist", true
		},
		entryFn: func(_ string) (string, string, bool) { t.Errorf("orphan path should not be hit"); return "", "", false },
	}

	uc := usecase.NewLookupBMSSearchUseCase(resolver, folderResolver, bmssearchRepo)
	dto, err := uc.Execute(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || !dto.HasInfo || dto.BMSID != "bms-1" {
		t.Errorf("dto = %+v", dto)
	}
	if dto.Source != string(model.BMSSearchSourceOfficial) {
		t.Errorf("source = %q", dto.Source)
	}
}

func TestLookupBMSSearch_OrphanMD5(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) {
			p := &gateway.BMSSearchPattern{}
			p.BMS.ID = "bms-orphan"
			return p, nil
		},
		bmsFn: func(_ context.Context, _ string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: "bms-orphan", Title: "Orphan"}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}
	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)

	folderResolver := &fakeChartFolderResolver{
		folderFn: func(_ string) (string, []string, string, string, bool) { return "", nil, "", "", false },
		entryFn:  func(_ string) (string, string, bool) { return "Orphan", "X", true },
	}

	uc := usecase.NewLookupBMSSearchUseCase(resolver, folderResolver, bmssearchRepo)
	dto, err := uc.Execute(context.Background(), "morph")
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || !dto.HasInfo || dto.BMSID != "bms-orphan" {
		t.Errorf("dto = %+v", dto)
	}
	if dto.Source != string(model.BMSSearchSourceOfficial) {
		t.Errorf("source = %q", dto.Source)
	}
}

func TestLookupBMSSearch_NotResolved(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) { return nil, nil },
		bmsFn:     func(_ context.Context, _ string) (*gateway.BMSSearchBMS, error) { return nil, nil },
		searchFn:  func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}
	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)

	folderResolver := &fakeChartFolderResolver{
		folderFn: func(_ string) (string, []string, string, string, bool) {
			return "f1", []string{"m1"}, "T", "A", true
		},
		entryFn: func(_ string) (string, string, bool) { return "", "", false },
	}

	uc := usecase.NewLookupBMSSearchUseCase(resolver, folderResolver, bmssearchRepo)
	dto, err := uc.Execute(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || dto.HasInfo {
		t.Errorf("dto = %+v, want hasInfo=false", dto)
	}
}
