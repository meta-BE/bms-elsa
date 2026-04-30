package persistence_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

func newTestDBWithMigration(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBMSSearchRepository_UpsertAndGetLink(t *testing.T) {
	db := newTestDBWithMigration(t)
	defer db.Close()
	repo := persistence.NewBMSSearchRepository(db)
	ctx := context.Background()

	now := time.Unix(1700000000, 0)
	links := []model.BMSSearchLink{
		{MD5: "md5a", BMSID: "bms-1", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "md5b", BMSID: "bms-1", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	}
	if err := repo.UpsertLinks(ctx, links); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetLinkByMD5(ctx, "md5a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.BMSID != "bms-1" || got.Source != model.BMSSearchSourceOfficial {
		t.Errorf("got = %+v", got)
	}

	// UPSERT: source 上書き
	links2 := []model.BMSSearchLink{
		{MD5: "md5a", BMSID: "bms-2", Source: model.BMSSearchSourceUnofficial, ResolvedAt: now},
	}
	if err := repo.UpsertLinks(ctx, links2); err != nil {
		t.Fatal(err)
	}
	got2, _ := repo.GetLinkByMD5(ctx, "md5a")
	if got2.BMSID != "bms-2" || got2.Source != model.BMSSearchSourceUnofficial {
		t.Errorf("got2 = %+v, expected upsert", got2)
	}
}

func TestBMSSearchRepository_DeleteLinks(t *testing.T) {
	db := newTestDBWithMigration(t)
	defer db.Close()
	repo := persistence.NewBMSSearchRepository(db)
	ctx := context.Background()

	now := time.Unix(1700000000, 0)
	_ = repo.UpsertLinks(ctx, []model.BMSSearchLink{
		{MD5: "x", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "y", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "z", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})

	if err := repo.DeleteLinkByMD5(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetLinkByMD5(ctx, "x")
	if got != nil {
		t.Errorf("got = %+v, want nil after delete", got)
	}

	if err := repo.DeleteLinksByMD5s(ctx, []string{"y", "z"}); err != nil {
		t.Fatal(err)
	}
	g, _ := repo.GetLinkByMD5(ctx, "y")
	if g != nil {
		t.Errorf("y should be deleted")
	}
}

func TestBMSSearchRepository_UpsertAndGetBMS(t *testing.T) {
	db := newTestDBWithMigration(t)
	defer db.Close()
	repo := persistence.NewBMSSearchRepository(db)
	ctx := context.Background()

	exID := "ex-1"
	bms := model.BMSSearchBMS{
		BMSID:          "bms-1",
		Title:          "Test Song",
		Artist:         "Artist",
		SubArtist:      "feat. X",
		Genre:          "TECHNO",
		ExhibitionID:   &exID,
		ExhibitionName: "BOFXX",
		PublishedAt:    "2024-08-01T00:00:00Z",
		Downloads: []model.BMSSearchURLEntry{
			{URL: "https://dl.example.com/x.zip", Description: "本体"},
		},
		Previews: []model.BMSSearchPreview{
			{Service: "YOUTUBE", Parameter: "abc123"},
		},
		RelatedLinks: []model.BMSSearchURLEntry{
			{URL: "https://twitter.com/a", Description: "作者"},
		},
		FetchedAt: time.Unix(1700000000, 0),
	}
	if err := repo.UpsertBMS(ctx, bms); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetBMSByID(ctx, "bms-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Title != "Test Song" || got.ExhibitionID == nil || *got.ExhibitionID != "ex-1" {
		t.Errorf("got = %+v", got)
	}
	if len(got.Downloads) != 1 || got.Downloads[0].URL != "https://dl.example.com/x.zip" {
		t.Errorf("downloads = %+v", got.Downloads)
	}
	if len(got.Previews) != 1 || got.Previews[0].Service != "YOUTUBE" {
		t.Errorf("previews = %+v", got.Previews)
	}

	// 独立曲（exhibition_id=NULL）
	bms2 := model.BMSSearchBMS{BMSID: "bms-2", Title: "Solo", FetchedAt: time.Unix(1700000001, 0)}
	if err := repo.UpsertBMS(ctx, bms2); err != nil {
		t.Fatal(err)
	}
	got2, _ := repo.GetBMSByID(ctx, "bms-2")
	if got2 == nil || got2.ExhibitionID != nil {
		t.Errorf("got2.ExhibitionID = %v, want nil", got2)
	}
}
