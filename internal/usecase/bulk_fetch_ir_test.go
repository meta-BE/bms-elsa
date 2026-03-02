package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// mockMetaRepoForBulk はBulkFetchIRのテスト用モック
type mockMetaRepoForBulk struct {
	model.MetaRepository
	unfetchedKeys        []model.ChartKey
	upsertChartMetaCalls []model.ChartIRMeta
}

func (m *mockMetaRepoForBulk) ListUnfetchedChartKeys(_ context.Context) ([]model.ChartKey, error) {
	return m.unfetchedKeys, nil
}

func (m *mockMetaRepoForBulk) UpsertChartMeta(_ context.Context, meta model.ChartIRMeta) error {
	m.upsertChartMetaCalls = append(m.upsertChartMetaCalls, meta)
	return nil
}

func TestBulkFetchIR_AllRegistered(t *testing.T) {
	repo := &mockMetaRepoForBulk{
		unfetchedKeys: []model.ChartKey{
			{MD5: "aaa", SHA256: "sha_aaa"},
			{MD5: "bbb", SHA256: "sha_bbb"},
		},
	}
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			return &port.IRResponse{
				Registered: true,
				BodyURL:    "http://example.com/" + md5,
			}, nil
		},
	}

	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	var progresses []usecase.BulkFetchProgress
	result, err := uc.Execute(context.Background(), func(p usecase.BulkFetchProgress) {
		progresses = append(progresses, p)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if result.Fetched != 2 {
		t.Errorf("Fetched = %d, want 2", result.Fetched)
	}
	if result.NotFound != 0 {
		t.Errorf("NotFound = %d, want 0", result.NotFound)
	}
	if len(progresses) != 2 {
		t.Errorf("progress callbacks = %d, want 2", len(progresses))
	}
	if progresses[1].Current != 2 || progresses[1].Total != 2 {
		t.Errorf("last progress = %d/%d", progresses[1].Current, progresses[1].Total)
	}
}

func TestBulkFetchIR_MixedResults(t *testing.T) {
	repo := &mockMetaRepoForBulk{
		unfetchedKeys: []model.ChartKey{
			{MD5: "found", SHA256: "sha1"},
			{MD5: "notfound", SHA256: "sha2"},
		},
	}
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			if md5 == "found" {
				return &port.IRResponse{Registered: true, BodyURL: "http://example.com"}, nil
			}
			return &port.IRResponse{Registered: false}, nil
		},
	}

	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	result, err := uc.Execute(context.Background(), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("Fetched = %d, want 1", result.Fetched)
	}
	if result.NotFound != 1 {
		t.Errorf("NotFound = %d, want 1", result.NotFound)
	}
}

func TestBulkFetchIR_Cancellation(t *testing.T) {
	repo := &mockMetaRepoForBulk{
		unfetchedKeys: []model.ChartKey{
			{MD5: "aaa", SHA256: "sha1"},
			{MD5: "bbb", SHA256: "sha2"},
			{MD5: "ccc", SHA256: "sha3"},
		},
	}
	callCount := 0
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, _ string) (*port.IRResponse, error) {
			callCount++
			return &port.IRResponse{Registered: true}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	result, err := uc.Execute(ctx, func(p usecase.BulkFetchProgress) {
		if p.Current == 1 {
			cancel() // 1件目完了後にキャンセル
		}
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Cancelled {
		t.Error("expected Cancelled=true")
	}
	if result.Fetched < 1 {
		t.Errorf("Fetched = %d, want >= 1", result.Fetched)
	}
}
