package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

func TestRunAutoInference_AllMatch(t *testing.T) {
	mappings := []model.EventMapping{
		{ID: 1, URLPattern: "bof2020", EventName: "BOF2020", ReleaseYear: 2020},
		{ID: 2, URLPattern: "bof2021", EventName: "BOF2021", ReleaseYear: 2021},
	}

	songs := []model.SongIRURLs{
		{FolderHash: "hash1", Title: "Song1", BodyURLs: []string{"http://example.com/bof2020/entry1"}, ChartCount: 2, IRCount: 2},
		{FolderHash: "hash2", Title: "Song2", BodyURLs: []string{"http://example.com/bof2021/entry2"}, ChartCount: 1, IRCount: 1},
	}

	// UpsertSongMetaの呼び出しを記録
	var upserted []model.SongMeta
	repo := &mockMetaRepo{
		listEventMappingsFunc: func(_ context.Context) ([]model.EventMapping, error) {
			return mappings, nil
		},
		listUnsetSongsWithIRURLsFunc: func(_ context.Context) ([]model.SongIRURLs, error) {
			return songs, nil
		},
		upsertSongMetaFunc: func(_ context.Context, meta model.SongMeta) error {
			upserted = append(upserted, meta)
			return nil
		},
	}

	uc := usecase.NewInferSongMetaUseCase(repo)
	result, err := uc.RunAutoInference(context.Background())

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// 全曲マッチするのでAutoSetCount=2
	if result.AutoSetCount != 2 {
		t.Errorf("AutoSetCount = %d, want %d", result.AutoSetCount, 2)
	}

	// 未マッチなし
	if len(result.UnmatchedSongs) != 0 {
		t.Errorf("UnmatchedSongs = %d件, want 0件", len(result.UnmatchedSongs))
	}

	if result.NoIRCount != 0 {
		t.Errorf("NoIRCount = %d, want %d", result.NoIRCount, 0)
	}

	// UpsertSongMetaが正しい引数で呼ばれたか確認
	if len(upserted) != 2 {
		t.Fatalf("UpsertSongMeta呼び出し回数 = %d, want %d", len(upserted), 2)
	}

	// hash1 → BOF2020, 2020
	if upserted[0].FolderHash != "hash1" {
		t.Errorf("upserted[0].FolderHash = %q, want %q", upserted[0].FolderHash, "hash1")
	}
	if *upserted[0].ReleaseYear != 2020 {
		t.Errorf("upserted[0].ReleaseYear = %d, want %d", *upserted[0].ReleaseYear, 2020)
	}
	if *upserted[0].EventName != "BOF2020" {
		t.Errorf("upserted[0].EventName = %q, want %q", *upserted[0].EventName, "BOF2020")
	}

	// hash2 → BOF2021, 2021
	if upserted[1].FolderHash != "hash2" {
		t.Errorf("upserted[1].FolderHash = %q, want %q", upserted[1].FolderHash, "hash2")
	}
	if *upserted[1].ReleaseYear != 2021 {
		t.Errorf("upserted[1].ReleaseYear = %d, want %d", *upserted[1].ReleaseYear, 2021)
	}
	if *upserted[1].EventName != "BOF2021" {
		t.Errorf("upserted[1].EventName = %q, want %q", *upserted[1].EventName, "BOF2021")
	}
}

func TestRunAutoInference_PartialMatch(t *testing.T) {
	mappings := []model.EventMapping{
		{ID: 1, URLPattern: "bof2020", EventName: "BOF2020", ReleaseYear: 2020},
	}

	songs := []model.SongIRURLs{
		{FolderHash: "hash1", Title: "Song1", BodyURLs: []string{"http://example.com/bof2020/entry1"}, ChartCount: 2, IRCount: 2},
		{FolderHash: "hash2", Title: "Song2", BodyURLs: []string{"http://example.com/unknown/entry2"}, ChartCount: 1, IRCount: 1},
		{FolderHash: "hash3", Title: "Song3", BodyURLs: []string{"http://example.com/other/entry3"}, ChartCount: 3, IRCount: 3},
	}

	var upserted []model.SongMeta
	repo := &mockMetaRepo{
		listEventMappingsFunc: func(_ context.Context) ([]model.EventMapping, error) {
			return mappings, nil
		},
		listUnsetSongsWithIRURLsFunc: func(_ context.Context) ([]model.SongIRURLs, error) {
			return songs, nil
		},
		upsertSongMetaFunc: func(_ context.Context, meta model.SongMeta) error {
			upserted = append(upserted, meta)
			return nil
		},
	}

	uc := usecase.NewInferSongMetaUseCase(repo)
	result, err := uc.RunAutoInference(context.Background())

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// hash1のみマッチ
	if result.AutoSetCount != 1 {
		t.Errorf("AutoSetCount = %d, want %d", result.AutoSetCount, 1)
	}

	// hash2, hash3が未マッチ
	if len(result.UnmatchedSongs) != 2 {
		t.Fatalf("UnmatchedSongs = %d件, want 2件", len(result.UnmatchedSongs))
	}
	if result.UnmatchedSongs[0].FolderHash != "hash2" {
		t.Errorf("UnmatchedSongs[0].FolderHash = %q, want %q", result.UnmatchedSongs[0].FolderHash, "hash2")
	}
	if result.UnmatchedSongs[1].FolderHash != "hash3" {
		t.Errorf("UnmatchedSongs[1].FolderHash = %q, want %q", result.UnmatchedSongs[1].FolderHash, "hash3")
	}

	// UpsertSongMetaはマッチ分のみ呼ばれる
	if len(upserted) != 1 {
		t.Fatalf("UpsertSongMeta呼び出し回数 = %d, want %d", len(upserted), 1)
	}
	if upserted[0].FolderHash != "hash1" {
		t.Errorf("upserted[0].FolderHash = %q, want %q", upserted[0].FolderHash, "hash1")
	}
	if *upserted[0].ReleaseYear != 2020 {
		t.Errorf("upserted[0].ReleaseYear = %d, want %d", *upserted[0].ReleaseYear, 2020)
	}
	if *upserted[0].EventName != "BOF2020" {
		t.Errorf("upserted[0].EventName = %q, want %q", *upserted[0].EventName, "BOF2020")
	}

	// NoIRCount: 全てIR取得済みなので0
	if result.NoIRCount != 0 {
		t.Errorf("NoIRCount = %d, want %d", result.NoIRCount, 0)
	}
}

func TestRunAutoInference_MultiPartPattern(t *testing.T) {
	mappings := []model.EventMapping{
		{ID: 1, URLPattern: "manbow.nothing.sh|&event=104", EventName: "BOFU2015", ReleaseYear: 2015},
	}

	songs := []model.SongIRURLs{
		// ドメイン+event両方マッチ
		{FolderHash: "hash1", Title: "Song1", BodyURLs: []string{"http://manbow.nothing.sh/event/event.cgi?action=More_def&num=78&event=104"}, ChartCount: 1, IRCount: 1},
		// eventだけマッチ（別ドメイン）→ マッチしない
		{FolderHash: "hash2", Title: "Song2", BodyURLs: []string{"http://other.example.com/page?&event=104"}, ChartCount: 1, IRCount: 1},
	}

	var upserted []model.SongMeta
	repo := &mockMetaRepo{
		listEventMappingsFunc: func(_ context.Context) ([]model.EventMapping, error) {
			return mappings, nil
		},
		listUnsetSongsWithIRURLsFunc: func(_ context.Context) ([]model.SongIRURLs, error) {
			return songs, nil
		},
		upsertSongMetaFunc: func(_ context.Context, meta model.SongMeta) error {
			upserted = append(upserted, meta)
			return nil
		},
	}

	uc := usecase.NewInferSongMetaUseCase(repo)
	result, err := uc.RunAutoInference(context.Background())

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if result.AutoSetCount != 1 {
		t.Errorf("AutoSetCount = %d, want %d", result.AutoSetCount, 1)
	}
	if len(result.UnmatchedSongs) != 1 {
		t.Fatalf("UnmatchedSongs = %d件, want 1件", len(result.UnmatchedSongs))
	}
	if result.UnmatchedSongs[0].FolderHash != "hash2" {
		t.Errorf("UnmatchedSongs[0].FolderHash = %q, want %q", result.UnmatchedSongs[0].FolderHash, "hash2")
	}
	if len(upserted) != 1 || upserted[0].FolderHash != "hash1" {
		t.Errorf("upserted = %v, want hash1のみ", upserted)
	}
}

func TestRunAutoInference_NoIRURLs(t *testing.T) {
	mappings := []model.EventMapping{
		{ID: 1, URLPattern: "bof2020", EventName: "BOF2020", ReleaseYear: 2020},
	}

	songs := []model.SongIRURLs{
		// IR未取得: BodyURLsが空、IRCount=0
		{FolderHash: "hash1", Title: "Song1", BodyURLs: []string{}, ChartCount: 3, IRCount: 0},
		// IR取得済みだがマッチしない
		{FolderHash: "hash2", Title: "Song2", BodyURLs: []string{"http://example.com/unknown/entry"}, ChartCount: 1, IRCount: 1},
		// IR一部取得済み、IRCount=0（全譜面未取得）
		{FolderHash: "hash3", Title: "Song3", BodyURLs: []string{}, ChartCount: 2, IRCount: 0},
	}

	var upserted []model.SongMeta
	repo := &mockMetaRepo{
		listEventMappingsFunc: func(_ context.Context) ([]model.EventMapping, error) {
			return mappings, nil
		},
		listUnsetSongsWithIRURLsFunc: func(_ context.Context) ([]model.SongIRURLs, error) {
			return songs, nil
		},
		upsertSongMetaFunc: func(_ context.Context, meta model.SongMeta) error {
			upserted = append(upserted, meta)
			return nil
		},
	}

	uc := usecase.NewInferSongMetaUseCase(repo)
	result, err := uc.RunAutoInference(context.Background())

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// マッチなし
	if result.AutoSetCount != 0 {
		t.Errorf("AutoSetCount = %d, want %d", result.AutoSetCount, 0)
	}

	// 全て未マッチ
	if len(result.UnmatchedSongs) != 3 {
		t.Fatalf("UnmatchedSongs = %d件, want 3件", len(result.UnmatchedSongs))
	}

	// UpsertSongMetaは呼ばれない
	if len(upserted) != 0 {
		t.Errorf("UpsertSongMeta呼び出し回数 = %d, want %d", len(upserted), 0)
	}

	// NoIRCount: hash1とhash3がIRCount==0なので2
	if result.NoIRCount != 2 {
		t.Errorf("NoIRCount = %d, want %d", result.NoIRCount, 2)
	}
}
