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

func TestRunMigrations_BMSSearchSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// song_meta.bms_search_source カラムが追加されている
	var hasSource int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('song_meta') WHERE name='bms_search_source'`,
	).Scan(&hasSource); err != nil {
		t.Fatal(err)
	}
	if hasSource != 1 {
		t.Errorf("song_meta.bms_search_source should exist, got %d", hasSource)
	}

	// bmssearch_bms_id_md5 テーブルが存在する
	var c int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bmssearch_bms_id_md5`).Scan(&c); err != nil {
		t.Errorf("bmssearch_bms_id_md5 table not found: %v", err)
	}

	// bmssearch_bms テーブルが存在する
	if err := db.QueryRow(`SELECT COUNT(*) FROM bmssearch_bms`).Scan(&c); err != nil {
		t.Errorf("bmssearch_bms table not found: %v", err)
	}
}

func TestRunMigrations_BMSSearchSourceBackfill(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 旧スキーマ相当: 先にマイグレーションを実行してから bms_search_source カラムを削除
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	// シミュレーション: bms_search_source を一旦消して bms_search_id だけ入れる
	if _, err := db.Exec(`UPDATE song_meta SET bms_search_source = NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO song_meta (folder_hash, bms_search_id) VALUES ('h1', 'bms-1')`); err != nil {
		t.Fatal(err)
	}

	// 再度マイグレーション → backfill ロジックで 'official' が入る
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	var src sql.NullString
	if err := db.QueryRow(`SELECT bms_search_source FROM song_meta WHERE folder_hash='h1'`).Scan(&src); err != nil {
		t.Fatal(err)
	}
	if !src.Valid || src.String != "official" {
		t.Errorf("expected bms_search_source='official', got %v", src)
	}
}

func TestRunMigrations_WavMinhashV2Clear(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 初回マイグレーションでテーブル一式を作成
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// v2 マイグレーション以前の状態をシミュレート: user_version=0 に戻し、
	// chart_meta に旧形式 wav_minhash を 1 件入れる
	if _, err := db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	legacy := make([]byte, 256)
	for i := range legacy {
		legacy[i] = byte(i)
	}
	if _, err := db.Exec(
		`INSERT INTO chart_meta (md5, wav_minhash) VALUES ('aabbcc', ?)`,
		legacy,
	); err != nil {
		t.Fatal(err)
	}

	// 再度マイグレーションを走らせると v2 移行が適用される
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// wav_minhash が NULL になっていること
	var stored sql.NullString
	if err := db.QueryRow(
		`SELECT wav_minhash FROM chart_meta WHERE md5 = 'aabbcc'`,
	).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Errorf("wav_minhash should be NULL after v2 migration, got %v", stored)
	}

	// user_version が 2 になっていること
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatal(err)
	}
	if uv != 2 {
		t.Errorf("user_version should be 2, got %d", uv)
	}
}

func TestRunMigrations_WavMinhashV2Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// v2 マイグレーション後の状態で wav_minhash 入りレコードを追加
	freshSig := make([]byte, 256)
	for i := range freshSig {
		freshSig[i] = byte(255 - i)
	}
	if _, err := db.Exec(
		`INSERT INTO chart_meta (md5, wav_minhash) VALUES ('ddeeff', ?)`,
		freshSig,
	); err != nil {
		t.Fatal(err)
	}

	// 2 回目のマイグレーション: user_version はすでに 2 なのでクリアされてはいけない
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	var stored []byte
	if err := db.QueryRow(
		`SELECT wav_minhash FROM chart_meta WHERE md5 = 'ddeeff'`,
	).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 256 {
		t.Fatalf("wav_minhash should be preserved (256 bytes), got %d", len(stored))
	}
	for i, b := range stored {
		if b != byte(255-i) {
			t.Fatalf("byte %d: expected %d, got %d", i, byte(255-i), b)
		}
	}
}
