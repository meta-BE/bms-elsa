package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupPatternByMD5_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/patterns/abc123" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"bms":{"id":"BMS-1","title":"Test"},"title":"Test [A]","artist":"Artist"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	client := NewBMSSearchClientWithBaseURL(srv.URL)
	p, err := client.LookupPatternByMD5(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil || p.BMS.ID != "BMS-1" {
		t.Errorf("expected BMS ID BMS-1, got %+v", p)
	}
}

func TestLookupPatternByMD5_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()
	client := NewBMSSearchClientWithBaseURL(srv.URL)
	p, err := client.LookupPatternByMD5(context.Background(), "nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil, got %+v", p)
	}
}

func TestLookupBMS_WithExhibition(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bmses/BMS-1" {
			w.Write([]byte(`{"id":"BMS-1","exhibition":{"id":"EX-1","name":"Test Event"},"publishedAt":"2024-01-01T00:00:00Z"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	client := NewBMSSearchClientWithBaseURL(srv.URL)
	b, err := client.LookupBMS(context.Background(), "BMS-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil || b.Exhibition == nil || b.Exhibition.ID != "EX-1" {
		t.Errorf("expected exhibition EX-1, got %+v", b)
	}
}
