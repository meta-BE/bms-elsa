package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// 共有モック（必要な機能だけ実装）
type fakeBMSClient struct {
	patternFn func(ctx context.Context, md5 string) (*gateway.BMSSearchPattern, error)
	bmsFn     func(ctx context.Context, id string) (*gateway.BMSSearchBMS, error)
	searchFn  func(ctx context.Context, title string, limit int) ([]gateway.BMSSearchBMS, error)
}

func (f *fakeBMSClient) LookupPatternByMD5(ctx context.Context, md5 string) (*gateway.BMSSearchPattern, error) {
	return f.patternFn(ctx, md5)
}
func (f *fakeBMSClient) LookupBMS(ctx context.Context, id string) (*gateway.BMSSearchBMS, error) {
	return f.bmsFn(ctx, id)
}
func (f *fakeBMSClient) SearchBMSesByTitle(ctx context.Context, title string, limit int) ([]gateway.BMSSearchBMS, error) {
	return f.searchFn(ctx, title, limit)
}

type fakeBMSSearchRepo struct {
	links     map[string]*model.BMSSearchLink
	bmsCache  map[string]*model.BMSSearchBMS
	upsertFn  func([]model.BMSSearchLink)
	upsertBMS func(model.BMSSearchBMS)
}

func newFakeBMSSearchRepo() *fakeBMSSearchRepo {
	return &fakeBMSSearchRepo{
		links:    map[string]*model.BMSSearchLink{},
		bmsCache: map[string]*model.BMSSearchBMS{},
	}
}

func (f *fakeBMSSearchRepo) GetLinkByMD5(_ context.Context, md5 string) (*model.BMSSearchLink, error) {
	return f.links[md5], nil
}
func (f *fakeBMSSearchRepo) UpsertLinks(_ context.Context, links []model.BMSSearchLink) error {
	for i := range links {
		l := links[i]
		f.links[l.MD5] = &l
	}
	if f.upsertFn != nil {
		f.upsertFn(links)
	}
	return nil
}
func (f *fakeBMSSearchRepo) DeleteLinkByMD5(_ context.Context, md5 string) error {
	delete(f.links, md5)
	return nil
}
func (f *fakeBMSSearchRepo) DeleteLinksByMD5s(_ context.Context, md5s []string) error {
	for _, m := range md5s {
		delete(f.links, m)
	}
	return nil
}
func (f *fakeBMSSearchRepo) GetBMSByID(_ context.Context, id string) (*model.BMSSearchBMS, error) {
	return f.bmsCache[id], nil
}
func (f *fakeBMSSearchRepo) UpsertBMS(_ context.Context, b model.BMSSearchBMS) error {
	f.bmsCache[b.BMSID] = &b
	if f.upsertBMS != nil {
		f.upsertBMS(b)
	}
	return nil
}

func TestResolveForFolder_OfficialHit(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, md5 string) (*gateway.BMSSearchPattern, error) {
			if md5 == "md5b" {
				p := &gateway.BMSSearchPattern{}
				p.BMS.ID = "bms-1"
				return p, nil
			}
			return nil, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "T", Artist: "A"}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) {
			t.Errorf("search should not be called when official hit succeeds")
			return nil, nil
		},
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{
		updateSongMetaBMSSearchFn: func(_ context.Context, _, _, _ string) error { return nil },
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForFolder(context.Background(), "folder1", []string{"md5a", "md5b", "md5c"}, "T", "A")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "bms-1" || source != model.BMSSearchSourceOfficial {
		t.Errorf("got bmsID=%q source=%q", bmsID, source)
	}
	// 全 md5 がリンクされていること
	for _, m := range []string{"md5a", "md5b", "md5c"} {
		if l := bmssearchRepo.links[m]; l == nil || l.BMSID != "bms-1" || l.Source != model.BMSSearchSourceOfficial {
			t.Errorf("link[%s] = %+v", m, l)
		}
	}
	// bmssearch_bms に保存されていること
	if bmssearchRepo.bmsCache["bms-1"] == nil {
		t.Errorf("bms not cached")
	}
}

func TestResolveForFolder_FallbackHit(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) { return nil, nil },
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "Test Song", Artist: "Artist"}, nil
		},
		searchFn: func(_ context.Context, title string, _ int) ([]gateway.BMSSearchBMS, error) {
			return []gateway.BMSSearchBMS{
				{ID: "bms-x", Title: "Test Song", Artist: "Artist"},
				{ID: "bms-y", Title: "Different", Artist: "Other"},
			}, nil
		},
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{
		updateSongMetaBMSSearchFn: func(_ context.Context, _, _, _ string) error { return nil },
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForFolder(context.Background(), "f1", []string{"md5a"}, "Test Song", "Artist")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "bms-x" || source != model.BMSSearchSourceUnofficial {
		t.Errorf("got bmsID=%q source=%q", bmsID, source)
	}
}

func TestResolveForFolder_BothFail(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) { return nil, nil },
		bmsFn:     func(_ context.Context, _ string) (*gateway.BMSSearchBMS, error) { return nil, nil },
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) {
			return []gateway.BMSSearchBMS{
				{ID: "bms-z", Title: "Foo", Artist: "Bar"},
			}, nil
		},
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForFolder(context.Background(), "f1", []string{"md5a"}, "Test Song", "Artist")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "" || source != "" {
		t.Errorf("got bmsID=%q source=%q, want empty", bmsID, source)
	}
	if len(bmssearchRepo.links) != 0 {
		t.Errorf("no link should be written, got %+v", bmssearchRepo.links)
	}
}

func TestResolveForOrphanMD5_OfficialHit(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) {
			p := &gateway.BMSSearchPattern{}
			p.BMS.ID = "bms-1"
			return p, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}

	called := false
	metaRepo.updateSongMetaBMSSearchFn = func(_ context.Context, _, _, _ string) error {
		called = true
		return nil
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForOrphanMD5(context.Background(), "md5x", "T", "A")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "bms-1" || source != model.BMSSearchSourceOfficial {
		t.Errorf("got bmsID=%q source=%q", bmsID, source)
	}
	if called {
		t.Error("ResolveForOrphanMD5 should NOT touch song_meta")
	}
	if l := bmssearchRepo.links["md5x"]; l == nil {
		t.Error("link should be saved")
	}
}
