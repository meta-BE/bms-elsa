package bms

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ParseWAVFiles はBMSファイルからWAV定義のファイル名集合を抽出する。
// RANDOM内は#IF 1のブロックのみ処理する。
// ファイル名は拡張子を除去したベース名で返す（大文字小文字を保持しない：小文字正規化）。
func ParseWAVFiles(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)

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

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
