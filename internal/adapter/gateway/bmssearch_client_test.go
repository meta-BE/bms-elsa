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

func TestSearchBMSesByTitle_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bmses/search" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("title") != "Test Song" || q.Get("limit") != "20" ||
			q.Get("orderBy") != "PUBLISHED" || q.Get("orderDirection") != "DESC" {
			t.Errorf("unexpected query: %v", q)
		}
		w.Write([]byte(`[
			{"id":"bms-1","title":"Test Song","artist":"A","subartist":"","genre":"G",
			 "exhibition":{"id":"ex","name":"BOFXX"},"publishedAt":"2024-01-01",
			 "downloads":[{"url":"https://x","description":"本体"}],
			 "previews":[{"service":"YOUTUBE","parameter":"abc"}],
			 "relatedLinks":[]},
			{"id":"bms-2","title":"Other","artist":"B"}
		]`))
	}))
	defer srv.Close()
	c := NewBMSSearchClientWithBaseURL(srv.URL)
	got, err := c.SearchBMSesByTitle(context.Background(), "Test Song", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "bms-1" {
		t.Errorf("got = %+v", got)
	}
	if len(got[0].Downloads) != 1 || got[0].Downloads[0].URL != "https://x" {
		t.Errorf("downloads = %+v", got[0].Downloads)
	}
}

func TestSearchBMSesByTitle_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := NewBMSSearchClientWithBaseURL(srv.URL)
	got, err := c.SearchBMSesByTitle(context.Background(), "nope", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want empty", got)
	}
}

func TestLookupBMS_FullFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"BMS-1","title":"T","artist":"A","subartist":"S","genre":"G",
			"exhibition":{"id":"EX","name":"E"},"publishedAt":"2024",
			"downloads":[{"url":"u","description":"d"}],
			"previews":[{"service":"YOUTUBE","parameter":"p"}],
			"relatedLinks":[{"url":"r","description":"rd"}]}`))
	}))
	defer srv.Close()
	c := NewBMSSearchClientWithBaseURL(srv.URL)
	b, err := c.LookupBMS(context.Background(), "BMS-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "T" || b.SubArtist != "S" || b.Genre != "G" {
		t.Errorf("metadata mismatch: %+v", b)
	}
	if len(b.Downloads) != 1 || len(b.Previews) != 1 || len(b.RelatedLinks) != 1 {
		t.Errorf("array fields mismatch: %+v", b)
	}
}
