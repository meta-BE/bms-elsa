package usecase_test

import (
	"testing"

	"github.com/meta-BE/bms-elsa/internal/usecase"
)

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Test Song", "test song"},
		{"  Spaces  ", "spaces"},
		{"FULLWIDTH", "fullwidth"},
		{"全角カタカナ", "全角カタカナ"},
	}
	for _, c := range cases {
		got := usecase.NormalizeTitle(c.in)
		if got != c.want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripTrailingDecorations(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Title [ANBMS]", "Title"},
		{"Title (BMS Edition)", "Title"},
		{"Title -Remix-", "Title"},
		{"Title [A] (B)", "Title"},
		{"Plain", "Plain"},
	}
	for _, c := range cases {
		got := usecase.StripTrailingDecorations(c.in)
		if got != c.want {
			t.Errorf("StripTrailingDecorations(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScoreCandidate_TitleExact(t *testing.T) {
	score := usecase.ScoreCandidate(usecase.ScoreInput{
		QueryTitle:      "Test Song",
		QueryArtist:     "Artist",
		CandidateTitle:  "Test Song",
		CandidateArtist: "Artist",
	})
	// title 完全一致(60) + artist 完全一致(20) + token率(10) = 90
	if score < 80 {
		t.Errorf("score = %d, want >=80", score)
	}
}

func TestScoreCandidate_TitleNormalized(t *testing.T) {
	score := usecase.ScoreCandidate(usecase.ScoreInput{
		QueryTitle:      "Test Song",
		QueryArtist:     "Artist",
		CandidateTitle:  "test  song",
		CandidateArtist: "ARTIST",
	})
	// title 正規化後一致(50) + artist 正規化後一致(15) + α
	if score < 60 {
		t.Errorf("score = %d, want >=60", score)
	}
}

func TestScoreCandidate_BelowThreshold(t *testing.T) {
	score := usecase.ScoreCandidate(usecase.ScoreInput{
		QueryTitle:      "Test Song",
		QueryArtist:     "Artist",
		CandidateTitle:  "Completely Different",
		CandidateArtist: "Other",
	})
	if score >= 50 {
		t.Errorf("score = %d, want <50", score)
	}
}

func TestPickBestCandidate_Empty(t *testing.T) {
	got, ok := usecase.PickBestCandidate(nil, "T", "A", 50)
	if ok || got != -1 {
		t.Errorf("got idx=%d ok=%v, want -1, false", got, ok)
	}
}

func TestPickBestCandidate_BelowThreshold(t *testing.T) {
	cands := []usecase.ScoreCandidateRef{
		{Title: "Foo", Artist: "X"},
		{Title: "Bar", Artist: "Y"},
	}
	got, ok := usecase.PickBestCandidate(cands, "Test", "Artist", 50)
	if ok {
		t.Errorf("got idx=%d, want not ok (below threshold)", got)
	}
}

func TestPickBestCandidate_Tied(t *testing.T) {
	cands := []usecase.ScoreCandidateRef{
		{Title: "Test Song", Artist: "Artist"},
		{Title: "Test Song", Artist: "Artist"},
	}
	_, ok := usecase.PickBestCandidate(cands, "Test Song", "Artist", 50)
	if ok {
		t.Errorf("tied top should not be picked")
	}
}

func TestPickBestCandidate_Picked(t *testing.T) {
	cands := []usecase.ScoreCandidateRef{
		{Title: "Other", Artist: "Z"},
		{Title: "Test Song", Artist: "Artist"},
		{Title: "Different", Artist: "W"},
	}
	idx, ok := usecase.PickBestCandidate(cands, "Test Song", "Artist", 50)
	if !ok || idx != 1 {
		t.Errorf("got idx=%d ok=%v, want 1, true", idx, ok)
	}
}
