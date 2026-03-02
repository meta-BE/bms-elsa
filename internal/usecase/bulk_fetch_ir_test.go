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
	upsertChartMetaCalls []model.ChartIRMeta
}

func (m *mockMetaRepoForBulk) UpsertChartMeta(_ context.Context, meta model.ChartIRMeta) error {
	m.upsertChartMetaCalls = append(m.upsertChartMetaCalls, meta)
	return nil
}

func TestBulkFetchIR_AllRegistered(t *testing.T) {
	repo := &mockMetaRepoForBulk{}
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			return &port.IRResponse{
				Registered: true,
				BodyURL:    "http://example.com/" + md5,
			}, nil
		},
	}

	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	md5s := []string{"aaa", "bbb"}
	var progresses []usecase.BulkFetchProgress
	result, err := uc.Execute(context.Background(), md5s, func(p usecase.BulkFetchProgress) {
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
	repo := &mockMetaRepoForBulk{}
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			if md5 == "found" {
				return &port.IRResponse{Registered: true, BodyURL: "http://example.com"}, nil
			}
			return &port.IRResponse{Registered: false}, nil
		},
	}

	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	md5s := []string{"found", "notfound"}
	result, err := uc.Execute(context.Background(), md5s, nil)

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
	repo := &mockMetaRepoForBulk{}
	callCount := 0
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, _ string) (*port.IRResponse, error) {
			callCount++
			return &port.IRResponse{Registered: true}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	md5s := []string{"aaa", "bbb", "ccc"}
	result, err := uc.Execute(ctx, md5s, func(p usecase.BulkFetchProgress) {
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
