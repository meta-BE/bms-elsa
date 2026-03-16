package dto

import (
	"strings"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type SongListDTO struct {
	Songs      []SongRowDTO `json:"songs"`
	TotalCount int          `json:"totalCount"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
}

type SongRowDTO struct {
	FolderHash  string  `json:"folderHash"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Genre       string  `json:"genre"`
	Path        string  `json:"path"`
	MinBPM      float64 `json:"minBpm"`
	MaxBPM      float64 `json:"maxBpm"`
	EventName   *string `json:"eventName"`
	ReleaseYear *int    `json:"releaseYear"`
	HasIRMeta   bool    `json:"hasIrMeta"`
	ChartCount  int     `json:"chartCount"`
}

type SongDetailDTO struct {
	FolderHash  string     `json:"folderHash"`
	Title       string     `json:"title"`
	Artist      string     `json:"artist"`
	Genre       string     `json:"genre"`
	EventName   *string    `json:"eventName"`
	ReleaseYear *int       `json:"releaseYear"`
	Charts      []ChartDTO `json:"charts"`
}

type ChartDTO struct {
	MD5            string  `json:"md5"`
	SHA256         string  `json:"sha256"`
	Title          string  `json:"title"`
	Subtitle       string  `json:"subtitle,omitempty"`
	Artist         string  `json:"artist,omitempty"`
	SubArtist      string  `json:"subArtist,omitempty"`
	Mode           int     `json:"mode"`
	Difficulty     int     `json:"difficulty"`
	Level          int     `json:"level"`
	MinBPM         float64 `json:"minBpm"`
	MaxBPM         float64 `json:"maxBpm"`
	Path           string  `json:"path,omitempty"`
	Notes          int     `json:"notes"`
	HasIRMeta      bool    `json:"hasIrMeta"`
	LR2IRTags      string  `json:"lr2irTags,omitempty"`
	LR2IRBodyURL   string  `json:"lr2irBodyUrl,omitempty"`
	LR2IRDiffURL   string  `json:"lr2irDiffUrl,omitempty"`
	LR2IRNotes     string  `json:"lr2irNotes,omitempty"`
	WorkingBodyURL string  `json:"workingBodyUrl,omitempty"`
	WorkingDiffURL   string               `json:"workingDiffUrl,omitempty"`
	DifficultyLabels []DifficultyLabelDTO `json:"difficultyLabels,omitempty"`
}

// ChartIRMetaDTO はchart_metaのIR情報のみ（未導入譜面の詳細表示用）
type ChartIRMetaDTO struct {
	MD5            string `json:"md5"`
	HasIRMeta      bool   `json:"hasIrMeta"`
	LR2IRTags      string `json:"lr2irTags,omitempty"`
	LR2IRBodyURL   string `json:"lr2irBodyUrl,omitempty"`
	LR2IRDiffURL   string `json:"lr2irDiffUrl,omitempty"`
	LR2IRNotes     string `json:"lr2irNotes,omitempty"`
	WorkingBodyURL string `json:"workingBodyUrl,omitempty"`
	WorkingDiffURL string `json:"workingDiffUrl,omitempty"`
}

// ChartListItemDTO は譜面一覧用の軽量DTO
type ChartListItemDTO struct {
	MD5         string  `json:"md5"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle,omitempty"`
	Artist      string  `json:"artist"`
	SubArtist   string  `json:"subArtist,omitempty"`
	Genre       string  `json:"genre"`
	Path        string  `json:"path"`
	MinBPM      float64 `json:"minBpm"`
	MaxBPM      float64 `json:"maxBpm"`
	Difficulty  int     `json:"difficulty"`
	Notes       int     `json:"notes"`
	EventName   string  `json:"eventName,omitempty"`
	ReleaseYear int     `json:"releaseYear,omitempty"`
	HasIRMeta   bool    `json:"hasIrMeta"`
}

type DifficultyLabelDTO struct {
	TableName string `json:"tableName"`
	Symbol    string `json:"symbol"`
	Level     string `json:"level"`
}

type InferenceResultDTO struct {
	AutoSetCount   int             `json:"autoSetCount"`
	UnmatchedSongs []SongIRURLsDTO `json:"unmatchedSongs"`
	NoIRCount      int             `json:"noIRCount"`
}

type SongIRURLsDTO struct {
	FolderHash string   `json:"folderHash"`
	Title      string   `json:"title"`
	Artist     string   `json:"artist"`
	Genre      string   `json:"genre"`
	BodyURLs   []string `json:"bodyUrls"`
	ChartCount int      `json:"chartCount"`
	IRCount    int      `json:"irCount"`
}

type EventMappingDTO struct {
	ID          int    `json:"id"`
	URLPattern  string `json:"urlPattern"`
	EventName   string `json:"eventName"`
	ReleaseYear int    `json:"releaseYear"`
}

func SongToRowDTO(s model.Song) SongRowDTO {
	return SongRowDTO{
		FolderHash:  s.FolderHash,
		Title:       s.Title,
		Artist:      s.Artist,
		Genre:       s.Genre,
		Path:        s.Path,
		MinBPM:      s.MinBPM,
		MaxBPM:      s.MaxBPM,
		EventName:   s.EventName,
		ReleaseYear: s.ReleaseYear,
		HasIRMeta:   s.HasIRMeta,
		ChartCount:  s.ChartCount,
	}
}

func SongToDetailDTO(s model.Song) SongDetailDTO {
	charts := make([]ChartDTO, len(s.Charts))
	for i, c := range s.Charts {
		charts[i] = ChartToDTO(c)
	}
	return SongDetailDTO{
		FolderHash:  s.FolderHash,
		Title:       s.Title,
		Artist:      s.Artist,
		Genre:       s.Genre,
		EventName:   s.EventName,
		ReleaseYear: s.ReleaseYear,
		Charts:      charts,
	}
}

func ChartToDTO(c model.Chart) ChartDTO {
	d := ChartDTO{
		MD5:        c.MD5,
		SHA256:     c.SHA256,
		Title:      c.Title,
		Subtitle:   c.Subtitle,
		Artist:     c.Artist,
		SubArtist:  c.SubArtist,
		Mode:       c.Mode,
		Difficulty: c.Difficulty,
		Level:      c.Level,
		MinBPM:     c.MinBPM,
		MaxBPM:     c.MaxBPM,
		Path:       c.Path,
		Notes:      c.Notes,
		HasIRMeta:  c.IRMeta != nil,
	}
	if c.IRMeta != nil {
		d.LR2IRTags = strings.Join(c.IRMeta.Tags, ",")
		d.LR2IRBodyURL = c.IRMeta.LR2IRBodyURL
		d.LR2IRDiffURL = c.IRMeta.LR2IRDiffURL
		d.LR2IRNotes = c.IRMeta.LR2IRNotes
		d.WorkingBodyURL = c.IRMeta.WorkingBodyURL
		d.WorkingDiffURL = c.IRMeta.WorkingDiffURL
	}
	if c.DifficultyLabels != nil {
		d.DifficultyLabels = make([]DifficultyLabelDTO, len(c.DifficultyLabels))
		for i, l := range c.DifficultyLabels {
			d.DifficultyLabels[i] = DifficultyLabelDTO{
				TableName: l.TableName, Symbol: l.Symbol, Level: l.Level,
			}
		}
	}
	return d
}

type DifficultyTableDTO struct {
	ID         int     `json:"id"`
	URL        string  `json:"url"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	EntryCount int     `json:"entryCount"`
	FetchedAt  *string `json:"fetchedAt"`
}

type DifficultyTableRefreshResult struct {
	TableName  string `json:"tableName"`
	Success    bool   `json:"success"`
	EntryCount int    `json:"entryCount"`
	Error      string `json:"error,omitempty"`
}

type DifficultyTableEntryDTO struct {
	MD5            string `json:"md5"`
	Level          string `json:"level"`
	Title          string `json:"title"`
	Artist         string `json:"artist"`
	URL            string `json:"url"`
	URLDiff        string `json:"urlDiff"`
	Status         string `json:"status"`
	InstalledCount int    `json:"installedCount"`
}

type RewriteRuleDTO struct {
	ID          int    `json:"id"`
	RuleType    string `json:"ruleType"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Priority    int    `json:"priority"`
}

type InferWorkingURLResultDTO struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

type InstallCandidateDTO struct {
	FolderPath string   `json:"folderPath"`
	Title      string   `json:"title"`
	Artist     string   `json:"artist"`
	MatchTypes []string `json:"matchTypes"`
	Score      int      `json:"score"`
}

type MoveSongFolderResultDTO struct {
	DestPath  string `json:"destPath"`
	FileCount int    `json:"fileCount"`
}

func ChartIRMetaToDTO(m model.ChartIRMeta) ChartIRMetaDTO {
	hasIR := m.FetchedAt != nil
	d := ChartIRMetaDTO{
		MD5:       m.MD5,
		HasIRMeta: hasIR,
	}
	if hasIR {
		d.LR2IRTags = strings.Join(m.Tags, ",")
		d.LR2IRBodyURL = m.LR2IRBodyURL
		d.LR2IRDiffURL = m.LR2IRDiffURL
		d.LR2IRNotes = m.LR2IRNotes
		d.WorkingBodyURL = m.WorkingBodyURL
		d.WorkingDiffURL = m.WorkingDiffURL
	}
	return d
}
