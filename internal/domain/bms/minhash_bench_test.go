package bms_test

import (
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"testing"

	"modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction(
		"minhash_similarity",
		2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			blob1, ok1 := args[0].([]byte)
			blob2, ok2 := args[1].([]byte)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("expected BLOB arguments")
			}
			if len(blob1) != 256 || len(blob2) != 256 {
				return 0.0, nil
			}
			match := 0
			for i := 0; i < 64; i++ {
				v1 := binary.LittleEndian.Uint32(blob1[i*4:])
				v2 := binary.LittleEndian.Uint32(blob2[i*4:])
				if v1 == v2 {
					match++
				}
			}
			return float64(match) / 64.0, nil
		},
	)
}

// generateRandomMinhash はランダムなMinHash署名を生成する
func generateRandomMinhash() []byte {
	buf := make([]byte, 256)
	rand.Read(buf)
	return buf
}

func BenchmarkMinHashSimilarity_GoScan(b *testing.B) {
	// 3000件のユニークminhashを生成（実データの想定サイズ）
	const numRecords = 3000
	records := make([]bms.MinHashSignature, numRecords)
	for i := range records {
		buf := generateRandomMinhash()
		sig, _ := bms.MinHashFromBytes(buf)
		records[i] = sig
	}

	// クエリ用minhash
	queryBuf := generateRandomMinhash()
	query, _ := bms.MinHashFromBytes(queryBuf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bestSim := 0.0
		bestIdx := -1
		for j, rec := range records {
			sim := query.Similarity(rec)
			if sim > bestSim {
				bestSim = sim
				bestIdx = j
			}
		}
		_ = bestIdx
	}
}

func BenchmarkMinHashSimilarity_SQLiteCustomFunc(b *testing.B) {
	const numRecords = 3000

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE chart_meta (md5 TEXT PRIMARY KEY, wav_minhash BLOB)`)
	if err != nil {
		b.Fatal(err)
	}

	// データ投入
	tx, _ := db.Begin()
	stmt, _ := tx.Prepare(`INSERT INTO chart_meta (md5, wav_minhash) VALUES (?, ?)`)
	for i := 0; i < numRecords; i++ {
		stmt.Exec(fmt.Sprintf("md5_%d", i), generateRandomMinhash())
	}
	stmt.Close()
	tx.Commit()

	queryMinhash := generateRandomMinhash()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var bestMD5 string
		var bestSim float64
		err := db.QueryRow(
			`SELECT md5, minhash_similarity(?, wav_minhash) as sim
			 FROM chart_meta
			 WHERE wav_minhash IS NOT NULL
			 ORDER BY sim DESC LIMIT 1`,
			queryMinhash,
		).Scan(&bestMD5, &bestSim)
		if err != nil {
			b.Fatal(err)
		}
	}
}
