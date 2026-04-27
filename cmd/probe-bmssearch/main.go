// Package main は BMS Search API のフォールバック検索仕様を確定するための調査スクリプト
// 使い方:
//
//	go run ./cmd/probe-bmssearch -songdata path/to/songdata.db -out docs/superpowers/specs/data/bmssearch-probe/YYYY-MM-DD
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const baseURL = "https://api.bmssearch.net/v1"

type sample struct {
	MD5    string
	Title  string
	Artist string
}

type queryResult struct {
	Variant   string          `json:"variant"`
	Query     string          `json:"query"`
	HTTPCode  int             `json:"httpCode"`
	Count     int             `json:"count"`
	Items     json.RawMessage `json:"items"`
	FetchedAt string          `json:"fetchedAt"`
}

func main() {
	var songdataPath, outDir string
	flag.StringVar(&songdataPath, "songdata", "testdata/songdata.db", "songdata.db path")
	flag.StringVar(&outDir, "out", "docs/superpowers/specs/data/bmssearch-probe/probe", "output dir for JSON dumps")
	flag.Parse()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	hits, misses, err := pickSamples(songdataPath)
	if err != nil {
		log.Fatal(err)
	}
	all := append(hits, misses...)

	for _, s := range all {
		variants := buildQueryVariants(s.Title)
		for _, v := range variants {
			res := fetchSearch(v.query)
			res.Variant = v.label
			fname := filepath.Join(outDir, fmt.Sprintf("%s_%s.json", s.MD5[:8], v.label))
			writeJSON(fname, map[string]any{
				"sample": s,
				"result": res,
			})
		}
	}
	fmt.Printf("done. %d samples × variants written to %s\n", len(all), outDir)
}

type variant struct{ label, query string }

func buildQueryVariants(title string) []variant {
	return []variant{
		{label: "raw", query: title},
		{label: "normalized", query: normalizeTitle(title)},
		{label: "stripped", query: stripTrailingBrackets(title)},
	}
}

// 暫定実装: 設計ドキュメントの初期案に沿った正規化（実調査結果で更新する）
func normalizeTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// TODO: 全半角統一・記号除去は調査結果を踏まえて拡充
	return s
}

func stripTrailingBrackets(s string) string {
	s = strings.TrimSpace(s)
	for {
		trimmed := s
		// 末尾の [...] / (...) / -...- を1段階ずつ剥離
		for _, pair := range [][2]string{{"[", "]"}, {"(", ")"}, {"-", "-"}} {
			if strings.HasSuffix(trimmed, pair[1]) {
				idx := strings.LastIndex(trimmed, pair[0])
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

func fetchSearch(title string) queryResult {
	q := url.Values{}
	q.Set("title", title)
	q.Set("limit", "20")
	q.Set("orderBy", "PUBLISHED")
	q.Set("orderDirection", "DESC")
	u := baseURL + "/bmses/search?" + q.Encode()
	time.Sleep(150 * time.Millisecond)
	resp, err := http.Get(u)
	if err != nil {
		return queryResult{Query: title, HTTPCode: -1, FetchedAt: time.Now().Format(time.RFC3339)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var arr []json.RawMessage
	_ = json.Unmarshal(body, &arr)
	return queryResult{
		Query:     title,
		HTTPCode:  resp.StatusCode,
		Count:     len(arr),
		Items:     body,
		FetchedAt: time.Now().Format(time.RFC3339),
	}
}

// pickSamples は調査用に song テーブルから md5 昇順に5件、md5 降順に5件のサンプルを取得する
// （song_meta は elsa DB にあり songdata.db には存在しないため、ヒット/ミス区別は出力 JSON を目視で確認する運用）
func pickSamples(songdataPath string) (hits, misses []sample, err error) {
	db, err := sql.Open("sqlite", songdataPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	ctx := context.Background()

	hitRows, err := db.QueryContext(ctx, `
		SELECT s.md5, s.title, s.artist
		FROM song s
		WHERE s.md5 != '' AND s.title != ''
		ORDER BY s.md5
		LIMIT 5`)
	if err != nil {
		return nil, nil, err
	}
	defer hitRows.Close()
	for hitRows.Next() {
		var s sample
		if err := hitRows.Scan(&s.MD5, &s.Title, &s.Artist); err != nil {
			return nil, nil, err
		}
		hits = append(hits, s)
	}

	missRows, err := db.QueryContext(ctx, `
		SELECT s.md5, s.title, s.artist
		FROM song s
		WHERE s.md5 != '' AND s.title != ''
		ORDER BY s.md5 DESC
		LIMIT 5`)
	if err != nil {
		return nil, nil, err
	}
	defer missRows.Close()
	for missRows.Next() {
		var s sample
		if err := missRows.Scan(&s.MD5, &s.Title, &s.Artist); err != nil {
			return nil, nil, err
		}
		misses = append(misses, s)
	}
	return hits, misses, nil
}

func writeJSON(path string, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}
