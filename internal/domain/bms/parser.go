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
	MD5          string         // ファイル全体のMD5ハッシュ（16進小文字32文字）
	Title        string         // #TITLE
	Subtitle     string         // #SUBTITLE
	Artist       string         // #ARTIST
	Subartist    string         // #SUBARTIST
	Genre        string         // #GENRE
	WAVFiles     []string       // WAV定義リスト（拡張子除去・小文字正規化済み）
	WAVRefCounts map[string]int // basename -> データセクション参照回数（#RANDOM #IF 1 のみ）
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
		MD5:          fmt.Sprintf("%x", hash),
		WAVRefCounts: make(map[string]int),
	}

	seen := make(map[string]struct{})
	slotToBasename := make(map[string]string) // 大文字 2 文字 slot -> basename
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
			slotPart, namePart, ok := strings.Cut(rest, " ")
			if !ok {
				continue
			}
			slot := strings.ToUpper(strings.TrimSpace(slotPart))
			if len(slot) != 2 {
				continue
			}
			filename := strings.TrimSpace(namePart)
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
			if _, exists := slotToBasename[slot]; !exists {
				slotToBasename[slot] = key
				if _, ok := result.WAVRefCounts[key]; !ok {
					result.WAVRefCounts[key] = 0
				}
			}
			continue
		}

		// データ行 #nnnCC:DDDD... の WAV 参照を集計
		// 対象チャンネル: 01, 11-19, 21-29, 31-39, 41-49, 51-59, 61-69
		if len(line) >= 7 && line[0] == '#' && isASCIIDigit(line[1]) && isASCIIDigit(line[2]) && isASCIIDigit(line[3]) {
			if line[6] != ':' {
				continue
			}
			channel := strings.ToUpper(line[4:6])
			if !isWAVReferenceChannel(channel) {
				continue
			}
			payload := strings.TrimSpace(line[7:])
			if len(payload) < 2 || len(payload)%2 != 0 {
				continue
			}
			for i := 0; i+1 < len(payload); i += 2 {
				slot := strings.ToUpper(payload[i : i+2])
				if slot == "00" {
					continue
				}
				if basename, ok := slotToBasename[slot]; ok {
					result.WAVRefCounts[basename]++
				} else {
					result.WAVRefCounts["__slot:"+slot]++
				}
			}
			continue
		}
	}
	// データ行が WAV 定義より前に出現していたケース: "__slot:XX" として保留したカウントを
	// basename に振り替える。
	for key, cnt := range result.WAVRefCounts {
		if !strings.HasPrefix(key, "__slot:") {
			continue
		}
		slot := key[len("__slot:"):]
		delete(result.WAVRefCounts, key)
		if basename, ok := slotToBasename[slot]; ok {
			result.WAVRefCounts[basename] += cnt
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

// isASCIIDigit は ASCII 数字 0-9 か判定する。
func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

// isWAVReferenceChannel は WAV を鳴らすチャンネルか判定する。
// 対象: 01 (BGM), 11-19, 21-29 (可視ノート), 31-39, 41-49 (不可視 keysound),
//       51-59, 61-69 (ロングノート)。
// 地雷 (D1-D9, E1-E9) は楽曲音源と無関係として除外する。
func isWAVReferenceChannel(ch string) bool {
	if len(ch) != 2 {
		return false
	}
	if ch == "01" {
		return true
	}
	switch ch[0] {
	case '1', '2', '3', '4', '5', '6':
		return ch[1] >= '1' && ch[1] <= '9'
	}
	return false
}
