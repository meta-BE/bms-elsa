# BMSパーサー・MinHash・ノート数表示 設計

## Goal

BMSファイルからWAV定義を抽出しMinHashで類似度比較できるライブラリを作成する。
併せてsongdata.dbのノート数をUIに表示する。

## スコープ

### 今回作るもの

1. **BMSパーサー** — WAVファイル名集合の抽出
2. **MinHash計算** — WAV集合の類似度比較ライブラリ
3. **DBスキーマ** — `chart_meta`に`wav_minhash BLOB`カラム追加
4. **ノート数表示** — songdata.dbの`notes`カラムをChartモデル→DTO→フロントエンドまで通す

### 今回作らないもの

- フォルダ走査機能（MinHash計算・保存の実行はここで行う予定）
- 導入先推定へのMinHashスコアリング統合（走査後に追加）
- ノート数のBMSパース（songdata.dbから取得するため不要）

## 設計

### 1. BMSパーサー

**配置**: `internal/domain/bms/parser.go`

**パース対象**:
- `#WAVxx <filename>` — ファイル名を収集（番号は無視、ファイル名の集合のみ保持）
- `#RANDOM N` / `#IF 1` / `#ENDIF` / `#ENDRANDOM` — RANDOM内は`#IF 1`のブロックのみ処理

**パースしないもの**: ノート数、BMP定義、データ部、ヘッダメタ情報

**出力**:
```go
type ParseResult struct {
    WAVFiles []string // ユニークなWAVファイル名のソート済みリスト（拡張子除去済み）
}
```

**拡張子の正規化**: BMSでは`.wav`と記述されていても実ファイルが`.ogg`のケースが多い。拡張子を除去したベース名で集合を構築する。

### 2. MinHash計算

**配置**: `internal/domain/bms/minhash.go`

**パラメータ**:
- K = 64（署名サイズ: 256バイト）
- ハッシュ関数: FNV-1aベースで64個のシード値を使用

**入力**: パーサーが返すWAVファイル名集合

**出力**:
```go
type MinHashSignature [64]uint32

func (s MinHashSignature) Similarity(other MinHashSignature) float64
```

**DB保存形式**: `chart_meta`テーブルに`wav_minhash BLOB`カラム追加（256バイト固定長）。フォルダ走査機能の実装時にここへ保存する。

**WAV定義比較の背景**:
- 全ファイル名をDBに保存すると1譜面あたり数百件で容量の無駄
- 全体ハッシュでは差分譜面がWAVを追加した場合に不一致になる
- MinHashなら固定256バイトで集合の類似度を近似推定でき、WAV追加による部分集合関係でもJaccard 0.8以上の類似度が得られる

### 3. ノート数表示

**バックエンド**:
- `Chart`モデルに`Notes int`フィールド追加
- `songdata_reader.go`のSQLクエリに`notes`カラム追加
- `ChartDTO`に`Notes int`追加

**フロントエンド**:
- 譜面一覧テーブルに`NOTES`列追加（ソート可能）
- ChartInfoCard（譜面詳細）にノート数表示

## テスト戦略

**testdata**: 3つの実BMSファイルを使用

| ファイル | 特徴 |
|---------|------|
| Dstorv [Ego] | LNOBJ方式、地雷あり、WAV 631件 |
| Dstorv [false_fix] | LNOBJ方式、WAV 630件（Egoとほぼ同一セット）|
| Random [SP ANOTHER] | LNTYPE方式、RANDOM 23ブロック、WAV 6427件 |

**BMSパーサーテスト**:
- 各ファイルのWAV定義数が期待値と一致
- RANDOM内の`#IF 1`のみが処理される

**MinHashテスト**:
- Dstorv [Ego] vs [false_fix] → 高類似度（0.9以上）
- Dstorv vs Random → 低類似度（0.1以下）

**ノート数テスト**:
- songdata_reader_testでnotesカラムが正しく取得される
