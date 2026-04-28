package usecase

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeTitle はタイトルを比較用に正規化する。
// 大小無視・前後空白除去・連続空白の単一化・NFKC 正規化（全半角統一）。
func NormalizeTitle(s string) string {
	s = norm.NFKC.String(s)
	s = strings.ToLower(strings.TrimSpace(s))
	// 連続空白を1つに圧縮
	out := strings.Builder{}
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				out.WriteRune(' ')
			}
			prevSpace = true
		} else {
			out.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(out.String())
}

// StripTrailingDecorations は末尾の [...] / (...) / -...- を再帰的に剥離する。
// "Title [A] (B)" → "Title"
func StripTrailingDecorations(s string) string {
	s = strings.TrimSpace(s)
	for {
		trimmed := s
		for _, pair := range [][2]rune{{'[', ']'}, {'(', ')'}, {'-', '-'}} {
			if len(trimmed) > 0 && rune(trimmed[len(trimmed)-1]) == pair[1] {
				idx := strings.LastIndexByte(trimmed[:len(trimmed)-1], byte(pair[0]))
				if idx > 0 {
					trimmed = strings.TrimSpace(trimmed[:idx])
				}
			}
		}
		if trimmed == s {
			break
		}
		s = trimmed
	}
	return s
}

type ScoreInput struct {
	QueryTitle      string
	QueryArtist     string
	CandidateTitle  string
	CandidateArtist string
}

// ScoreCandidate は候補1件のスコアを計算する（最大90点）。
// title 系3項目は最高1項目のみ採用、artist 系2項目も同様。token 率は独立加算。
func ScoreCandidate(in ScoreInput) int {
	score := 0

	// title 系（排他）
	titleScore := 0
	if in.QueryTitle == in.CandidateTitle {
		titleScore = 60
	} else if NormalizeTitle(in.QueryTitle) == NormalizeTitle(in.CandidateTitle) {
		titleScore = 50
	} else {
		nq := NormalizeTitle(in.QueryTitle)
		nc := NormalizeTitle(in.CandidateTitle)
		if nq != "" && nc != "" && (strings.Contains(nc, nq) || strings.Contains(nq, nc)) {
			titleScore = 25
		}
	}
	score += titleScore

	// artist 系（排他）
	artistScore := 0
	if in.QueryArtist != "" {
		if in.QueryArtist == in.CandidateArtist {
			artistScore = 20
		} else if NormalizeTitle(in.QueryArtist) == NormalizeTitle(in.CandidateArtist) {
			artistScore = 15
		}
	}
	score += artistScore

	// artist トークン共通率（独立加算、最大10点）
	score += artistTokenScore(in.QueryArtist, in.CandidateArtist)

	return score
}

func artistTokenScore(a, b string) int {
	at := tokenize(NormalizeTitle(a))
	bt := tokenize(NormalizeTitle(b))
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	common := 0
	bset := make(map[string]struct{}, len(bt))
	for _, t := range bt {
		bset[t] = struct{}{}
	}
	for _, t := range at {
		if _, ok := bset[t]; ok {
			common++
		}
	}
	denom := len(at)
	if len(bt) > denom {
		denom = len(bt)
	}
	return common * 10 / denom
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '/' || r == ',' || r == '&'
	})
	return fields
}

// ScoreCandidateRef はスコアリング対象の最小情報
type ScoreCandidateRef struct {
	Title  string
	Artist string
}

// PickBestCandidate は候補群から最高スコアのインデックスを返す。
// 閾値未満 / 同点首位の場合は ok=false を返す。
func PickBestCandidate(cands []ScoreCandidateRef, queryTitle, queryArtist string, threshold int) (int, bool) {
	if len(cands) == 0 {
		return -1, false
	}
	bestIdx := -1
	bestScore := -1
	tie := false
	for i, c := range cands {
		s := ScoreCandidate(ScoreInput{
			QueryTitle:      queryTitle,
			QueryArtist:     queryArtist,
			CandidateTitle:  c.Title,
			CandidateArtist: c.Artist,
		})
		if s > bestScore {
			bestScore = s
			bestIdx = i
			tie = false
		} else if s == bestScore {
			tie = true
		}
	}
	if bestScore < threshold || tie {
		return -1, false
	}
	return bestIdx, true
}
