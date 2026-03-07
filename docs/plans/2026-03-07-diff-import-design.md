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

### 推定順序（優先度順）

1. **WAV MinHash類似度検索**: パースしたWAV定義からMinHash署名を計算し、elsa.dbの既存chart_metaと比較。最も類似度の高いレコードのフォルダパスを候補とする（閾値以上の場合）
2. **LR2IR問い合わせ**: MD5でLR2IRに問い合わせ。登録があれば既存の `EstimateInstallLocationUseCase` にtitle/artist/bodyURLを渡して推定
3. **タイトル・アーティスト一致**: パースしたTITLE/ARTISTで `EstimateInstallLocationUseCase` に渡して推定

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
    MatchMethod string     // "minhash" / "ir" / "title"
    Similarity  float64    // MinHash類似度（minhashの場合）
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

- `EstimateDiffInstallUseCase` — MinHash→IR→タイトルの3段階推定を統括
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
