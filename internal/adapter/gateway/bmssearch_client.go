package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBMSSearchBaseURL  = "https://api.bmssearch.net/v1"
	bmsSearchRequestInterval = 100 * time.Millisecond
)

type BMSSearchPattern struct {
	BMS struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"bms"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type BMSSearchBMS struct {
	ID          string               `json:"id"`
	Exhibition  *BMSSearchExhibition `json:"exhibition"`
	PublishedAt string               `json:"publishedAt"`
}

type BMSSearchExhibition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BMSSearchExhibitionDetail struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Terms *struct {
		Entry *struct {
			StartsAt string `json:"startsAt"`
		} `json:"entry"`
	} `json:"terms"`
	LinkedProfile *struct {
		Websites []struct {
			URL string `json:"url"`
		} `json:"websites"`
	} `json:"linkedProfile"`
	CreatedAt string `json:"createdAt"`
}

type BMSSearchClient struct {
	client   *http.Client
	baseURL  string
	mu       sync.Mutex
	lastReq  time.Time
	interval time.Duration
}

func NewBMSSearchClient() *BMSSearchClient {
	return &BMSSearchClient{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  defaultBMSSearchBaseURL,
		interval: bmsSearchRequestInterval,
	}
}

func NewBMSSearchClientWithBaseURL(baseURL string) *BMSSearchClient {
	c := NewBMSSearchClient()
	c.baseURL = baseURL
	return c
}

func (c *BMSSearchClient) LookupPatternByMD5(ctx context.Context, md5 string) (*BMSSearchPattern, error) {
	c.rateLimit()
	url := fmt.Sprintf("%s/patterns/%s", c.baseURL, md5)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BMS Search API: HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var pattern BMSSearchPattern
	if err := json.Unmarshal(body, &pattern); err != nil {
		return nil, fmt.Errorf("BMS Search pattern parse: %w", err)
	}
	return &pattern, nil
}

func (c *BMSSearchClient) LookupBMS(ctx context.Context, bmsID string) (*BMSSearchBMS, error) {
	c.rateLimit()
	url := fmt.Sprintf("%s/bmses/%s", c.baseURL, bmsID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BMS Search API: HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var bms BMSSearchBMS
	if err := json.Unmarshal(body, &bms); err != nil {
		return nil, fmt.Errorf("BMS Search BMS parse: %w", err)
	}
	return &bms, nil
}

func (c *BMSSearchClient) FetchAllExhibitions(ctx context.Context) ([]BMSSearchExhibitionDetail, error) {
	var all []BMSSearchExhibitionDetail
	offset := 0
	limit := 100
	for {
		c.rateLimit()
		url := fmt.Sprintf("%s/exhibitions/search?offset=%d&limit=%d", c.baseURL, offset, limit)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("BMS Search exhibitions: HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var batch []BMSSearchExhibitionDetail
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("exhibitions parse: %w", err)
		}
		all = append(all, batch...)
		if len(batch) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *BMSSearchClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastReq)
	if elapsed < c.interval {
		time.Sleep(c.interval - elapsed)
	}
	c.lastReq = time.Now()
}
