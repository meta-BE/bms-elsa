# wav_minhash アルゴリズム v2 設計

## 背景

`chart_meta.wav_minhash` は譜面の WAV 定義集合から計算する MinHash 署名（K=64, 256 byte）で、譜面類似度判定（`similarity.Score()` の 50% 重み）、差分自動インストール先推定（`EstimateDiffInstall`）、重複検出（`ScanDuplicates`）の基盤となっている。

現行 v1 アルゴリズム（`internal/domain/bms/parser.go` + `internal/domain/bms/minhash.go`）は **WAV ファイル名の basename（拡張子除去・小文字化）の集合** から MinHash を作る。実データ（17,135 譜面）の検証で、`00.wav 01.wav 02.wav ...` のような数値命名スキーマを使う譜面同士で署名が完全一致し、別曲（Laplace / Kronos / The World of Cyber / Night Resolution など）が 100% 類似と誤判定される問題が発見された。クロスフォルダで完全一致するハッシュは 58 種、影響譜面 451 件に及ぶ。

## 目的

WAV 定義の basename だけでなく、譜面内での **参照回数**（自動再生・プレイチャンネル合算）を MinHash 入力に組み込み、楽曲としての同一性判定の精度を上げる。差分譜面間（同曲別難易度）で参照回数が小さく揺れた場合の安定性も担保する。

## 対象外

- 物理 WAV ファイルへのアクセス（サイズ・コンテンツハッシュ）。譜面ファイル単体での配布を許容するため
- BGA / BMP / TEXT 系チャンネル。WAV を参照しない
- 地雷チャンネル D1-D9 / E1-E9。仕様上は WAV 参照を持つが、楽曲音源としての扱いはノイズに近いと判断し除外
- #RANDOM 全分岐ユニオン処理。既存の #IF 1 のみ採用を踏襲

## 設計概要

### 新アルゴリズム

各 BMS ファイルから以下を抽出する:

1. **WAV basename 集合**: `#WAVxx filename.ext` から拡張子除去・小文字化した basename
2. **参照回数**: データセクション `#nnnCC:DDDD...` のうち、CC が WAV 参照チャンネルである行で各 WAV スロットへの参照回数を集計

参照対象チャンネル:

| チャンネル | 用途 |
|---|---|
| `01` | BGM (自動再生) |
| `11-19` | 1P 可視ノート（含 scratch=16, foot=17, 7key拡張=18-19）|
| `21-29` | 2P 可視ノート |
| `31-39` | 1P 不可視 keysound |
| `41-49` | 2P 不可視 keysound |
| `51-59` | 1P ロングノート |
| `61-69` | 2P ロングノート |

除外チャンネル: `02`（小節長）, `03`/`08`（BPM）, `04`/`06`/`07`（BGA）, `05`（BM98拡張）, `09`（STOP）, `99`（TEXT）, `A0-A6`（OPTION）, `D1-D9`/`E1-E9`（地雷）

MinHash 入力集合は各 (basename, count) について以下を追加する:

```
"n:" + basename                           ← 必ず追加
"n:" + basename + "#t1"   if count >= 1
"n:" + basename + "#t2"   if count >= 2
"n:" + basename + "#t4"   if count >= 4
"n:" + basename + "#t8"   if count >= 8
"n:" + basename + "#t16"  if count >= 16
"n:" + basename + "#t32"  if count >= 32
"n:" + basename + "#t64"  if count >= 64
```

累積タグ方式により、参照回数の小さな揺れ（バケット境界を跨がない）は類似度に影響せず、大きく異なる（複数階層跨ぐ）場合のみ類似度が低下する。

### #RANDOM 処理

既存パーサと同様、`#IF 1` のブロックのみ処理する。`#IF 1` 以外のブロック内の `#WAVxx` 定義と参照は集計対象外。

### 検証結果（v1 → v2）

`testdata/songdata.db` の 17,123 譜面で `cmd/validate-minhash/` により事前検証済み:

- **クロスフォルダ衝突解消**: 旧 100% 衝突 1,373 ペア中 531 ペア (38.7%) が新方式で 90% 未満に低下。Laplace × Kronos などの偽陽性は 57-69% 帯まで明確に分離
- **同フォルダ同曲難易度差分の維持**: 95% → 92% に微減。劣化分の多くは「別差分作者によるキーサウンディング設計違い」の正しい分離（例: ピアノ協奏曲蠍火フォルダの "izigen" 譜面が他と分離されるなど）

### 互換性

新方式と旧方式の署名形式は同じ 256 byte（64 × uint32）だが、意味が異なるため両者を混在した類似度計算は無意味になる。

## マイグレーション

### バージョン管理

SQLite の `PRAGMA user_version` で elsa.db のデータマイグレーション世代を管理する。今回 v2 を導入するため値 2 を割り当てる。今後の wav_minhash 以外のデータマイグレーションでも 3, 4, … と通し番号で使う。

| user_version | 意味 |
|---|---|
| 0 | データマイグレーション未適用。v1 wav_minhash 残存の可能性あり、または新規 DB |
| 1 | 未使用・予約 |
| 2 | v2 wav_minhash 適用済み（本設計） |

### マイグレーション処理

`migrations.go::RunMigrations()` の末尾に追加:

```go
// wav_minhash アルゴリズム v2 への移行: 旧署名をクリアして再計算待ちにする
var userVersion int
_ = db.QueryRow(`PRAGMA user_version`).Scan(&userVersion)
if userVersion < 2 {
    if _, err := db.Exec(`UPDATE chart_meta SET wav_minhash = NULL WHERE wav_minhash IS NOT NULL`); err != nil {
        return fmt.Errorf("clear legacy wav_minhash: %w", err)
    }
    if _, err := db.Exec(`PRAGMA user_version = 2`); err != nil {
        return fmt.Errorf("bump user_version: %w", err)
    }
}
```

- 冪等。`user_version >= 2` なら何もしない
- クリア処理は SQLite の高速 UPDATE なので 17k 件で 1 秒未満
- マイグレーション後、起動シーケンスの `StartMinHashScan` が `ListChartsWithoutMinhash` で全件取得 → バックグラウンド再計算

### 起動時の利用者体験

- 旧クリア後の `StartMinHashScan` が全譜面の再計算を実行（17k 譜面で 4 分前後、100k 級なら 25 分前後）
- 進捗は既存の `wails runtime.EventsEmit("scan:progress", …)` でフロントエンドに反映
- 再計算中の `EstimateDiffInstall` / `ScanDuplicates` / `similarity.Score()` は部分結果になる。具体的には NULL 行は比較対象から除外され、新方式で計算済みの行のみ比較される
- 追加 UI は今回入れない。リリースノートに「初回起動時に WAV 類似度の再スキャンが走る」旨を記載

## コンポーネント変更

### `internal/domain/bms/parser.go`

`ParsedBMS` に `WAVRefCounts map[string]int` を追加。

```go
type ParsedBMS struct {
    MD5           string
    Title         string
    Subtitle      string
    Artist        string
    Subartist     string
    Genre         string
    WAVFiles      []string             // 既存互換のため残す
    WAVRefCounts  map[string]int       // 新規: basename -> 参照回数 (#RANDOM #IF 1 のみ)
}
```

パース処理は既存スキャナを拡張:

- `#WAVxx filename` 行: slot (2文字) → basename のマップを構築し、`WAVRefCounts[basename] = 0` で初期化
- データ行 `#nnnCC:DDDD...`: CC が WAV 参照チャンネルなら 2 文字ペアごとに slot を解決し、対応する basename の参照回数を +1。slot 定義が未到達の場合は `__slot:XX` キーで一時保持し、走査終了後に basename へ振り替える

検証ツール `cmd/validate-minhash/parser.go` で実装した方式をそのまま移植する。

既存 `WAVFiles` は変更せず、後方互換性のため公開フィールドとして残置する。`WAVRefCounts` のキー集合と一致するが、両方を参照するコードはないので二重保持の害は小さい。

### `internal/domain/bms/minhash.go`

`ComputeMinHash` のシグネチャを変更:

```go
// 旧
func ComputeMinHash(files []string) MinHashSignature

// 新
func ComputeMinHash(refCounts map[string]int) MinHashSignature
```

内部で `basename + 累積バケットタグ` の集合を構築し、現状の FNV-32a × 64 シードによる MinHash 計算ロジックに渡す。`Similarity`, `Bytes`, `MinHashFromBytes` は変更しない。

バケット閾値は `var bucketThresholds = []int{1, 2, 4, 8, 16, 32, 64}`。

### 呼び出し更新

| 呼び出し元 | 変更 |
|---|---|
| `internal/usecase/scan_minhash.go:64` | `bms.ComputeMinHash(parsed.WAVFiles)` → `bms.ComputeMinHash(parsed.WAVRefCounts)` |
| `internal/usecase/estimate_diff_install.go:90` | 同上 |

### `internal/adapter/persistence/migrations.go`

末尾にデータマイグレーションステップを追加。詳細は上記「マイグレーション処理」参照。

## テスト

### ユニットテスト

- **`parser_test.go`**:
  - 既存のテスト（`WAVFiles` の件数アサート）は引き続き通る
  - 新規: 小さなインライン BMS で `WAVRefCounts` の値が期待どおりになるケース（BGM 1 回, プレイチャンネル 3 回 など）
  - 新規: 地雷チャンネル `#xxxD1: XXXX` が **無視される** ことの確認
  - 新規: #RANDOM 内 `#IF 1` のみ集計対象になることの確認

- **`minhash_test.go`**:
  - 既存のテスト（`ComputeMinHash` の結果サイズ等）は新シグネチャに合わせて更新
  - 新規: 同じ basename 集合でも参照回数が大きく違えば類似度が下がることの確認
  - 新規: 同じ basename + 同じバケット帯なら類似度 100% になることの確認

- **`migrations_test.go`**（新規 or 既存に追記）:
  - `user_version=0` の DB + 既存 `wav_minhash` 入り行 → マイグレーション後に `wav_minhash = NULL` かつ `user_version = 2` になる
  - `user_version=2` の DB → 何も変わらない（冪等性）
  - クリア対象は `wav_minhash IS NOT NULL` の行だけで、他のカラムは保持される

### 統合検証

`cmd/validate-minhash/` を新アルゴリズム実装後に再ビルドして実 BMS で再検証。

## 検証 CLI の取り扱い

`cmd/validate-minhash/` は現状の検証用コードをそのまま残す（dev ツール）。本実装の `internal/domain/bms` を使うようにリファクタするかは別途検討。今後 v3 を試したくなった時に再利用できる。

## リリース運用

- リリースノート / CHANGELOG に「WAV 類似度判定アルゴリズムを改善。初回起動時に全譜面の再スキャンが走ります（17k 件で約 4 分）」と明記
- マイグレーション失敗時は `RunMigrations` 自体がエラーを返し、起動が継続しない（既存挙動）

## ロールバック

旧バイナリへのダウングレード時:

- 新 v2 で記録された `wav_minhash`（または NULL）は、旧 v1 から見ると単に「不一致」または「未計算」として扱われる
- 旧 v1 が起動すれば `ListChartsWithoutMinhash` が NULL 行を返し、旧アルゴリズムで再計算する
- `PRAGMA user_version = 2` は旧バイナリ側からは見えないが、SQLite として動作には影響しない

つまり旧バージョンに戻すと、自動的に旧アルゴリズムで再スキャンが走る。データ破壊はない。
