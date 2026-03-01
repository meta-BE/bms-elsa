// 負荷試験用songdata.dbを生成するスクリプト
// 使い方: go run ./cmd/gen-testdata
package main

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

const (
	numSongs       = 10000
	maxChartsPerSong = 19
	numArtists     = 100
	numGenres      = 20
	numParents     = 10
)

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	outPath := filepath.Join(projectRoot, "testdata", "songdata_10k.db")

	os.Remove(outPath)

	db, err := sql.Open("sqlite", outPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// スキーマ作成
	for _, stmt := range []string{
		`CREATE TABLE song (
			md5 TEXT NOT NULL, sha256 TEXT NOT NULL,
			title TEXT, subtitle TEXT, genre TEXT, artist TEXT, subartist TEXT, tag TEXT,
			path TEXT PRIMARY KEY, folder TEXT,
			stagefile TEXT, banner TEXT, backbmp TEXT, preview TEXT, parent TEXT,
			level INTEGER, difficulty INTEGER, maxbpm INTEGER, minbpm INTEGER,
			length INTEGER, mode INTEGER, judge INTEGER, feature INTEGER, content INTEGER,
			date INTEGER, favorite INTEGER, adddate INTEGER, notes INTEGER, charthash TEXT
		)`,
		`CREATE TABLE folder (
			title TEXT, subtitle TEXT, command TEXT, path TEXT PRIMARY KEY,
			banner TEXT, parent TEXT, type INTEGER, date INTEGER, adddate INTEGER, max INTEGER
		)`,
		`CREATE INDEX idx_song_folder ON song(folder)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("schema: %v", err)
		}
	}

	rng := rand.New(rand.NewSource(42))
	baseDate := int(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix())

	// 親グループ（イベント）の固定ハッシュ
	parents := make([]string, numParents)
	for i := range parents {
		parents[i] = fmt.Sprintf("%08x", md5.Sum([]byte(fmt.Sprintf("event_%d", i))))[:8]
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	folderStmt, err := tx.Prepare(`INSERT INTO folder (path, title, subtitle, command, banner, parent, type, date, adddate, max) VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		log.Fatal(err)
	}

	songStmt, err := tx.Prepare(`INSERT INTO song (md5, sha256, title, subtitle, genre, artist, subartist, tag, path, folder, stagefile, banner, backbmp, preview, parent, level, difficulty, maxbpm, minbpm, length, mode, judge, feature, content, date, favorite, adddate, notes, charthash) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		log.Fatal(err)
	}

	totalCharts := 0

	for songIdx := 0; songIdx < numSongs; songIdx++ {
		// 譜面数: 1〜19の均等分布
		chartsForSong := (songIdx*maxChartsPerSong)/numSongs + 1

		folderHash := fmt.Sprintf("%08x", md5.Sum([]byte(fmt.Sprintf("folder_%d", songIdx))))[:8]
		folderPath := fmt.Sprintf(`G:\BMS\SONGS\loadtest\song_%04d\`, songIdx)
		songTitle := fmt.Sprintf("LoadTest Song %04d", songIdx)
		parent := parents[rng.Intn(numParents)]
		artist := fmt.Sprintf("Artist %03d", rng.Intn(numArtists)+1)
		genre := fmt.Sprintf("Genre %02d", rng.Intn(numGenres)+1)

		if _, err := folderStmt.Exec(folderPath, songTitle, "", "", "", parent, 0, baseDate, baseDate, 0); err != nil {
			log.Fatalf("folder %d: %v", songIdx, err)
		}

		for chartIdx := 0; chartIdx < chartsForSong; chartIdx++ {
			uid := songIdx*100 + chartIdx
			chartMD5 := fmt.Sprintf("%032x", uid)
			chartSHA256 := fmt.Sprintf("%064x", uid)
			chartPath := fmt.Sprintf(`%schart_%02d.bme`, folderPath, chartIdx)
			difficulty := (chartIdx % 5) + 1
			level := rng.Intn(12) + 1
			bpm := rng.Intn(201) + 100
			notes := rng.Intn(2801) + 200
			length := rng.Intn(150001) + 90000
			subtitle := ""
			if chartIdx > 0 {
				subtitle = fmt.Sprintf("[DIFF %d]", chartIdx)
			}

			if _, err := songStmt.Exec(
				chartMD5, chartSHA256,
				songTitle, subtitle, genre, artist, "", "",
				chartPath, folderHash,
				"", "", "", "", parent,
				level, difficulty, bpm, bpm, length, 7, 100, 0, 3,
				baseDate, 0, baseDate, notes, chartSHA256,
			); err != nil {
				log.Fatalf("song %d chart %d: %v", songIdx, chartIdx, err)
			}
			totalCharts++
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("生成完了: %s\n", outPath)
	fmt.Printf("楽曲数: %d, 譜面数: %d\n", numSongs, totalCharts)
}
