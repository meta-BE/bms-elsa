package similarity

import "testing"

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
}

func (s testSong) GetTitle() string   { return s.title }
func (s testSong) GetArtist() string  { return s.artist }
func (s testSong) GetGenre() string   { return s.genre }
func (s testSong) GetMinBPM() float64 { return s.minBPM }
func (s testSong) GetMaxBPM() float64 { return s.maxBPM }

func TestScore(t *testing.T) {
	a := testSong{"FREEDOM", "xi", "ARROW", 100, 200}
	b := testSong{"FREEDOM DiVE", "xi", "ARROW", 150, 222}
	result := Score(a, b)

	if result.Total < 50 {
		t.Errorf("Total = %d, want >= 50", result.Total)
	}
	if result.Title < 50 {
		t.Errorf("Title = %d, want >= 50", result.Title)
	}
	if result.Artist != 100 {
		t.Errorf("Artist = %d, want 100", result.Artist)
	}
}
