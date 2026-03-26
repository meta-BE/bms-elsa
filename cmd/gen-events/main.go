// BMS Search APIの全exhibitionと既存event_mappings.csvをマージしてevents.csvを生成するCLIツール
// 使い方: go run ./cmd/gen-events
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	apiBase    = "https://api.bmssearch.net/v1/exhibitions/search"
	pageLimit  = 100
	rateDelay  = 500 * time.Millisecond
	mappingCSV = "internal/adapter/persistence/event_mappings.csv"
	outputCSV  = "internal/adapter/persistence/events.csv"
)

type exhibition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Terms struct {
		Entry *struct {
			StartsAt string `json:"startsAt"`
		} `json:"entry"`
	} `json:"terms"`
	CreatedAt string `json:"createdAt"`
}

type eventRow struct {
	BmsSearchID string
	Name        string
	ShortName   string
	ReleaseYear string
}

func main() {
	// 1. BMS Search APIから全exhibition取得
	fmt.Fprintln(os.Stderr, "BMS Search APIからexhibitionを取得中...")
	exhibitions, err := fetchAllExhibitions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "API取得エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "API: %d件取得\n", len(exhibitions))

	// 2. APIデータをeventRowに変換
	var rows []eventRow
	bmsSearchNames := make(map[string]bool) // 重複チェック用（名前ベース）
	for _, ex := range exhibitions {
		year := extractYear(ex)
		row := eventRow{
			BmsSearchID: ex.ID,
			Name:        ex.Name,
			ShortName:   ex.Name,
			ReleaseYear: year,
		}
		rows = append(rows, row)
		bmsSearchNames[ex.Name] = true
	}

	// 3. 既存event_mappings.csv読み込み
	mappingRows, err := loadMappings(mappingCSV)
	if err != nil {
		fmt.Fprintf(os.Stderr, "event_mappings.csv読み込みエラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "event_mappings.csv: %d件読み込み\n", len(mappingRows))

	// 4. マージ（BMS Searchに存在しないイベントのみ追加）
	seen := make(map[string]bool) // event_mappings内の重複排除
	var added int
	for _, mr := range mappingRows {
		if seen[mr.Name] {
			continue
		}
		seen[mr.Name] = true

		if bmsSearchNames[mr.Name] {
			continue
		}
		rows = append(rows, mr)
		added++
	}
	fmt.Fprintf(os.Stderr, "event_mappings.csvから%d件追加（BMS Searchと重複しない分）\n", added)

	// 5. release_yearの降順、同年はname昇順でソート
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ReleaseYear != rows[j].ReleaseYear {
			return rows[i].ReleaseYear > rows[j].ReleaseYear
		}
		return rows[i].Name < rows[j].Name
	})

	// 6. CSV出力
	f, err := os.Create(outputCSV)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CSV作成エラー: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"bms_search_id", "name", "short_name", "release_year"})
	for _, row := range rows {
		w.Write([]string{row.BmsSearchID, row.Name, row.ShortName, row.ReleaseYear})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "CSV書き込みエラー: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "完了: %s に%d件出力\n", outputCSV, len(rows))
}

// fetchAllExhibitions はページネーションで全exhibitionを取得する
func fetchAllExhibitions() ([]exhibition, error) {
	var all []exhibition
	client := &http.Client{Timeout: 30 * time.Second}

	for offset := 0; ; offset += pageLimit {
		url := fmt.Sprintf("%s?limit=%d&offset=%d", apiBase, pageLimit, offset)
		fmt.Fprintf(os.Stderr, "  GET %s\n", url)

		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("offset=%d: %w", offset, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("offset=%d body読み込み: %w", offset, err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("offset=%d: status=%d body=%s", offset, resp.StatusCode, string(body))
		}

		var page []exhibition
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("offset=%d JSON解析: %w", offset, err)
		}

		all = append(all, page...)

		if len(page) < pageLimit {
			break
		}

		time.Sleep(rateDelay)
	}

	return all, nil
}

// extractYear はexhibitionからrelease_yearを抽出する
// 優先: terms.entry.startsAt → createdAt
func extractYear(ex exhibition) string {
	if ex.Terms.Entry != nil && ex.Terms.Entry.StartsAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, ex.Terms.Entry.StartsAt); err == nil {
			return fmt.Sprintf("%d", t.Year())
		}
		// ミリ秒付きISO 8601形式のフォールバック
		if t, err := time.Parse("2006-01-02T15:04:05.000Z", ex.Terms.Entry.StartsAt); err == nil {
			return fmt.Sprintf("%d", t.Year())
		}
	}

	if ex.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, ex.CreatedAt); err == nil {
			return fmt.Sprintf("%d", t.Year())
		}
		if t, err := time.Parse("2006-01-02T15:04:05.000Z", ex.CreatedAt); err == nil {
			return fmt.Sprintf("%d", t.Year())
		}
	}

	return ""
}

// loadMappings は既存のevent_mappings.csvを読み込む
// 形式: url_pattern,event_name,release_year
func loadMappings(path string) ([]eventRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	// ヘッダースキップ
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("ヘッダー読み込みエラー: %w", err)
	}

	var rows []eventRow
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 3 {
			continue
		}

		name := strings.TrimSpace(rec[1])
		year := strings.TrimSpace(rec[2])
		if name == "" {
			continue
		}

		rows = append(rows, eventRow{
			BmsSearchID: "", // event_mappings由来はbms_search_idなし
			Name:        name,
			ShortName:   name,
			ReleaseYear: year,
		})
	}

	return rows, nil
}
