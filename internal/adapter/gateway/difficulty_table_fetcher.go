package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// DifficultyTableHeader はheader.jsonの構造
type DifficultyTableHeader struct {
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
	DataURL string `json:"data_url"`
}

// DifficultyTableBodyEntry はbody JSONの1エントリ
type DifficultyTableBodyEntry struct {
	MD5     string `json:"md5"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	URL     string `json:"url"`
	URLDiff string `json:"url_diff"`
}

type DifficultyTableFetcher struct {
	client *http.Client
}

func NewDifficultyTableFetcher() *DifficultyTableFetcher {
	return &DifficultyTableFetcher{client: &http.Client{}}
}

var bmstableMetaRe = regexp.MustCompile(`<meta\s+name=["']bmstable["']\s+content=["']([^"']+)["']`)

// FetchHeaderURL はHTMLからbmstableメタタグのURLを取得する
func (f *DifficultyTableFetcher) FetchHeaderURL(tableURL string) (string, error) {
	body, err := f.get(tableURL)
	if err != nil {
		return "", fmt.Errorf("HTML取得失敗: %w", err)
	}

	matches := bmstableMetaRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("bmstableメタタグが見つかりません")
	}

	return resolveURL(tableURL, matches[1])
}

// FetchHeader はheader.jsonを取得する
func (f *DifficultyTableFetcher) FetchHeader(headerURL string) (*DifficultyTableHeader, error) {
	body, err := f.get(headerURL)
	if err != nil {
		return nil, fmt.Errorf("header.json取得失敗: %w", err)
	}

	var header DifficultyTableHeader
	if err := json.Unmarshal([]byte(body), &header); err != nil {
		return nil, fmt.Errorf("header.jsonパース失敗: %w", err)
	}

	// data_urlを絶対URLに変換
	absDataURL, err := resolveURL(headerURL, header.DataURL)
	if err != nil {
		return nil, fmt.Errorf("data_url解決失敗: %w", err)
	}
	header.DataURL = absDataURL

	return &header, nil
}

// FetchBody はbody JSONを取得する
func (f *DifficultyTableFetcher) FetchBody(dataURL string) ([]DifficultyTableBodyEntry, error) {
	body, err := f.get(dataURL)
	if err != nil {
		return nil, fmt.Errorf("body JSON取得失敗: %w", err)
	}

	var entries []DifficultyTableBodyEntry
	if err := json.Unmarshal([]byte(body), &entries); err != nil {
		return nil, fmt.Errorf("body JSONパース失敗: %w", err)
	}

	return entries, nil
}

func (f *DifficultyTableFetcher) get(targetURL string) (string, error) {
	resp, err := f.client.Get(targetURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, targetURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// resolveURL はbaseURLに対してrefを解決する
func resolveURL(baseURL, ref string) (string, error) {
	// 既に絶対URLならそのまま返す
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref, nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(refURL).String(), nil
}
