package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/meta-BE/bms-elsa/internal/port"
)

const (
	lr2irBaseURL       = "http://www.dream-pro.info/~lavalse/LR2IR/search.cgi"
	minRequestInterval = 500 * time.Millisecond
)

// LR2IRClient はLR2IRへのHTTPクライアント実装
type LR2IRClient struct {
	client   *http.Client
	baseURL  string
	mu       sync.Mutex
	lastReq  time.Time
	interval time.Duration
}

func NewLR2IRClient() *LR2IRClient {
	return &LR2IRClient{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  lr2irBaseURL,
		interval: minRequestInterval,
	}
}

// NewLR2IRClientWithBaseURL はテスト用にbaseURLを差し替え可能にするコンストラクタ
func NewLR2IRClientWithBaseURL(baseURL string) *LR2IRClient {
	return &LR2IRClient{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  baseURL,
		interval: minRequestInterval,
	}
}

// SetInterval はリクエスト間隔を設定する（デフォルト: 500ms）
func (c *LR2IRClient) SetInterval(d time.Duration) {
	c.interval = d
}

var _ port.IRClient = (*LR2IRClient)(nil)

func (c *LR2IRClient) LookupByMD5(ctx context.Context, md5 string) (*port.IRResponse, error) {
	c.mu.Lock()
	var wait time.Duration
	if !c.lastReq.IsZero() {
		elapsed := time.Since(c.lastReq)
		if elapsed < c.interval {
			wait = c.interval - elapsed
		}
	}
	c.mu.Unlock()

	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	c.mu.Lock()
	c.lastReq = time.Now()
	c.mu.Unlock()

	url := fmt.Sprintf("%s?mode=ranking&bmsmd5=%s", c.baseURL, md5)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエスト作成に失敗: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエストに失敗: %w", err)
	}
	defer resp.Body.Close()

	// Shift_JISからUTF-8へデコード
	utf8Reader := transform.NewReader(resp.Body, japanese.ShiftJIS.NewDecoder())
	body, err := io.ReadAll(utf8Reader)
	if err != nil {
		return nil, fmt.Errorf("レスポンス読み取りに失敗: %w", err)
	}

	return ParseLR2IRResponse(string(body))
}
