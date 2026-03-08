package bms

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// ParsedBMS はBMSファイルのパース結果を保持する。
type ParsedBMS struct {
	MD5       string   // ファイル全体のMD5ハッシュ（16進小文字32文字）
	Title     string   // #TITLE
	Subtitle  string   // #SUBTITLE
	Artist    string   // #ARTIST
	Subartist string   // #SUBARTIST
	Genre     string   // #GENRE
	WAVFiles  []string // WAV定義リスト（拡張子除去・小文字正規化済み）
}

// ParseBMSFile はBMSファイルをパースし、ヘッダー・WAV定義・MD5を抽出する。
// RANDOM内は#IF 1のブロックのみ処理する。
// ヘッダーフィールドは最初にヒットした値を採用する。
func ParseBMSFile(path string) (*ParsedBMS, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hash := md5.Sum(data)

	// Shift-JIS → UTF-8 変換（BMSの事実上の標準エンコーディング）
	if !utf8.Valid(data) {
		decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), japanese.ShiftJIS.NewDecoder()))
		if err == nil {
			data = decoded
		}
	}

	result := &ParsedBMS{
		MD5: fmt.Sprintf("%x", hash),
	}

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// RANDOM処理用: スキップ中のネスト深さ（0=スキップしていない）
	type randomState struct {
		active bool // このRANDOMブロック内で現在の#IFが処理対象（=1）か
	}
	var stack []randomState
	skipDepth := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] != '#' {
			continue
		}

		upper := strings.ToUpper(line)

		// RANDOM制御
		if strings.HasPrefix(upper, "#RANDOM ") {
			if skipDepth > 0 {
				skipDepth++
			} else {
				stack = append(stack, randomState{active: false})
			}
			continue
		}
		if strings.HasPrefix(upper, "#ENDRANDOM") {
			if skipDepth > 0 {
				skipDepth--
			} else if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if strings.HasPrefix(upper, "#IF ") {
			if skipDepth > 0 {
				skipDepth++
				continue
			}
			if len(stack) > 0 {
				val := strings.TrimSpace(line[4:])
				if val == "1" {
					stack[len(stack)-1].active = true
				} else {
					skipDepth = 1
				}
			}
			continue
		}
		if strings.HasPrefix(upper, "#ENDIF") {
			if skipDepth > 0 {
				skipDepth--
				if skipDepth == 0 && len(stack) > 0 {
					stack[len(stack)-1].active = false
				}
			} else if len(stack) > 0 {
				stack[len(stack)-1].active = false
			}
			continue
		}

		// スキップ中なら無視
		if skipDepth > 0 {
			continue
		}

		// ヘッダーフィールドの抽出（最初にヒットした値を採用）
		if strings.HasPrefix(upper, "#TITLE ") && result.Title == "" {
			result.Title = strings.TrimSpace(line[7:])
			continue
		}
		if strings.HasPrefix(upper, "#SUBTITLE ") && result.Subtitle == "" {
			result.Subtitle = strings.TrimSpace(line[10:])
			continue
		}
		if strings.HasPrefix(upper, "#ARTIST ") && result.Artist == "" {
			result.Artist = strings.TrimSpace(line[8:])
			continue
		}
		if strings.HasPrefix(upper, "#SUBARTIST ") && result.Subartist == "" {
			result.Subartist = strings.TrimSpace(line[11:])
			continue
		}
		if strings.HasPrefix(upper, "#GENRE ") && result.Genre == "" {
			result.Genre = strings.TrimSpace(line[7:])
			continue
		}

		// #WAVxx の処理
		if len(upper) >= 6 && upper[:4] == "#WAV" && upper[4] != ' ' {
			rest := line[4:]
			spaceIdx := strings.IndexByte(rest, ' ')
			if spaceIdx < 0 {
				continue
			}
			filename := strings.TrimSpace(rest[spaceIdx+1:])
			if filename == "" {
				continue
			}
			// 拡張子を除去してベース名にする
			ext := filepath.Ext(filename)
			if ext != "" {
				filename = filename[:len(filename)-len(ext)]
			}
			key := strings.ToLower(filename)
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	wavFiles := make([]string, 0, len(seen))
	for name := range seen {
		wavFiles = append(wavFiles, name)
	}
	sort.Strings(wavFiles)
	result.WAVFiles = wavFiles

	return result, nil
}
