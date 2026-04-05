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
	WavMinHash []byte `json:"-"` // MinHash署名（フロントエンドには送らない）
}

func (s SongInfo) GetTitle() string      { return s.Title }
func (s SongInfo) GetArtist() string     { return s.Artist }
func (s SongInfo) GetGenre() string      { return s.Genre }
func (s SongInfo) GetMinBPM() float64    { return s.MinBPM }
func (s SongInfo) GetMaxBPM() float64    { return s.MaxBPM }
func (s SongInfo) GetWavMinHash() []byte { return s.WavMinHash }

// DuplicateGroup は重複候補のグループ
type DuplicateGroup struct {
	ID      int
	Members []DuplicateMember
	Score   int // グループ内の最高類似度（%）
}

// FolderPair はMD5完全一致で検出されたフォルダのペア
type FolderPair struct {
	FolderA string
	FolderB string
}

// DuplicateMember はグループ内の各楽曲
type DuplicateMember struct {
	SongInfo
	Scores   ScoreResult // グループ内で最も類似度が高いペアのスコア
	MD5Match bool        // MD5完全一致で検出されたか
}

// FindDuplicateGroups はMD5完全一致→ファジー比較の2段階で重複グループを返す
func FindDuplicateGroups(songs []SongInfo, md5Pairs []FolderPair, threshold int) []DuplicateGroup {
	// FolderHash → index マップ
	folderIdx := make(map[string]int, len(songs))
	for i, s := range songs {
		folderIdx[s.FolderHash] = i
	}

	// Union-Find
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
	md5Matched := make(map[[2]int]bool) // MD5一致済みペア
	md5MatchedIdx := make(map[int]bool) // MD5一致に関与したindex

	// --- 第1段階: MD5完全一致ペア ---
	md5Score := ScoreResult{Total: 100}
	for _, p := range md5Pairs {
		idxA, okA := folderIdx[p.FolderA]
		idxB, okB := folderIdx[p.FolderB]
		if !okA || !okB {
			continue
		}
		union(idxA, idxB)
		if md5Score.Total > bestScore[idxA].Total {
			bestScore[idxA] = md5Score
		}
		if md5Score.Total > bestScore[idxB].Total {
			bestScore[idxB] = md5Score
		}
		key := [2]int{idxA, idxB}
		if idxA > idxB {
			key = [2]int{idxB, idxA}
		}
		md5Matched[key] = true
		md5MatchedIdx[idxA] = true
		md5MatchedIdx[idxB] = true
	}

	// --- 第2段階: ファジーマッチ ---

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

	// グループ内でペア比較（MD5一致済みペアはスキップ）
	for _, indices := range blocks {
		if len(indices) < 2 {
			continue
		}
		for a := 0; a < len(indices); a++ {
			for b := a + 1; b < len(indices); b++ {
				idxA, idxB := indices[a], indices[b]
				key := [2]int{idxA, idxB}
				if idxA > idxB {
					key = [2]int{idxB, idxA}
				}
				if md5Matched[key] {
					continue
				}
				result := Score(songs[idxA], songs[idxB])
				if result.Total >= threshold {
					union(idxA, idxB)
					if result.Total > bestScore[idxA].Total {
						bestScore[idxA] = result
					}
					if result.Total > bestScore[idxB].Total {
						bestScore[idxB] = result
					}
				}
			}
		}
	}

	// グループ集約
	groupMap := make(map[int][]int)
	// MD5一致ペアのルートを登録
	for p := range md5Matched {
		groupMap[find(p[0])] = nil
		groupMap[find(p[1])] = nil
	}
	// ファジーマッチのルートを登録
	for i := range songs {
		root := find(i)
		if _, ok := groupMap[root]; ok {
			continue
		}
		if bestScore[i].Total > 0 {
			groupMap[root] = nil
		}
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
				MD5Match: md5MatchedIdx[idx],
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
