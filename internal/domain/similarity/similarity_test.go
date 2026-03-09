package similarity

import (
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
)

func TestPrefixRatio(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"FREEDOM", "FREEDOM DiVE", 0.583}, // 7/12
		{"ABC", "ABC", 1.0},
		{"ABC", "XYZ", 0.0},
		{"", "", 0.0},
		{"A", "", 0.0},
	}
	for _, tt := range tests {
		got := PrefixRatio(tt.a, tt.b)
		if diff := got - tt.want; diff > 0.01 || diff < -0.01 {
			t.Errorf("PrefixRatio(%q, %q) = %.3f, want %.3f", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBPMOverlap(t *testing.T) {
	tests := []struct {
		minA, maxA, minB, maxB float64
		want                   float64
	}{
		{120, 180, 150, 200, 1.0},
		{120, 130, 140, 150, 0.0},
		{100, 100, 100, 100, 1.0},
	}
	for _, tt := range tests {
		got := BPMOverlap(tt.minA, tt.maxA, tt.minB, tt.maxB)
		if got != tt.want {
			t.Errorf("BPMOverlap(%.0f-%.0f, %.0f-%.0f) = %.1f, want %.1f",
				tt.minA, tt.maxA, tt.minB, tt.maxB, got, tt.want)
		}
	}
}

type testSong struct {
	title, artist, genre string
	minBPM, maxBPM       float64
	wavMinHash           []byte
}

func (s testSong) GetTitle() string      { return s.title }
func (s testSong) GetArtist() string     { return s.artist }
func (s testSong) GetGenre() string      { return s.genre }
func (s testSong) GetMinBPM() float64    { return s.minBPM }
func (s testSong) GetMaxBPM() float64    { return s.maxBPM }
func (s testSong) GetWavMinHash() []byte { return s.wavMinHash }

func TestScore(t *testing.T) {
	// MinHash なし: Title 58*0.20=11.6, Artist 100*0.15=15, Genre 100*0.05=5, BPM 100*0.10=10 → Total≈41
	a := testSong{"FREEDOM", "xi", "ARROW", 100, 200, nil}
	b := testSong{"FREEDOM DiVE", "xi", "ARROW", 150, 222, nil}
	result := Score(a, b)

	if result.Total < 40 || result.Total > 42 {
		t.Errorf("Total = %d, want ~41 (no MinHash)", result.Total)
	}
	if result.Title < 50 {
		t.Errorf("Title = %d, want >= 50", result.Title)
	}
	if result.Artist != 100 {
		t.Errorf("Artist = %d, want 100", result.Artist)
	}
}

func TestScoreWithMinHash(t *testing.T) {
	// 同一 MinHash 署名を作成（Jaccard = 1.0）
	sig := bms.ComputeMinHash([]string{"bgm01", "kick", "snare"})
	mh := sig.Bytes()

	a := testSong{"FREEDOM", "xi", "ARROW", 100, 200, mh}
	b := testSong{"FREEDOM DiVE", "xi", "ARROW", 150, 222, mh}
	result := Score(a, b)

	// MinHash 100*0.50=50, Title 58*0.20≈11, Artist 100*0.15=15, Genre 100*0.05=5, BPM 100*0.10=10 → Total≈91
	if result.Total < 89 || result.Total > 93 {
		t.Errorf("Total = %d, want ~91 (identical MinHash)", result.Total)
	}
}

func TestScoreWithDifferentMinHash(t *testing.T) {
	sigA := bms.ComputeMinHash([]string{"bgm01", "kick", "snare"})
	sigB := bms.ComputeMinHash([]string{"piano", "bass", "hihat"})

	a := testSong{"SongA", "ArtistA", "Genre", 120, 120, sigA.Bytes()}
	b := testSong{"SongA", "ArtistA", "Genre", 120, 120, sigB.Bytes()}
	result := Score(a, b)

	// 完全にWAVが異なる → MinHash ≈ 0
	// Title 100*0.20=20, Artist 100*0.15=15, Genre 100*0.05=5, BPM 100*0.10=10 → Total≈50
	if result.Total > 55 {
		t.Errorf("Total = %d, want <= 55 (different MinHash)", result.Total)
	}
}

func TestScoreWithOneMinHashMissing(t *testing.T) {
	sig := bms.ComputeMinHash([]string{"bgm01", "kick"})

	a := testSong{"Song", "Artist", "Genre", 120, 120, sig.Bytes()}
	b := testSong{"Song", "Artist", "Genre", 120, 120, nil}
	result := Score(a, b)

	// 片方 MinHash なし → WAV=0, 残り最大50 → Total=50
	if result.Total > 50 {
		t.Errorf("Total = %d, want <= 50 (one MinHash missing)", result.Total)
	}
}
