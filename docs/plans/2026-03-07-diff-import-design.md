# 新規差分導入画面 設計

## Goal

BMS/BME/BMLファイルをドラッグ＆ドロップし、既存楽曲パッケージへの導入先を自動推定してファイルを移動する画面を作成する。

## 前提

- 差分ファイル（BMS単体）の導入が目的。楽曲フォルダごとの導入は想定しない
- beatorajaが次回起動時にスキャンするため、songdata.dbへの登録は不要
- 移動対象はBMSファイル単体（WAV等の関連ファイルは移動しない）

## BMSパーサー拡張

### 新規関数

`ParseBMSFile(path string) (*ParsedBMS, error)` を新設。

```go
type ParsedBMS struct {
    MD5        string   // ファイル全体のMD5ハッシュ
    Title      string   // #TITLE
    Subtitle   string   // #SUBTITLE
    Artist     string   // #ARTIST
    Subartist  string   // #SUBARTIST
    Genre      string   // #GENRE
    WAVFiles   []string // WAV定義リスト
}
```

- 全項目で `#RANDOM`〜`#ENDRANDOM` / `#IF 1` ルールを適用（testdata/[Clue]Random にヘッダーがRANDOM変化する例あり）
- MD5はファイル全体のバイナリから計算
- 既存の `ParseWAVFiles` は削除し、呼び出し元を `ParseBMSFile` + `.WAVFiles` に置換

## 導入先推定ロジック

### 統一スコア方式

複数の推定手段の結果をフォルダ単位でマージし、統合スコアで最上位候補を選択する。

**スコア算出:**
- MinHashスコア = MinHash類似度（0.0〜1.0）× 10（最大10点）
- メタデータスコア = EstimateInstallLocationUseCaseのスコア（title=3, base_title=2, body_url=3, artist=1）
- 統合スコア = MinHashスコア + メタデータスコア

**推定フロー:**
1. WAV MinHash類似度検索を実行 → フォルダごとにMinHashスコアを付与
2. MinHashスコア ≥ 8.0（類似度0.8以上）→ IR問い合わせスキップ、最上位候補を採用
3. MinHashスコア < 8.0 → LR2IRにMD5で問い合わせ（IR登録があれば情報を保存）
4. IR情報またはパースしたTITLE/ARTISTで EstimateInstallLocationUseCase を実行 → メタデータスコアを取得
5. 同一フォルダに複数手段がヒットした場合はスコアを加算
6. 統合スコア最上位のフォルダを推定先とする

### MinHash類似度検索の実装

実装前にベンチマークを行い、方式を決定する。

- **A. SQLiteカスタム関数**: `minhash_similarity(?, wav_minhash)` を `modernc.org/sqlite` の `RegisterDeterministicScalarFunction` で登録。SQL内で比較・ソート
- **B. Go全件スキャン**: minhash値ベースでユニーク化した全件（約3,000件）をGoに読み込み、ループで比較

ベンチマーク方法:
- Go標準の `testing.B` で実際のelsa.dbデータを使用
- パフォーマンス差が大きければ速い方、差が小さければシンプルな方を採用

### 結果の構造体

```go
type ImportCandidate struct {
    FilePath    string     // ドロップされたファイルのパス
    Parsed      *ParsedBMS
    DestFolder  string     // 推定先フォルダ（空なら未推定）
    Score       float64    // 統合スコア
    MatchMethod string     // 最もスコアに寄与した手段: "minhash" / "ir" / "title"
}
```

## フロントエンドUI

### 画面配置

5つ目のタブ「差分導入」として追加。

- テーブル領域全体がドロップゾーンを兼ねる
- 件数0のとき: プレースホルダーテキスト（「BMS/BME/BMLファイルをドラッグ＆ドロップして差分を追加」等）を表示
- 下部: 「推定先に導入」ボタン

### テーブルカラム

| カラム | 内容 |
|--------|------|
| ファイル名 | ドロップしたファイルのベース名 |
| TITLE | TITLE + " " + SUBTITLE（SUBTITLEがある場合） |
| ARTIST | ARTIST + " " + SUBARTIST（SUBARTISTがある場合） |
| 推定先 | 推定フォルダパス（なければ「未推定」表示） |
| スコア | 統合スコア |
| 推定方法 | minhash / ir / title |
| 操作 | 「クリア」ボタン（推定先を除外） |

### 動作フロー

1. ファイルをテーブル領域にD&D → バックエンドでパース＋推定を実行（進捗表示）
2. 結果がテーブルに表示される
3. ユーザーが不要な行の「クリア」ボタンで推定先を除外
4. 「推定先に導入」ボタン → 推定先があるファイルのみ移動実行
5. 移動完了後、結果を表示（成功数・失敗数）

### D&D時のフォルダ対応

フォルダがドロップされた場合、内部のBMS/BME/BMLファイルを再帰的に収集。

## バックエンドAPI

### ハンドラー層

新規 `DiffImportHandler` を作成。

- `ParseAndEstimate(filePaths []string) []ImportCandidate` — D&D時に呼ばれる。パース→推定を一括実行。IR問い合わせを含むため進捗イベント(`diff-import:progress`)を発信
- `ExecuteImport(candidates []ImportCandidateRequest) ImportResult` — 「推定先に導入」ボタンで呼ばれる。確定済み候補のファイル移動を実行

### ユースケース層

- `EstimateDiffInstallUseCase` — MinHash＋メタデータの統一スコア方式で推定を統括
- `ExecuteDiffImportUseCase` — ファイル移動を実行

### ファイル移動ユーティリティ

汎用の「ファイルをフォルダに移動する」関数を作成。

```go
// internal/domain/fileutil/move.go
func MoveFileToFolder(srcPath, destFolder string) error
```

- `os.Rename` を使用（同一ファイルシステム内の移動）
- 移動先に同名ファイルが存在する場合はエラーを返す（上書きしない）
- 移動先フォルダが存在しない場合もエラー
