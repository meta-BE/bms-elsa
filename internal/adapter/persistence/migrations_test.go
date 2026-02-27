package persistence_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
)

func TestRunMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// song_metaテーブルが存在することを確認
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM song_meta").Scan(&count)
	if err != nil {
		t.Fatalf("song_meta table not found: %v", err)
	}

	// chart_metaテーブルが存在することを確認
	err = db.QueryRow("SELECT COUNT(*) FROM chart_meta").Scan(&count)
	if err != nil {
		t.Fatalf("chart_meta table not found: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations should be idempotent: %v", err)
	}
}
