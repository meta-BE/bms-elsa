package similarity

import "strings"

// SongInfo は重複スキャン対象の楽曲情報
type SongInfo struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	ChartCount int
	Path       string
}

func (s SongInfo) GetTitle() string   { return s.Title }
func (s SongInfo) GetArtist() string  { return s.Artist }
func (s SongInfo) GetGenre() string   { return s.Genre }
func (s SongInfo) GetMinBPM() float64 { return s.MinBPM }
func (s SongInfo) GetMaxBPM() float64 { return s.MaxBPM }

// DuplicateGroup は重複候補のグループ
type DuplicateGroup struct {
	ID      int
	Members []DuplicateMember
	Score   int // グループ内の最高類似度（%）
}

// DuplicateMember はグループ内の各楽曲
type DuplicateMember struct {
	SongInfo
	Scores ScoreResult // グループ内で最も類似度が高いペアのスコア
}

// FindDuplicateGroups はartist正規化によるブロッキング→グループ内ファジー比較で重複グループを返す
func FindDuplicateGroups(songs []SongInfo, threshold int) []DuplicateGroup {
	// ブロッキング: artist正規化でグループ化
	blocks := make(map[string][]int)
	for i, s := range songs {
		key := strings.ToLower(strings.TrimSpace(s.Artist))
		blocks[key] = append(blocks[key], i)
	}

	// title完全一致ブロッキングも追加
	titleBlocks := make(map[string][]int)
	for i, s := range songs {
		key := strings.ToLower(strings.TrimSpace(s.Title))
		titleBlocks[key] = append(titleBlocks[key], i)
	}
	for _, indices := range titleBlocks {
		if len(indices) >= 2 {
			key := "__title__" + strings.ToLower(strings.TrimSpace(songs[indices[0]].Title))
			blocks[key] = indices
		}
	}

	// グループ内でペア比較
	type pair struct{ i, j int }
	matched := make(map[pair]ScoreResult)
	for _, indices := range blocks {
		if len(indices) < 2 {
			continue
		}
		for a := 0; a < len(indices); a++ {
			for b := a + 1; b < len(indices); b++ {
				p := pair{indices[a], indices[b]}
				if _, ok := matched[p]; ok {
					continue
				}
				result := Score(songs[p.i], songs[p.j])
				if result.Total >= threshold {
					matched[p] = result
				}
			}
		}
	}

	// Union-Findでグループ化
	parent := make([]int, len(songs))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(x, y int) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	bestScore := make(map[int]ScoreResult)
	for p, score := range matched {
		union(p.i, p.j)
		if score.Total > bestScore[p.i].Total {
			bestScore[p.i] = score
		}
		if score.Total > bestScore[p.j].Total {
			bestScore[p.j] = score
		}
	}

	// グループ集約
	groupMap := make(map[int][]int)
	for p := range matched {
		groupMap[find(p.i)] = nil
		groupMap[find(p.j)] = nil
	}
	for i := range songs {
		root := find(i)
		if _, ok := groupMap[root]; ok {
			groupMap[root] = append(groupMap[root], i)
		}
	}

	var groups []DuplicateGroup
	id := 1
	for _, indices := range groupMap {
		if len(indices) < 2 {
			continue
		}
		g := DuplicateGroup{ID: id}
		for _, idx := range indices {
			g.Members = append(g.Members, DuplicateMember{
				SongInfo: songs[idx],
				Scores:   bestScore[idx],
			})
			if bestScore[idx].Total > g.Score {
				g.Score = bestScore[idx].Total
			}
		}
		groups = append(groups, g)
		id++
	}

	return groups
}
