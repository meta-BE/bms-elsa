package port

import "context"

// IRResponse はLR2IRの情報セクションのパース結果
type IRResponse struct {
	Registered bool
	Genre      string
	Title      string
	Artist     string
	BPM        string
	Level      string
	Keys       string
	JudgeRank  string
	Tags       []string
	BodyURL    string
	DiffURL    string
	Notes      string
}

// IRClient はLR2IRへのアクセスインターフェース
type IRClient interface {
	LookupByMD5(ctx context.Context, md5 string) (*IRResponse, error)
}
