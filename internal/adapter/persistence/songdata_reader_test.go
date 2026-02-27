package persistence_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// projectRoot はテストファイルからプロジェクトルートへのパスを返す
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/adapter/persistence/ → プロジェクトルートは3階層上
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func setupSongdataReader(t *testing.T) (*persistence.SongdataReader, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// ATTACHはコネクション単位なのでプールサイズを1に制限
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// elsa.dbスキーマを作成
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// songdata.dbをATTACH
	songdataPath := filepath.Join(projectRoot(t), "testdata", "songdata.db")
	if err := persistence.AttachSongdata(db, songdataPath); err != nil {
		t.Fatalf("AttachSongdata failed: %v", err)
	}

	metaRepo := persistence.NewElsaRepository(db)
	reader := persistence.NewSongdataReader(db, metaRepo)

	return reader, db
}

func TestListSongs_Default(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	opts := model.ListOptions{
		Page:     1,
		PageSize: 50,
	}

	songs, totalCount, err := reader.ListSongs(ctx, opts)
	if err != nil {
		t.Fatalf("ListSongs failed: %v", err)
	}

	if len(songs) != 50 {
		t.Errorf("len(songs) = %d, want 50", len(songs))
	}

	// songdata.dbには2666ユニークfolder がある
	if totalCount < 1000 {
		t.Errorf("totalCount = %d, want > 1000", totalCount)
	}

	// 各Songの必須フィールドが埋まっていること
	for i, s := range songs {
		if s.FolderHash == "" {
			t.Errorf("songs[%d].FolderHash is empty", i)
		}
		if s.Title == "" {
			t.Errorf("songs[%d].Title is empty", i)
		}
	}

	// リスト表示ではChartsは空
	for i, s := range songs {
		if len(s.Charts) != 0 {
			t.Errorf("songs[%d].Charts should be empty in list view, got %d", i, len(s.Charts))
		}
	}
}

func TestListSongs_Paging(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	opts1 := model.ListOptions{Page: 1, PageSize: 10}
	songs1, total1, err := reader.ListSongs(ctx, opts1)
	if err != nil {
		t.Fatalf("ListSongs page1 failed: %v", err)
	}

	opts2 := model.ListOptions{Page: 2, PageSize: 10}
	songs2, total2, err := reader.ListSongs(ctx, opts2)
	if err != nil {
		t.Fatalf("ListSongs page2 failed: %v", err)
	}

	if len(songs1) != 10 {
		t.Errorf("page1 len = %d, want 10", len(songs1))
	}
	if len(songs2) != 10 {
		t.Errorf("page2 len = %d, want 10", len(songs2))
	}

	// totalCountは同じ
	if total1 != total2 {
		t.Errorf("totalCount mismatch: page1=%d, page2=%d", total1, total2)
	}

	// ページ1とページ2の結果が異なること
	if songs1[0].FolderHash == songs2[0].FolderHash {
		t.Error("page 1 and page 2 should return different results")
	}
}

func TestListSongs_Search(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	// songdata.dbに存在する楽曲タイトルで検索
	opts := model.ListOptions{
		Page:     1,
		PageSize: 50,
		Search:   "Love & Justice",
	}

	songs, totalCount, err := reader.ListSongs(ctx, opts)
	if err != nil {
		t.Fatalf("ListSongs with search failed: %v", err)
	}

	if totalCount == 0 {
		t.Fatal("search should return at least 1 result")
	}

	if len(songs) == 0 {
		t.Fatal("search should return at least 1 song")
	}

	// 検索結果に期待する楽曲が含まれていること
	found := false
	for _, s := range songs {
		if s.Title == "Love & Justice [EXTREME]" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'Love & Justice [EXTREME]' in search results, got titles: %v", songTitles(songs))
	}
}

func TestGetSongByFolder(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	// まずListSongsで結果を取得
	opts := model.ListOptions{Page: 1, PageSize: 1}
	songs, _, err := reader.ListSongs(ctx, opts)
	if err != nil {
		t.Fatalf("ListSongs failed: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("ListSongs returned 0 songs")
	}

	folderHash := songs[0].FolderHash

	song, err := reader.GetSongByFolder(ctx, folderHash)
	if err != nil {
		t.Fatalf("GetSongByFolder failed: %v", err)
	}
	if song == nil {
		t.Fatal("GetSongByFolder returned nil")
	}

	if song.FolderHash != folderHash {
		t.Errorf("FolderHash = %q, want %q", song.FolderHash, folderHash)
	}

	// 詳細取得ではChartsが非空
	if len(song.Charts) == 0 {
		t.Error("GetSongByFolder should return non-empty Charts")
	}

	// 各Chartの必須フィールドが埋まっていること
	for i, c := range song.Charts {
		if c.MD5 == "" {
			t.Errorf("Charts[%d].MD5 is empty", i)
		}
		if c.SHA256 == "" {
			t.Errorf("Charts[%d].SHA256 is empty", i)
		}
	}
}

func TestGetSongByFolder_NotFound(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	song, err := reader.GetSongByFolder(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSongByFolder should not error for missing folder: %v", err)
	}
	if song != nil {
		t.Errorf("GetSongByFolder should return nil for missing folder, got %+v", song)
	}
}

func TestListSongs_SortDesc(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	opts := model.ListOptions{
		Page:     1,
		PageSize: 50,
		SortBy:   "title",
		SortDesc: true,
	}

	songs, _, err := reader.ListSongs(ctx, opts)
	if err != nil {
		t.Fatalf("ListSongs sort desc failed: %v", err)
	}

	if len(songs) < 2 {
		t.Fatal("need at least 2 songs to test sorting")
	}

	// 降順なので先頭 > 末尾
	if songs[0].Title < songs[len(songs)-1].Title {
		t.Errorf("sort desc: first title %q should be > last title %q",
			songs[0].Title, songs[len(songs)-1].Title)
	}
}

func TestListSongs_DefaultPageSize(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	// PageSize=0 の場合デフォルト50が適用される
	opts := model.ListOptions{Page: 1, PageSize: 0}
	songs, _, err := reader.ListSongs(ctx, opts)
	if err != nil {
		t.Fatalf("ListSongs with default pageSize failed: %v", err)
	}
	if len(songs) != 50 {
		t.Errorf("default pageSize: len(songs) = %d, want 50", len(songs))
	}
}

// songTitles はデバッグ用にSongのタイトル一覧を返す
func songTitles(songs []model.Song) []string {
	titles := make([]string, len(songs))
	for i, s := range songs {
		titles[i] = s.Title
	}
	return titles
}
