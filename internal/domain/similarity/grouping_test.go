package similarity

import "testing"

func TestFindDuplicateGroups(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 3, Path: "/a"},
		{FolderHash: "bbb", Title: "FREEDOM DiVE", Artist: "xi", Genre: "ARROW", MinBPM: 150, MaxBPM: 222, ChartCount: 1, Path: "/b"},
		{FolderHash: "ccc", Title: "全く別の曲", Artist: "someone", Genre: "POP", MinBPM: 80, MaxBPM: 80, ChartCount: 2, Path: "/c"},
	}

	// MinHash なしのため、スコアは約41（Title 58*0.20 + Artist 100*0.15 + Genre 100*0.05 + BPM 100*0.10）
	groups := FindDuplicateGroups(songs, 40)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 2 {
		t.Errorf("len(groups[0].Members) = %d, want 2", len(groups[0].Members))
	}
	if groups[0].Score < 40 {
		t.Errorf("Score = %d, want >= 40", groups[0].Score)
	}
}

func TestFindDuplicateGroups_NoMatch(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "曲A", Artist: "アーティスト1", Genre: "G1", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/a"},
		{FolderHash: "bbb", Title: "曲B", Artist: "アーティスト2", Genre: "G2", MinBPM: 200, MaxBPM: 200, ChartCount: 1, Path: "/b"},
	}

	groups := FindDuplicateGroups(songs, 60)

	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}
