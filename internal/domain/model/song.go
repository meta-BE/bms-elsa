package model

import "time"

// Song は楽曲（フォルダ単位のグルーピング）
type Song struct {
	FolderHash string
	Title      string // 代表譜面から取得
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	Charts     []Chart
	// elsa.db メタデータ
	ReleaseYear *int
	EventName   *string
	// 一覧表示用の集約フィールド（リポジトリが設定）
	ChartCount int
	HasIRMeta  bool
}

// Chart は譜面（個々のBMSファイル）
type Chart struct {
	MD5        string
	SHA256     string
	Title      string
	Subtitle   string
	Artist     string
	SubArtist  string
	Genre      string
	Mode       int
	Difficulty int
	Level      int
	MinBPM     float64
	MaxBPM     float64
	Path       string
	Notes      int
	// elsa.db メタデータ
	IRMeta           *ChartIRMeta
	DifficultyLabels []DifficultyLabel
}

// SongMeta は楽曲レベルの追加メタデータ
type SongMeta struct {
	FolderHash  string
	ReleaseYear *int
	EventName   *string
}

// ChartIRMeta はLR2IR + 動作URLメタデータ
type ChartIRMeta struct {
	MD5            string
	SHA256         string
	Tags           []string
	LR2IRBodyURL   string
	LR2IRDiffURL   string
	LR2IRNotes     string
	WorkingBodyURL string
	WorkingDiffURL string
	FetchedAt      *time.Time
}

// DifficultyLabel は難易度表から取得した難易度ラベル
type DifficultyLabel struct {
	TableName string
	Symbol    string
	Level     string
}

// EventMapping はURLパターンとイベント名のマッピング
type EventMapping struct {
	ID          int
	URLPattern  string
	EventName   string
	ReleaseYear int
}

// SongIRURLs は未設定曲のfolderHashと紐づくIR本体URLの集約
type SongIRURLs struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	BodyURLs   []string // この曲の全譜面のlr2ir_body_url（空文字列除く）
	ChartCount int      // 総譜面数
	IRCount    int      // IR取得済み譜面数
}

// RewriteRule はURL書き換えルール
type RewriteRule struct {
	ID          int
	RuleType    string // "replace" or "regex"
	Pattern     string
	Replacement string
	Priority    int
}

// InstallCandidate は導入先推定の候補
type InstallCandidate struct {
	FolderPath string   // 楽曲フォルダのパス（songdata.songのpath/folderから導出）
	Title      string   // フォルダ内の代表タイトル
	Artist     string   // フォルダ内の代表アーティスト
	MatchTypes []string // マッチ理由: "title", "base_title", "body_url", "artist"
	Score      int      // マッチ手法のスコア合算
}
