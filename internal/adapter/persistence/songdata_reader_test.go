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
	dtRepo := persistence.NewDifficultyTableRepository(db)
	reader := persistence.NewSongdataReader(db, metaRepo, dtRepo)

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

func TestListAllCharts(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	charts, err := reader.ListAllCharts(ctx)
	if err != nil {
		t.Fatalf("ListAllCharts failed: %v", err)
	}

	// testdata/songdata.db に譜面が存在すること
	if len(charts) == 0 {
		t.Fatal("expected charts, got 0")
	}

	// 各譜面にMD5があること
	for i, c := range charts {
		if c.MD5 == "" {
			t.Errorf("charts[%d].MD5 is empty", i)
		}
	}

	// タイトルが非空の譜面が存在すること
	hasTitle := false
	for _, c := range charts {
		if c.Title != "" {
			hasTitle = true
			break
		}
	}
	if !hasTitle {
		t.Error("expected at least one chart with non-empty Title")
	}
}

func TestListSongGroupsForDuplicateScan(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	groups, err := reader.ListSongGroupsForDuplicateScan(ctx)
	if err != nil {
		t.Fatalf("ListSongGroupsForDuplicateScan failed: %v", err)
	}

	if len(groups) == 0 {
		t.Fatal("expected at least one song group")
	}

	// FolderHashは全グループで非空であること
	for i, g := range groups {
		if g.FolderHash == "" {
			t.Errorf("groups[%d].FolderHash is empty", i)
		}
	}

	// タイトルが非空のグループが存在すること
	hasTitle := false
	for _, g := range groups {
		if g.Title != "" {
			hasTitle = true
			break
		}
	}
	if !hasTitle {
		t.Error("expected at least one group with non-empty Title")
	}
}

func TestListChartsWithoutMinhash(t *testing.T) {
	_, db := setupSongdataReader(t)
	repo := persistence.NewElsaRepository(db)

	targets, err := repo.ListChartsWithoutMinhash(context.Background())
	if err != nil {
		t.Fatalf("ListChartsWithoutMinhash failed: %v", err)
	}

	// songdata.dbには譜面が存在するので、chart_metaが空の状態では全譜面が対象
	if len(targets) == 0 {
		t.Fatal("expected non-empty targets, got 0")
	}

	// 各ターゲットにMD5とPathが設定されていることを確認
	for _, tgt := range targets {
		if tgt.MD5 == "" {
			t.Error("target has empty MD5")
		}
		if tgt.Path == "" {
			t.Error("target has empty Path")
		}
	}
}

func TestUpdateWavMinhash(t *testing.T) {
	_, db := setupSongdataReader(t)
	repo := persistence.NewElsaRepository(db)

	// テスト用のMinHashデータ（256バイト）
	minhash := make([]byte, 256)
	for i := range minhash {
		minhash[i] = byte(i)
	}

	// songdata.dbから実在するMD5を1つ取得
	targets, err := repo.ListChartsWithoutMinhash(context.Background())
	if err != nil || len(targets) == 0 {
		t.Fatal("ListChartsWithoutMinhash failed or empty")
	}
	md5 := targets[0].MD5

	// UpdateWavMinhashを実行
	if err := repo.UpdateWavMinhash(context.Background(), md5, minhash); err != nil {
		t.Fatalf("UpdateWavMinhash failed: %v", err)
	}

	// wav_minhashが保存されたことを確認
	var stored []byte
	err = db.QueryRow(`SELECT wav_minhash FROM chart_meta WHERE md5 = ?`, md5).Scan(&stored)
	if err != nil {
		t.Fatalf("failed to read wav_minhash: %v", err)
	}
	if len(stored) != 256 {
		t.Fatalf("expected 256 bytes, got %d", len(stored))
	}
	for i, b := range stored {
		if b != byte(i) {
			t.Fatalf("byte %d: expected %d, got %d", i, byte(i), b)
		}
	}

	// 更新後はListChartsWithoutMinhashから除外されることを確認
	targets2, err := repo.ListChartsWithoutMinhash(context.Background())
	if err != nil {
		t.Fatalf("ListChartsWithoutMinhash after update failed: %v", err)
	}
	for _, tgt := range targets2 {
		if tgt.MD5 == md5 {
			t.Errorf("md5 %s should not appear after minhash update", md5)
		}
	}
}

func TestFindFolderInfoByMD5_Found(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	// songdata.db の先頭1件から実在 md5 を取得
	opts := model.ListOptions{Page: 1, PageSize: 1}
	songs, _, err := reader.ListSongs(ctx, opts)
	if err != nil || len(songs) == 0 {
		t.Fatal("前提: ListSongs で楽曲が取得できること")
	}
	song, err := reader.GetSongByFolder(ctx, songs[0].FolderHash)
	if err != nil || song == nil || len(song.Charts) == 0 {
		t.Fatal("前提: GetSongByFolder で譜面が取得できること")
	}
	md5 := song.Charts[0].MD5

	folder, md5s, title, artist, found, err := reader.FindFolderInfoByMD5(ctx, md5)
	if err != nil {
		t.Fatalf("FindFolderInfoByMD5 failed: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if folder == "" {
		t.Error("folder is empty")
	}
	if len(md5s) == 0 {
		t.Error("md5s is empty")
	}
	if title == "" {
		t.Error("title is empty")
	}
	// artist は空文字の楽曲もあり得るため必須チェックしない
	_ = artist
}

func TestFindFolderInfoByMD5_NotFound(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	folder, md5s, title, artist, found, err := reader.FindFolderInfoByMD5(ctx, "nonexistent_md5_xxxxx")
	if err != nil {
		t.Fatalf("FindFolderInfoByMD5 error: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
	if folder != "" || len(md5s) != 0 || title != "" || artist != "" {
		t.Error("戻り値はすべてゼロ値であること")
	}
}

func TestFindOrphanInfoByMD5_Found(t *testing.T) {
	reader, db := setupSongdataReader(t)
	ctx := context.Background()

	// difficulty_table と difficulty_table_entry にデータを挿入
	var tableID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO difficulty_table(url, header_url, data_url, name, symbol)
		VALUES ('http://t.example.com','http://t.example.com/h','http://t.example.com/b','TestTable','T')
		RETURNING id`,
	).Scan(&tableID)
	if err != nil {
		t.Fatalf("difficulty_table INSERT failed: %v", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO difficulty_table_entry(table_id, md5, level, title, artist)
		VALUES (?, 'orphan_md5_001', '9', 'Orphan Song', 'Orphan Artist')`, tableID)
	if err != nil {
		t.Fatalf("difficulty_table_entry INSERT failed: %v", err)
	}

	title, artist, found, err := reader.FindOrphanInfoByMD5(ctx, "orphan_md5_001")
	if err != nil {
		t.Fatalf("FindOrphanInfoByMD5 failed: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if title != "Orphan Song" {
		t.Errorf("title = %q, want %q", title, "Orphan Song")
	}
	if artist != "Orphan Artist" {
		t.Errorf("artist = %q, want %q", artist, "Orphan Artist")
	}
}

func TestFindOrphanInfoByMD5_NotFound(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	title, artist, found, err := reader.FindOrphanInfoByMD5(ctx, "nonexistent_md5_yyy")
	if err != nil {
		t.Fatalf("FindOrphanInfoByMD5 error: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
	if title != "" || artist != "" {
		t.Error("戻り値はすべてゼロ値であること")
	}
}

func TestFindOrphanInfoByMD5_EmptyTitle(t *testing.T) {
	reader, db := setupSongdataReader(t)
	ctx := context.Background()

	// title が NULL のエントリは found=false として扱う
	var tableID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO difficulty_table(url, header_url, data_url, name, symbol)
		VALUES ('http://t2.example.com','http://t2.example.com/h','http://t2.example.com/b','TestTable2','T2')
		RETURNING id`,
	).Scan(&tableID)
	if err != nil {
		t.Fatalf("difficulty_table INSERT failed: %v", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO difficulty_table_entry(table_id, md5, level, title, artist)
		VALUES (?, 'empty_title_md5_002', '9', NULL, 'Some Artist')`, tableID)
	if err != nil {
		t.Fatalf("difficulty_table_entry INSERT failed: %v", err)
	}

	_, _, found, err := reader.FindOrphanInfoByMD5(ctx, "empty_title_md5_002")
	if err != nil {
		t.Fatalf("FindOrphanInfoByMD5 error: %v", err)
	}
	if found {
		t.Error("title が NULL の場合は found=false であること")
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
