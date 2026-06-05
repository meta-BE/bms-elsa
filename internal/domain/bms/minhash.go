package bms

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
)

const MinHashSize = 64

// MinHashSignature はK=64のMinHash署名（256バイト）
type MinHashSignature [MinHashSize]uint32

// bucketThresholds は累積タグの閾値（2 の冪）。
// count >= threshold[i] の各 i について "n:<basename>#t<threshold>" を集合に追加する。
// 注意: 昇順前提。ComputeMinHash の閾値ループは初回ミスで break するため順序が必須。
var bucketThresholds = [...]int{1, 2, 4, 8, 16, 32, 64}

// ComputeMinHash は basename と参照回数から K=64 の MinHash 署名を計算する。
//
// 入力集合の構築:
//   - 各 basename b について常に "n:" + b を追加（参照回数 0 でも残す）
//   - count が bucketThresholds の各値以上のとき、累積タグ "n:" + b + "#t" + 閾値 を追加
//
// この設計により:
//   - 同曲難易度差分間で参照回数が小さく揺れても、跨ぐバケット数が少なければ類似度は維持される
//   - 同じ basename だけ偶然一致するが参照回数階層が違う別曲は、類似度が大きく下がる
func ComputeMinHash(refCounts map[string]int) MinHashSignature {
	elements := make([]string, 0, len(refCounts)*4)
	for basename, count := range refCounts {
		base := "n:" + basename
		elements = append(elements, base)
		for _, t := range bucketThresholds {
			if count >= t {
				elements = append(elements, base+"#t"+strconv.Itoa(t))
			} else {
				break
			}
		}
	}
	return computeMinHashFromElements(elements)
}

func computeMinHashFromElements(elements []string) MinHashSignature {
	var sig MinHashSignature
	for i := range sig {
		sig[i] = math.MaxUint32
	}
	if len(elements) == 0 {
		return sig
	}
	for _, e := range elements {
		for i := range MinHashSize {
			h := fnv.New32a()
			_ = binary.Write(h, binary.LittleEndian, uint32(i))
			h.Write([]byte(e))
			v := h.Sum32()
			if v < sig[i] {
				sig[i] = v
			}
		}
	}
	return sig
}

// Similarity は2つのMinHash署名のJaccard類似度の近似値を返す（0.0〜1.0）。
func (s MinHashSignature) Similarity(other MinHashSignature) float64 {
	// 両方が空集合（全てMaxUint32）の場合は1.0
	allMax := true
	for i := range MinHashSize {
		if s[i] != math.MaxUint32 || other[i] != math.MaxUint32 {
			allMax = false
			break
		}
	}
	if allMax {
		return 1.0
	}

	match := 0
	for i := range MinHashSize {
		if s[i] == other[i] {
			match++
		}
	}
	return float64(match) / float64(MinHashSize)
}

// Bytes はMinHash署名を256バイトのバイト列にシリアライズする。
func (s MinHashSignature) Bytes() []byte {
	buf := make([]byte, MinHashSize*4)
	for i, v := range s {
		binary.LittleEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}

// MinHashFromBytes は256バイトのバイト列からMinHash署名を復元する。
func MinHashFromBytes(data []byte) (MinHashSignature, error) {
	if len(data) != MinHashSize*4 {
		return MinHashSignature{}, fmt.Errorf("invalid minhash data length: %d", len(data))
	}
	var sig MinHashSignature
	for i := range sig {
		sig[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return sig, nil
}
