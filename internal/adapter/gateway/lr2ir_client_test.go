package gateway_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
)

// shiftJISEncode はUTF-8文字列をShift_JISにエンコードする。
// U+301C (WAVE DASH) などShift_JIS未対応文字は代替文字に置換する。
func shiftJISEncode(t *testing.T, utf8 string) []byte {
	t.Helper()
	encoder := encoding.ReplaceUnsupported(japanese.ShiftJIS.NewEncoder())
	r := transform.NewReader(
		strings.NewReader(utf8),
		encoder,
	)
	encoded, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Shift_JISエンコードに失敗: %v", err)
	}
	return encoded
}

func TestLR2IRClient_Registered(t *testing.T) {
	// fixtureをUTF-8で読み込み、Shift_JISにエンコードしてサーバーから返す
	utf8Body, err := os.ReadFile("testdata/registered_full.html")
	if err != nil {
		t.Fatal(err)
	}

	sjisBody := shiftJISEncode(t, string(utf8Body))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// クエリパラメータの検証
		if r.URL.Query().Get("mode") != "ranking" {
			t.Errorf("mode = %q, want %q", r.URL.Query().Get("mode"), "ranking")
		}
		if r.URL.Query().Get("bmsmd5") != "abc123" {
			t.Errorf("bmsmd5 = %q, want %q", r.URL.Query().Get("bmsmd5"), "abc123")
		}

		w.Header().Set("Content-Type", "text/html; charset=Shift_JIS")
		w.Write(sjisBody)
	}))
	defer server.Close()

	client := gateway.NewLR2IRClientWithBaseURL(server.URL)
	resp, err := client.LookupByMD5(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("LookupByMD5がエラーを返した: %v", err)
	}

	if !resp.Registered {
		t.Fatal("Registered should be true")
	}
	if resp.Genre != "DEMENTIA PROVECTO" {
		t.Errorf("Genre = %q, want %q", resp.Genre, "DEMENTIA PROVECTO")
	}
	if resp.Title != "Anima Mundi -Incipiens Finis- [EX]" {
		t.Errorf("Title = %q, want %q", resp.Title, "Anima Mundi -Incipiens Finis- [EX]")
	}
	if resp.Artist != "Limstella BGA:アカツキユウ / obj:仙人掌の人" {
		t.Errorf("Artist = %q, want %q", resp.Artist, "Limstella BGA:アカツキユウ / obj:仙人掌の人")
	}
	if resp.BPM != "95 - 190" {
		t.Errorf("BPM = %q, want %q", resp.BPM, "95 - 190")
	}
	if resp.Level != "☆21" {
		t.Errorf("Level = %q, want %q", resp.Level, "☆21")
	}
	if resp.Keys != "7KEYS" {
		t.Errorf("Keys = %q, want %q", resp.Keys, "7KEYS")
	}
	if resp.JudgeRank != "EASY" {
		t.Errorf("JudgeRank = %q, want %q", resp.JudgeRank, "EASY")
	}
	if len(resp.Tags) != 2 || resp.Tags[0] != "Stella" || resp.Tags[1] != "st2" {
		t.Errorf("Tags = %v, want [Stella st2]", resp.Tags)
	}
}

func TestLR2IRClient_Unregistered(t *testing.T) {
	utf8Body, err := os.ReadFile("testdata/unregistered.html")
	if err != nil {
		t.Fatal(err)
	}

	sjisBody := shiftJISEncode(t, string(utf8Body))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=Shift_JIS")
		w.Write(sjisBody)
	}))
	defer server.Close()

	client := gateway.NewLR2IRClientWithBaseURL(server.URL)
	resp, err := client.LookupByMD5(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("LookupByMD5がエラーを返した: %v", err)
	}

	if resp.Registered {
		t.Fatal("Registered should be false")
	}
}

func TestLR2IRClient_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// レスポンスを遅延させてキャンセルを確実にテストする
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := gateway.NewLR2IRClientWithBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 呼び出し前にキャンセル

	_, err := client.LookupByMD5(ctx, "abc123")
	if err == nil {
		t.Fatal("キャンセル済みコンテキストでエラーが返されるべき")
	}
}

func TestLR2IRClient_RateLimit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// 未登録レスポンス（Shift_JISエンコード）
		utf8 := `<html><body>この曲は登録されていません。</body></html>`
		sjis := shiftJISEncode(t, utf8)
		w.Header().Set("Content-Type", "text/html; charset=Shift_JIS")
		w.Write(sjis)
	}))
	defer server.Close()

	client := gateway.NewLR2IRClientWithBaseURL(server.URL)

	start := time.Now()

	_, err := client.LookupByMD5(context.Background(), "md5_1")
	if err != nil {
		t.Fatalf("1回目のLookupByMD5がエラー: %v", err)
	}

	_, err = client.LookupByMD5(context.Background(), "md5_2")
	if err != nil {
		t.Fatalf("2回目のLookupByMD5がエラー: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 500*time.Millisecond {
		t.Errorf("レートリミットが機能していない: 経過時間 %v（0.5秒以上必要）", elapsed)
	}

	if callCount != 2 {
		t.Errorf("サーバーへのリクエスト回数 = %d, want 2", callCount)
	}
}
