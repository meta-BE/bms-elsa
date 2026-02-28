package persistence

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupDTRepo(t *testing.T) *DifficultyTableRepository {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return NewDifficultyTableRepository(db)
}

func TestListEntries(t *testing.T) {
	repo := setupDTRepo(t)
	ctx := context.Background()

	tableID, err := repo.InsertTable(ctx, DifficultyTable{
		URL: "http://example.com", HeaderURL: "http://example.com/header.json",
		DataURL: "http://example.com/body.json", Name: "Test Table", Symbol: "T",
	})
	if err != nil {
		t.Fatal(err)
	}

	entries := []DifficultyTableEntry{
		{TableID: tableID, MD5: "aaa", Level: "1", Title: "Song A", Artist: "Artist A", URL: "http://dl.example.com/a", URLDiff: ""},
		{TableID: tableID, MD5: "bbb", Level: "2", Title: "Song B", Artist: "Artist B", URL: "", URLDiff: "http://dl.example.com/b"},
	}
	if err := repo.ReplaceEntries(ctx, tableID, entries); err != nil {
		t.Fatal(err)
	}

	result, err := repo.ListEntries(ctx, tableID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].MD5 != "aaa" || result[0].Level != "1" {
		t.Errorf("unexpected first entry: %+v", result[0])
	}
}

func TestListEntries_EmptyTable(t *testing.T) {
	repo := setupDTRepo(t)
	ctx := context.Background()

	tableID, _ := repo.InsertTable(ctx, DifficultyTable{
		URL: "http://example.com", HeaderURL: "http://example.com/h",
		DataURL: "http://example.com/b", Name: "Empty", Symbol: "E",
	})

	result, err := repo.ListEntries(ctx, tableID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}
