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

func setupRepo(t *testing.T) *persistence.ElsaRepository {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	return persistence.NewElsaRepository(db)
}

func TestUpsertAndGetSongMeta(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	year := 2023
	eventID := "EX-001"
	meta := model.SongMeta{
		FolderHash:  "abc123",
		ReleaseYear: &year,
		EventID:     &eventID,
	}

	// Insert
	if err := repo.UpsertSongMeta(ctx, meta); err != nil {
		t.Fatalf("UpsertSongMeta failed: %v", err)
	}

	// Get して全フィールドを検証
	got, err := repo.GetSongMeta(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetSongMeta failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetSongMeta returned nil")
	}
	if got.FolderHash != "abc123" {
		t.Errorf("FolderHash = %q, want %q", got.FolderHash, "abc123")
	}
	if got.ReleaseYear == nil || *got.ReleaseYear != 2023 {
		t.Errorf("ReleaseYear = %v, want 2023", got.ReleaseYear)
	}
	if got.EventID == nil || *got.EventID != "EX-001" {
		t.Errorf("EventID = %v, want EX-001", got.EventID)
	}

	// Update
	updatedEventID := "EX-002"
	meta.EventID = &updatedEventID
	if err := repo.UpsertSongMeta(ctx, meta); err != nil {
		t.Fatalf("UpsertSongMeta (update) failed: %v", err)
	}

	got, err = repo.GetSongMeta(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetSongMeta after update failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetSongMeta after update returned nil")
	}
	if got.EventID == nil || *got.EventID != "EX-002" {
		t.Errorf("EventID after update = %v, want EX-002", got.EventID)
	}
}

func TestGetSongMeta_NotFound(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	got, err := repo.GetSongMeta(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSongMeta should not return error for missing row: %v", err)
	}
	if got != nil {
		t.Fatalf("GetSongMeta should return nil for missing row, got %+v", got)
	}
}

func TestUpsertAndGetChartMeta(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	meta := model.ChartIRMeta{
		MD5:            "aaa",
		SHA256:         "bbb",
		Tags:           []string{"Stella", "st2"},
		LR2IRBodyURL:   "http://example.com",
		LR2IRDiffURL:   "http://example.com/diff",
		LR2IRNotes:     "some notes",
		FetchedAt:      &now,
	}

	if err := repo.UpsertChartMeta(ctx, meta); err != nil {
		t.Fatalf("UpsertChartMeta failed: %v", err)
	}

	got, err := repo.GetChartMeta(ctx, "aaa")
	if err != nil {
		t.Fatalf("GetChartMeta failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetChartMeta returned nil")
	}

	if got.MD5 != "aaa" {
		t.Errorf("MD5 = %q, want %q", got.MD5, "aaa")
	}
	if got.SHA256 != "bbb" {
		t.Errorf("SHA256 = %q, want %q", got.SHA256, "bbb")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "Stella" || got.Tags[1] != "st2" {
		t.Errorf("Tags = %v, want [Stella st2]", got.Tags)
	}
	if got.LR2IRBodyURL != "http://example.com" {
		t.Errorf("LR2IRBodyURL = %q, want %q", got.LR2IRBodyURL, "http://example.com")
	}
	if got.FetchedAt == nil {
		t.Fatal("FetchedAt should not be nil")
	}
	if !got.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, now)
	}
}

func TestBulkUpsertChartMeta(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	metas := []model.ChartIRMeta{
		{MD5: "m1", SHA256: "s1", Tags: []string{"tag1"}, LR2IRBodyURL: "url1"},
		{MD5: "m2", SHA256: "s2", Tags: []string{"tag2"}, LR2IRBodyURL: "url2"},
		{MD5: "m3", SHA256: "s3", Tags: []string{"tag3"}, LR2IRBodyURL: "url3"},
	}

	if err := repo.BulkUpsertChartMeta(ctx, metas); err != nil {
		t.Fatalf("BulkUpsertChartMeta failed: %v", err)
	}

	// 各レコードが取得できることを検証
	for _, m := range metas {
		got, err := repo.GetChartMeta(ctx, m.MD5)
		if err != nil {
			t.Fatalf("GetChartMeta(%q) failed: %v", m.MD5, err)
		}
		if got == nil {
			t.Fatalf("GetChartMeta(%q) returned nil", m.MD5)
		}
		if got.LR2IRBodyURL != m.LR2IRBodyURL {
			t.Errorf("LR2IRBodyURL = %q, want %q", got.LR2IRBodyURL, m.LR2IRBodyURL)
		}
	}
}


func TestUpsertChartMeta_Update(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	meta := model.ChartIRMeta{
		MD5:          "aaa",
		SHA256:       "bbb",
		Tags:         []string{"old-tag"},
		LR2IRBodyURL: "http://old.example.com",
	}

	if err := repo.UpsertChartMeta(ctx, meta); err != nil {
		t.Fatalf("UpsertChartMeta (insert) failed: %v", err)
	}

	// 更新
	now := time.Now().Truncate(time.Second)
	meta.Tags = []string{"new-tag1", "new-tag2"}
	meta.LR2IRBodyURL = "http://new.example.com"
	meta.FetchedAt = &now

	if err := repo.UpsertChartMeta(ctx, meta); err != nil {
		t.Fatalf("UpsertChartMeta (update) failed: %v", err)
	}

	got, err := repo.GetChartMeta(ctx, "aaa")
	if err != nil {
		t.Fatalf("GetChartMeta after update failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetChartMeta after update returned nil")
	}
	if got.LR2IRBodyURL != "http://new.example.com" {
		t.Errorf("LR2IRBodyURL = %q, want %q", got.LR2IRBodyURL, "http://new.example.com")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "new-tag1" || got.Tags[1] != "new-tag2" {
		t.Errorf("Tags = %v, want [new-tag1 new-tag2]", got.Tags)
	}
	if got.FetchedAt == nil || !got.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, now)
	}
}
