package similarity

import "testing"

func TestFindDuplicateGroups(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 3, Path: "/a"},
		{FolderHash: "bbb", Title: "FREEDOM DiVE", Artist: "xi", Genre: "ARROW", MinBPM: 150, MaxBPM: 222, ChartCount: 1, Path: "/b"},
		{FolderHash: "ccc", Title: "全く別の曲", Artist: "someone", Genre: "POP", MinBPM: 80, MaxBPM: 80, ChartCount: 2, Path: "/c"},
	}

	groups := FindDuplicateGroups(songs, nil, 40)

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

	groups := FindDuplicateGroups(songs, nil, 60)

	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}

func TestFindDuplicateGroups_MD5Only(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "曲A", Artist: "アーティスト1", Genre: "G1", MinBPM: 120, MaxBPM: 120, ChartCount: 2, Path: "/a"},
		{FolderHash: "bbb", Title: "曲A別名", Artist: "アーティスト2", Genre: "G2", MinBPM: 120, MaxBPM: 120, ChartCount: 1, Path: "/b"},
	}
	md5Pairs := []FolderPair{{FolderA: "aaa", FolderB: "bbb"}}

	groups := FindDuplicateGroups(songs, md5Pairs, 60)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if groups[0].Score != 100 {
		t.Errorf("Score = %d, want 100", groups[0].Score)
	}
	for _, m := range groups[0].Members {
		if !m.MD5Match {
			t.Errorf("member %s: MD5Match = false, want true", m.FolderHash)
		}
	}
}

func TestFindDuplicateGroups_MD5PlusFuzzy(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 2, Path: "/a"},
		{FolderHash: "bbb", Title: "FREEDOM", Artist: "xi", Genre: "ARROW", MinBPM: 100, MaxBPM: 200, ChartCount: 1, Path: "/b"},
		{FolderHash: "ccc", Title: "FREEDOM DiVE", Artist: "xi", Genre: "ARROW", MinBPM: 150, MaxBPM: 222, ChartCount: 1, Path: "/c"},
	}
	md5Pairs := []FolderPair{{FolderA: "aaa", FolderB: "bbb"}}

	groups := FindDuplicateGroups(songs, md5Pairs, 40)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Members) != 3 {
		t.Errorf("len(Members) = %d, want 3", len(groups[0].Members))
	}
	if groups[0].Score != 100 {
		t.Errorf("Score = %d, want 100", groups[0].Score)
	}
	md5MatchMap := map[string]bool{}
	for _, m := range groups[0].Members {
		md5MatchMap[m.FolderHash] = m.MD5Match
	}
	if !md5MatchMap["aaa"] {
		t.Error("aaa: MD5Match = false, want true")
	}
	if !md5MatchMap["bbb"] {
		t.Error("bbb: MD5Match = false, want true")
	}
	if md5MatchMap["ccc"] {
		t.Error("ccc: MD5Match = true, want false")
	}
}

func TestFindDuplicateGroups_MD5SkipsFuzzy(t *testing.T) {
	songs := []SongInfo{
		{FolderHash: "aaa", Title: "SAME", Artist: "SAME", Genre: "SAME", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/a"},
		{FolderHash: "bbb", Title: "SAME", Artist: "SAME", Genre: "SAME", MinBPM: 100, MaxBPM: 100, ChartCount: 1, Path: "/b"},
	}
	md5Pairs := []FolderPair{{FolderA: "aaa", FolderB: "bbb"}}

	groups := FindDuplicateGroups(songs, md5Pairs, 60)

	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	for _, m := range groups[0].Members {
		if m.Scores.Total != 100 {
			t.Errorf("member %s: Scores.Total = %d, want 100", m.FolderHash, m.Scores.Total)
		}
		if m.Scores.WAV != 0 {
			t.Errorf("member %s: Scores.WAV = %d, want 0 (fuzzy skipped)", m.FolderHash, m.Scores.WAV)
		}
	}
}
