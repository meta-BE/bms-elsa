# 負荷試験用データ生成ツール設計

## 目的
bms-elsaの負荷試験用に、10,000曲・約100,000譜面のsongdata.dbを生成するGoスクリプトを作成する。

## 出力
- `testdata/songdata_10k.db` — beatorajaのsongdata.dbと同一スキーマ
- 既存の `testdata/songdata.db` は変更しない

## ツール配置
- `cmd/gen-testdata/main.go` — `go run ./cmd/gen-testdata` で実行

## データ仕様

### folderテーブル（10,000レコード）
| カラム | 値 |
|--------|------|
| path | `G:\BMS\SONGS\loadtest\song_NNNN\`（NNNN=0000〜9999） |
| title | `LoadTest Song NNNN` |
| parent | 10個のイベントグループにランダム割り当て（8桁hex固定値） |
| type | 0 |
| date/adddate | 固定のUnixタイムスタンプ |

### songテーブル（約100,000レコード）
| カラム | 値 |
|--------|------|
| path | `G:\BMS\SONGS\loadtest\song_NNNN\chart_MM.bme` |
| folder | 対応folderの8桁hex |
| md5 | `fmt.Sprintf("%032x", songIdx*100+chartIdx)` で一意生成 |
| sha256 | `fmt.Sprintf("%064x", songIdx*100+chartIdx)` で一意生成 |
| charthash | sha256と同値 |
| title | `LoadTest Song NNNN` |
| artist | 100パターンからランダム（`Artist 001`〜`Artist 100`） |
| genre | 20パターンからランダム（`Genre 01`〜`Genre 20`） |
| difficulty | 1〜5を譜面インデックスで割り当て |
| level | 1〜12のランダム |
| mode | 7（7KEYS固定） |
| BPM | 100〜300のランダム（min=max） |
| notes | 200〜3000のランダム |
| length | 90000〜240000のランダム |

### 譜面数分布
1曲あたり1〜19譜面を均等に割り当て：
- 曲0〜525 → 1譜面
- 曲526〜1051 → 2譜面
- ...
- 曲9474〜9999 → 19譜面

合計: 約100,000譜面
