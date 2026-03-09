package similarity

import (
	"github.com/meta-BE/bms-elsa/internal/domain/bms"
)

// Comparable は類似度計算に必要なフィールドを持つ型
type Comparable interface {
	GetTitle() string
	GetArtist() string
	GetGenre() string
	GetMinBPM() float64
	GetMaxBPM() float64
	GetWavMinHash() []byte
}

// ScoreResult は各フィールドの類似度（%）と総合スコア
type ScoreResult struct {
	Title  int // 0-100
	Artist int // 0-100
	Genre  int // 0-100
	BPM    int // 0 or 100
	Total  int // 加重平均 0-100
}

// PrefixRatio は2つの文字列の前方一致文字数率を返す（0.0〜1.0）
func PrefixRatio(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 0
	}
	match := 0
	minLen := len(ra)
	if len(rb) < minLen {
		minLen = len(rb)
	}
	for i := 0; i < minLen; i++ {
		if ra[i] != rb[i] {
			break
		}
		match++
	}
	return float64(match) / float64(maxLen)
}

// BPMOverlap はBPM範囲が重なっていれば1.0、そうでなければ0.0を返す
func BPMOverlap(minA, maxA, minB, maxB float64) float64 {
	if minA <= maxB && minB <= maxA {
		return 1.0
	}
	return 0.0
}

// Score は2つのComparableの類似度を計算する
func Score(a, b Comparable) ScoreResult {
	titleR := PrefixRatio(a.GetTitle(), b.GetTitle())
	artistR := PrefixRatio(a.GetArtist(), b.GetArtist())
	genreR := PrefixRatio(a.GetGenre(), b.GetGenre())
	bpmR := BPMOverlap(a.GetMinBPM(), a.GetMaxBPM(), b.GetMinBPM(), b.GetMaxBPM())

	var wavR float64
	mhA, mhB := a.GetWavMinHash(), b.GetWavMinHash()
	if len(mhA) > 0 && len(mhB) > 0 {
		sigA, errA := bms.MinHashFromBytes(mhA)
		sigB, errB := bms.MinHashFromBytes(mhB)
		if errA == nil && errB == nil {
			wavR = sigA.Similarity(sigB)
		}
	}

	total := wavR*0.50 + titleR*0.20 + artistR*0.15 + genreR*0.05 + bpmR*0.10

	return ScoreResult{
		Title:  int(titleR * 100),
		Artist: int(artistR * 100),
		Genre:  int(genreR * 100),
		BPM:    int(bpmR * 100),
		Total:  int(total * 100),
	}
}
