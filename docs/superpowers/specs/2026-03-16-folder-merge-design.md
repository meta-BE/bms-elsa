# フォルダマージ機能 設計書

## 概要

重複検知画面から、あるBMSフォルダを別のフォルダにマージ（統合）する機能を実装する。
合わせて、ファイル移動操作のログ出力基盤を整備する。

## 方針

- ファイルシステム操作のみ。DB（elsa.db / songdata.db）は更新しない
- beatoraja側の再スキャンで自然にDBが反映される想定
- クリーンアーキテクチャに従い、usecase層でオーケストレーション・ログ書き込みを行う

## 1. ファイルシステム操作層

### `internal/domain/fileutil/merge.go`（新規）

既存の `fileutil/move.go` と同じパッケージに配置する。
fileutil はファイル操作のユーティリティ群として domain 層に存在しており、
既存の `MoveFileToFolder` と同様の位置づけとする。

```go
type MergeResult struct {
    Moved    []string     // 新規移動したファイル（相対パス）
    Replaced []string     // 上書きしたファイル（移動元が新しい）
    Skipped  []string     // スキップしたファイル（移動先が新しいor同一）
    Errors   []MergeError
}

type MergeError struct {
    FileName string
    Err      error
}

// MergeFolders は srcDir 内の全ファイルを destDir に移動し、
// 成功後に srcDir を削除する。
// - サブディレクトリも再帰的に処理（相対パス構造を維持）
// - 競合時はファイルの作成日時を比較し、新しい方を残す
// - 作成日時が同一の場合はスキップ
// - 1ファイルでもエラーがあった場合、srcDirの削除はスキップ
//   （一部移動済みの状態で再実行しても競合解決ルールにより冪等に動作する）
// - srcDir が空の場合は即座に削除して空の MergeResult を返す
// - シンボリックリンクは無視する（filepath.Walk のデフォルト挙動）
func MergeFolders(srcDir, destDir string) (*MergeResult, error)
```

事前バリデーション（`MergeFolders` 内で実行、違反時は即エラー返却）:
- srcDir と destDir が同一でないこと
- srcDir が destDir のサブディレクトリでないこと（またはその逆）
- srcDir と destDir がともに存在すること

ファイル移動は既存の `copyFile` + `os.Remove` パターンを再利用する
（`os.Rename` はクロスドライブで失敗するため）。

### 競合解決ルール

| 移動先にファイルが… | 動作 |
|---|---|
| 存在しない | そのまま移動（`+`） |
| 存在し、移動元の方が新しい | 移動先を上書き（`>`） |
| 存在し、移動先の方が新しい or 同一 | スキップ（`=`） |

### 作成日時の取得

ファイルの作成日時取得はプラットフォーム依存のため、ビルドタグで分離する。

`internal/domain/fileutil/ctime_windows.go`:
```go
//go:build windows

// fileCreationTime は Windows の CreationTime を返す
func fileCreationTime(path string) (time.Time, error)
// syscall.Win32FileAttributeData.CreationTime を使用
```

`internal/domain/fileutil/ctime_other.go`:
```go
//go:build !windows

// fileCreationTime は非Windowsでは ModTime にフォールバックする
func fileCreationTime(path string) (time.Time, error)
// os.Stat().ModTime() を使用
```

## 2. ロガー

### `internal/adapter/logger/logger.go`（新規）

既存の adapter 層（gateway, persistence）と並列に配置する。

```go
// port層にインターフェースを定義
// internal/port/logger.go
type Logger interface {
    Log(message string)
}

// adapter層に実装
// internal/adapter/logger/logger.go
type FileLogger struct {
    file *os.File
    mu   sync.Mutex
}

// New は実行ファイルと同じディレクトリに system.log を開く（追記モード）
func New() (*FileLogger, error)

// Close はログファイルを閉じる
func (l *FileLogger) Close() error

// Log は1行のログを書き込む（タイムスタンプ自動付与）
// 形式: "2026-03-16 15:00:00 <message>"
func (l *FileLogger) Log(message string)
```

- 汎用的な文字列書き込みのみ。フォーマットの責任はUseCase層
- ログ書き込み失敗は握りつぶす（マージ操作の成否に影響を与えない）
- ログローテーションは初期実装では不要（追記のみ）
- UseCase層は `port.Logger` インターフェースに依存する

## 3. コンフィグ

### `app.go` の既存 `Config` 構造体に追加

```go
type Config struct {
    SongdataDBPath string `json:"songdataDBPath"`
    FileLog        bool   `json:"fileLog"`        // ファイル別ログのオン/オフ（デフォルトfalse）
}
```

- `config.json` に `"fileLog": true` を設定すればファイル別ログが有効
- 未指定時は `false`（サマリー行のみ出力）
- 既存の `GetConfig` / `SaveConfig` をそのまま利用

## 4. ユースケース層

### `internal/usecase/merge_folders.go`（新規）

```go
type MergeFoldersUseCase struct {
    logger  port.Logger
    fileLog bool
}

func NewMergeFoldersUseCase(logger port.Logger, fileLog bool) *MergeFoldersUseCase

func (u *MergeFoldersUseCase) Execute(ctx context.Context, srcDir, destDir string) (*fileutil.MergeResult, error)
```

**ログフォーマット（UseCase側で組み立て）:**

fileLog=true の場合:
```
2026-03-16 15:00:00 MERGE C:\bms\src_folder → C:\bms\dest_folder
  + file1.bms
  > sub/file2.wav
  = file3.bms
  ! file4.bms: permission denied
```

fileLog=false の場合:
```
2026-03-16 15:00:00 MERGE C:\bms\src_folder → C:\bms\dest_folder (moved:10, replaced:3, skipped:2, errors:0)
```

### `internal/usecase/execute_diff_import.go`（既存修正）

Logger を注入可能にする。`Execute` メソッドのシグネチャ（引数・戻り値）は変更しない。
内部でログ出力を追加するのみ。

```go
type ExecuteDiffImportUseCase struct {
    logger  port.Logger  // 追加
    fileLog bool         // 追加
}

func NewExecuteDiffImportUseCase(logger port.Logger, fileLog bool) *ExecuteDiffImportUseCase
// Execute のシグネチャは既存のまま変更なし
```

fileLog=true の場合:
```
2026-03-16 15:01:00 MOVE file.bms → C:\bms\dest_folder
```

fileLog=false の場合:
```
2026-03-16 15:01:00 IMPORT 5 files → C:\bms\dest_folder (success:5, failed:0)
```

## 5. ハンドラー層・DI配線

### `internal/app/duplicate_handler.go`（既存修正）

```go
type DuplicateHandler struct {
    ctx            context.Context
    scanDuplicates *usecase.ScanDuplicatesUseCase
    mergeFolders   *usecase.MergeFoldersUseCase  // 追加
}

type MergeFoldersResultDTO struct {
    Success  bool
    Moved    int
    Replaced int
    Skipped  int
    Errors   int
    ErrorMsg string // エラーがあれば最初のエラーメッセージ
}

// MergeFolders は srcDir を destDir にマージする
func (h *DuplicateHandler) MergeFolders(srcDir, destDir string) (*MergeFoldersResultDTO, error)
```

### `app.go` の変更

App 構造体に logger フィールドを追加:
```go
type App struct {
    // 既存フィールド...
    logger *logger.FileLogger  // 追加: shutdown で Close するため具象型を保持
}
```

DI配線:
```go
func (a *App) Init() {
    // 既存のDB初期化...

    // Logger の生成
    a.logger, err = logger.New()
    // ...
    cfg := loadConfig()

    // MergeFoldersUseCase の組み立て
    mergeFoldersUC := usecase.NewMergeFoldersUseCase(a.logger, cfg.FileLog)

    // DuplicateHandler に注入
    a.duplicateHandler = NewDuplicateHandler(scanDupUC, mergeFoldersUC)

    // ExecuteDiffImportUseCase にも Logger 注入
    a.diffImportHandler = NewDiffImportHandler(
        usecase.NewExecuteDiffImportUseCase(a.logger, cfg.FileLog),
        // ...
    )
}

func (a *App) shutdown(ctx context.Context) {
    a.db.Close()     // 既存
    a.logger.Close() // 追加
}
```

## 6. フロントエンド

### `DuplicateDetail.svelte`（既存修正）

**操作フロー:**

1. メンバー一覧に「マージ先に指定」ボタンを追加
2. マージ先が選択されると、他のメンバーに「→ マージ」ボタンが表示される
3. 「→ マージ」クリック → 確認ダイアログ表示
   - 「フォルダXをフォルダYにマージします。移動元は削除されます。よろしいですか？」
4. 確認後、バックエンドの `MergeFolders` API 呼び出し（ローディング表示）
5. 成功 → メンバーを一覧から楽観的に除去、結果をトースト表示
6. 部分エラー（Errors > 0）→ 警告トースト表示（エラーメッセージ含む）、メンバーは除去しない
7. メンバーが1つ以下 → グループ一覧からも除去

## スコープ外

- DB更新（elsa.db / songdata.db）— beatoraja再スキャンで対応
- ログローテーション
- 一括マージ（複数フォルダを一度にマージ）
- マージ中のキャンセル操作（初期実装では同期処理）
