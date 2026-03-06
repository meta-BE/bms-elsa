package bms

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
)

const MinHashSize = 64

// MinHashSignature はK=64のMinHash署名（256バイト）
type MinHashSignature [MinHashSize]uint32

// ComputeMinHash はファイル名集合からMinHash署名を計算する。
func ComputeMinHash(files []string) MinHashSignature {
	var sig MinHashSignature
	for i := range sig {
		sig[i] = math.MaxUint32
	}
	if len(files) == 0 {
		return sig
	}
	for _, f := range files {
		for i := 0; i < MinHashSize; i++ {
			h := fnv.New32a()
			// シードとしてインデックスを書き込み
			_ = binary.Write(h, binary.LittleEndian, uint32(i))
			h.Write([]byte(f))
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
	for i := 0; i < MinHashSize; i++ {
		if s[i] != math.MaxUint32 || other[i] != math.MaxUint32 {
			allMax = false
			break
		}
	}
	if allMax {
		return 1.0
	}

	match := 0
	for i := 0; i < MinHashSize; i++ {
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
