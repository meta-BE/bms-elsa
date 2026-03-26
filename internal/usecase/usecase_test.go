package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// --- モック実装 ---

type mockSongRepo struct {
	listSongsFunc       func(ctx context.Context, opts model.ListOptions) ([]model.Song, int, error)
	getSongByFolderFunc func(ctx context.Context, folderHash string) (*model.Song, error)
}

func (m *mockSongRepo) ListSongs(ctx context.Context, opts model.ListOptions) ([]model.Song, int, error) {
	return m.listSongsFunc(ctx, opts)
}

func (m *mockSongRepo) ListAllSongs(ctx context.Context) ([]model.Song, error) {
	songs, _, err := m.listSongsFunc(ctx, model.ListOptions{})
	return songs, err
}

func (m *mockSongRepo) GetSongByFolder(ctx context.Context, folderHash string) (*model.Song, error) {
	return m.getSongByFolderFunc(ctx, folderHash)
}

func (m *mockSongRepo) FindChartFoldersByTitle(_ context.Context, _ string) ([]model.InstallCandidate, error) {
	return nil, nil
}

func (m *mockSongRepo) FindChartFoldersByBodyURL(_ context.Context, _ string) ([]model.InstallCandidate, error) {
	return nil, nil
}

func (m *mockSongRepo) FindChartFoldersByArtist(_ context.Context, _ string) ([]model.InstallCandidate, error) {
	return nil, nil
}

func (m *mockSongRepo) ListSongGroupsForDuplicateScan(_ context.Context) ([]model.SongGroup, error) {
	return nil, nil
}

type mockMetaRepo struct {
	getSongMetaFunc         func(ctx context.Context, folderHash string) (*model.SongMeta, error)
	upsertSongMetaFunc      func(ctx context.Context, meta model.SongMeta) error
	getChartMetaFunc        func(ctx context.Context, md5 string) (*model.ChartIRMeta, error)
	upsertChartMetaFunc     func(ctx context.Context, meta model.ChartIRMeta) error
	bulkUpsertChartMetaFunc func(ctx context.Context, metas []model.ChartIRMeta) error
	updateWorkingURLsFunc   func(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
}

func (m *mockMetaRepo) GetSongMeta(ctx context.Context, folderHash string) (*model.SongMeta, error) {
	return m.getSongMetaFunc(ctx, folderHash)
}

func (m *mockMetaRepo) UpsertSongMeta(ctx context.Context, meta model.SongMeta) error {
	return m.upsertSongMetaFunc(ctx, meta)
}

func (m *mockMetaRepo) GetChartMeta(ctx context.Context, md5 string) (*model.ChartIRMeta, error) {
	return m.getChartMetaFunc(ctx, md5)
}

func (m *mockMetaRepo) UpsertChartMeta(ctx context.Context, meta model.ChartIRMeta) error {
	return m.upsertChartMetaFunc(ctx, meta)
}

func (m *mockMetaRepo) BulkUpsertChartMeta(ctx context.Context, metas []model.ChartIRMeta) error {
	return m.bulkUpsertChartMetaFunc(ctx, metas)
}

func (m *mockMetaRepo) UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error {
	return m.updateWorkingURLsFunc(ctx, md5, workingBodyURL, workingDiffURL)
}

func (m *mockMetaRepo) ListEvents(_ context.Context) ([]model.Event, error) {
	return nil, nil
}

func (m *mockMetaRepo) GetEventByBMSSearchID(_ context.Context, _ string) (*model.Event, error) {
	return nil, nil
}

func (m *mockMetaRepo) UpsertEventByBMSSearchID(_ context.Context, _ model.Event) error {
	return nil
}

func (m *mockMetaRepo) UpdateEventShortName(_ context.Context, _ int, _ string) error {
	return nil
}

func (m *mockMetaRepo) ListFoldersWithoutEvent(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockMetaRepo) UpdateSongMetaEvent(_ context.Context, _ string, _ int, _ string) error {
	return nil
}

func (m *mockMetaRepo) ListUnfetchedChartMD5s(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockMetaRepo) ListUnfetchedDTEntryMD5s(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}

func (m *mockMetaRepo) ListRewriteRules(_ context.Context) ([]model.RewriteRule, error) {
	return nil, nil
}

func (m *mockMetaRepo) UpsertRewriteRule(_ context.Context, _ model.RewriteRule) error {
	return nil
}

func (m *mockMetaRepo) DeleteRewriteRule(_ context.Context, _ int) error {
	return nil
}

func (m *mockMetaRepo) ListChartsForWorkingURLInference(_ context.Context) ([]model.ChartIRMeta, error) {
	return nil, nil
}

func (m *mockMetaRepo) FindMostSimilarByMinHash(_ context.Context, _ []byte, _ float64) (*model.MinHashMatch, error) {
	return nil, nil
}

func (m *mockMetaRepo) ListChartsWithoutMinhash(_ context.Context) ([]model.ChartScanTarget, error) {
	return nil, nil
}

func (m *mockMetaRepo) UpdateWavMinhash(_ context.Context, _ string, _ []byte) error {
	return nil
}

type mockIRClient struct {
	lookupFunc func(ctx context.Context, md5 string) (*port.IRResponse, error)
}

func (m *mockIRClient) LookupByMD5(ctx context.Context, md5 string) (*port.IRResponse, error) {
	return m.lookupFunc(ctx, md5)
}

// --- テストケース ---

func TestListSongs(t *testing.T) {
	expectedSongs := []model.Song{
		{FolderHash: "abc", Title: "Test Song"},
	}
	expectedTotal := 1

	var calledOpts model.ListOptions
	repo := &mockSongRepo{
		listSongsFunc: func(_ context.Context, opts model.ListOptions) ([]model.Song, int, error) {
			calledOpts = opts
			return expectedSongs, expectedTotal, nil
		},
	}

	uc := usecase.NewListSongsUseCase(repo)
	opts := model.ListOptions{Page: 1, PageSize: 20, Search: "test"}
	songs, total, err := uc.Execute(context.Background(), opts)

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if total != expectedTotal {
		t.Errorf("total = %d, want %d", total, expectedTotal)
	}
	if len(songs) != 1 || songs[0].FolderHash != "abc" {
		t.Errorf("songs = %v, want %v", songs, expectedSongs)
	}
	if calledOpts != opts {
		t.Errorf("リポジトリに渡されたopts = %+v, want %+v", calledOpts, opts)
	}
}

func TestGetSongDetail(t *testing.T) {
	expectedSong := &model.Song{FolderHash: "abc123", Title: "Detail Song"}
	var calledHash string
	repo := &mockSongRepo{
		getSongByFolderFunc: func(_ context.Context, folderHash string) (*model.Song, error) {
			calledHash = folderHash
			return expectedSong, nil
		},
	}

	uc := usecase.NewGetSongDetailUseCase(repo)
	song, err := uc.Execute(context.Background(), "abc123")

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if song.FolderHash != "abc123" {
		t.Errorf("song.FolderHash = %q, want %q", song.FolderHash, "abc123")
	}
	if calledHash != "abc123" {
		t.Errorf("リポジトリに渡されたfolderHash = %q, want %q", calledHash, "abc123")
	}
}

func TestUpdateSongMeta(t *testing.T) {
	var calledMeta model.SongMeta
	repo := &mockMetaRepo{
		upsertSongMetaFunc: func(_ context.Context, meta model.SongMeta) error {
			calledMeta = meta
			return nil
		},
	}

	year := 2020
	inputMeta := model.SongMeta{FolderHash: "folder1", ReleaseYear: &year}
	uc := usecase.NewUpdateSongMetaUseCase(repo)
	err := uc.Execute(context.Background(), inputMeta)

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if calledMeta.FolderHash != "folder1" {
		t.Errorf("calledMeta.FolderHash = %q, want %q", calledMeta.FolderHash, "folder1")
	}
	if *calledMeta.ReleaseYear != 2020 {
		t.Errorf("calledMeta.ReleaseYear = %d, want %d", *calledMeta.ReleaseYear, 2020)
	}
}

func TestUpdateChartMeta(t *testing.T) {
	var calledMD5, calledBodyURL, calledDiffURL string
	repo := &mockMetaRepo{
		updateWorkingURLsFunc: func(_ context.Context, md5, workingBodyURL, workingDiffURL string) error {
			calledMD5 = md5
			calledBodyURL = workingBodyURL
			calledDiffURL = workingDiffURL
			return nil
		},
	}

	uc := usecase.NewUpdateChartMetaUseCase(repo)
	err := uc.Execute(context.Background(), "md5hash", "http://body.url", "http://diff.url")

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if calledMD5 != "md5hash" {
		t.Errorf("calledMD5 = %q, want %q", calledMD5, "md5hash")
	}
	if calledBodyURL != "http://body.url" {
		t.Errorf("calledBodyURL = %q, want %q", calledBodyURL, "http://body.url")
	}
	if calledDiffURL != "http://diff.url" {
		t.Errorf("calledDiffURL = %q, want %q", calledDiffURL, "http://diff.url")
	}
}

func TestLookupIR_Registered(t *testing.T) {
	irResp := &port.IRResponse{
		Registered: true,
		Tags:       []string{"NORMAL", "7KEYS"},
		BodyURL:    "http://example.com/body",
		DiffURL:    "http://example.com/diff",
		Notes:      "1000",
	}

	irClient := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			if md5 != "testmd5" {
				t.Errorf("IRClientに渡されたmd5 = %q, want %q", md5, "testmd5")
			}
			return irResp, nil
		},
	}

	var upsertedMeta model.ChartIRMeta
	upsertCalled := false
	metaRepo := &mockMetaRepo{
		upsertChartMetaFunc: func(_ context.Context, meta model.ChartIRMeta) error {
			upsertCalled = true
			upsertedMeta = meta
			return nil
		},
	}

	uc := usecase.NewLookupIRUseCase(irClient, metaRepo)
	resp, err := uc.Execute(context.Background(), "testmd5", "testsha256")

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !resp.Registered {
		t.Error("resp.Registered = false, want true")
	}
	if !upsertCalled {
		t.Fatal("Registered=trueの場合、UpsertChartMetaが呼ばれるべき")
	}
	if upsertedMeta.MD5 != "testmd5" {
		t.Errorf("upsertedMeta.MD5 = %q, want %q", upsertedMeta.MD5, "testmd5")
	}
	if upsertedMeta.SHA256 != "testsha256" {
		t.Errorf("upsertedMeta.SHA256 = %q, want %q", upsertedMeta.SHA256, "testsha256")
	}
	if upsertedMeta.LR2IRBodyURL != "http://example.com/body" {
		t.Errorf("upsertedMeta.LR2IRBodyURL = %q, want %q", upsertedMeta.LR2IRBodyURL, "http://example.com/body")
	}
	if upsertedMeta.LR2IRDiffURL != "http://example.com/diff" {
		t.Errorf("upsertedMeta.LR2IRDiffURL = %q, want %q", upsertedMeta.LR2IRDiffURL, "http://example.com/diff")
	}
	if upsertedMeta.LR2IRNotes != "1000" {
		t.Errorf("upsertedMeta.LR2IRNotes = %q, want %q", upsertedMeta.LR2IRNotes, "1000")
	}
	if upsertedMeta.FetchedAt == nil {
		t.Error("upsertedMeta.FetchedAt = nil, want non-nil")
	}
}

func TestLookupIR_NotRegistered_StillSavesFetchedAt(t *testing.T) {
	irClient := &mockIRClient{
		lookupFunc: func(_ context.Context, _ string) (*port.IRResponse, error) {
			return &port.IRResponse{Registered: false}, nil
		},
	}

	var upsertedMeta model.ChartIRMeta
	upsertCalled := false
	metaRepo := &mockMetaRepo{
		upsertChartMetaFunc: func(_ context.Context, meta model.ChartIRMeta) error {
			upsertCalled = true
			upsertedMeta = meta
			return nil
		},
	}

	uc := usecase.NewLookupIRUseCase(irClient, metaRepo)
	resp, err := uc.Execute(context.Background(), "md5notfound", "sha256notfound")

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if resp.Registered {
		t.Error("resp.Registered = true, want false")
	}
	// 未登録でもfetched_atを保存する
	if !upsertCalled {
		t.Fatal("未登録でもUpsertChartMetaが呼ばれるべき")
	}
	if upsertedMeta.FetchedAt == nil {
		t.Error("FetchedAt should be set even for unregistered")
	}
	if upsertedMeta.MD5 != "md5notfound" {
		t.Errorf("MD5 = %q, want %q", upsertedMeta.MD5, "md5notfound")
	}
	if upsertedMeta.SHA256 != "sha256notfound" {
		t.Errorf("SHA256 = %q, want %q", upsertedMeta.SHA256, "sha256notfound")
	}
	// 未登録なのでURLは空
	if upsertedMeta.LR2IRBodyURL != "" {
		t.Errorf("BodyURL should be empty, got %q", upsertedMeta.LR2IRBodyURL)
	}
	if upsertedMeta.LR2IRDiffURL != "" {
		t.Errorf("DiffURL should be empty, got %q", upsertedMeta.LR2IRDiffURL)
	}
}
