# 楽曲フォルダ移動 設計

## 概要

楽曲一覧画面の詳細パネルから、楽曲フォルダを別のディレクトリに移動する機能。移動先は空ディレクトリ（新規作成）であるため、既存のマージ機能は再利用せず、シンプルな移動操作として実装する。

## 要件

- 楽曲詳細パネルから「フォルダ移動」ボタンで移動を実行できる
- 移動先はOSネイティブのディレクトリ選択ダイアログで指定する
- 移動先ディレクトリ配下に楽曲フォルダ名でフォルダを作成し、全ファイルを移動する
- 移動前に確認ダイアログを表示する（不可逆操作のため）
- 移動完了後に結果ダイアログ（移動先パス、ファイル数）を表示する
- songdata.dbは編集しない（beatorajaの再スキャンはユーザー手動）
- UI上は移動済みの楽曲をセッション中のみ黄色背景＋「移動済み」バッジで表示する

## バックエンド設計

### ドメイン層: `fileutil.MoveFolder`

**ファイル**: `internal/domain/fileutil/move.go`

```go
// MoveFolder は srcDir を destDir に移動する。
// destDir は移動先の完全パス（存在してはならない）。
// 既存の move.go 内の copyFile を再利用する。
func MoveFolder(srcDir, destDir string) (fileCount int, err error)
```

- バリデーション: srcDir の存在確認、destDir の非存在確認
- `os.Rename(srcDir, destDir)` を試行（同一ファイルシステムならアトミック完了）
  - rename成功時のファイル数は事前に `os.ReadDir` でカウント
- `EXDEV` エラー（クロスファイルシステム）の場合、再帰コピー＋`os.RemoveAll(srcDir)` にフォールバック
  - コピー途中で失敗した場合は `os.RemoveAll(destDir)` でクリーンアップし、srcDir は残す
- 戻り値: 移動したファイル数とエラー

### ユースケース層: `MoveSongFolderUseCase`

**ファイル**: `internal/usecase/move_song_folder.go`

```go
type MoveSongFolderUseCase struct {
    logger port.Logger
}

func (u *MoveSongFolderUseCase) Execute(ctx context.Context, srcFolderPath, destParentDir string) (destPath string, err error)
```

- `filepath.Base(srcFolderPath)` でフォルダ名を取得
- `destParentDir/フォルダ名` を移動先パスとして構築
- `fileutil.MoveFolder` を呼び出し
- ログ出力: `MOVE srcFolderPath → destPath`
- 戻り値: 移動先の完全パス

### ハンドラー層: `SongHandler.MoveSongFolder`

**ファイル**: `internal/app/song_handler.go` に追加

```go
func (h *SongHandler) MoveSongFolder(folderHash, destParentDir string) (*MoveSongFolderResultDTO, error)
```

- `folderHash` から songdata.db を参照して代表チャートの `path` を取得し、`parentDirOf` でフォルダパスを導出
- `MoveSongFolderUseCase.Execute` を呼び出し
- 戻り値DTO（`dto/dto.go` に定義）: 移動先パス、移動ファイル数

### ダイアログAPI: `App.SelectDirectory`

**ファイル**: `app.go` に追加

```go
func (a *App) SelectDirectory() (string, error)
```

- `wailsRuntime.OpenDirectoryDialog` を使用
- 既存の `SelectFile()` と並列配置

## フロントエンド設計

### SongDetail.svelte — 移動ボタン＋ダイアログ

- 「フォルダを開く」ボタンの隣に「フォルダ移動」ボタンを追加
- フロー:
  1. ボタンクリック → `SelectDirectory()` でディレクトリ選択（キャンセル時＝空文字列は処理中断）
  2. 確認ダイアログ表示:「[フォルダ名] を [移動先パス] に移動しますか？移動元フォルダは削除されます。」
  3. 実行 → `MoveSongFolder(folderHash, destParentDir)` 呼び出し
  4. 成功時: 結果ダイアログ（移動先パス、ファイル数）を表示
  5. 失敗時: エラーダイアログ（エラーメッセージ）を表示
  6. 結果ダイアログを閉じたら `dispatch('moved', { folderHash })` で親に通知＋詳細パネルを閉じる
- 移動済みの楽曲（`movedHashes` に含まれる）では「フォルダ移動」ボタンを無効化（移動後はsongdata.dbのパスが古いため再移動不可）

### App.svelte — 移動済み状態管理

- `movedFolderHashes: Set<string>` をセッション中のみ保持（DB保存なし）
- `moved` イベント受信時に folderHash を Set に追加＋選択解除

### SongTable.svelte — 移動済み表示

- `movedHashes: Set<string>` を props で受け取る
- 該当行に黄色背景（`bg-warning/20` 系）＋「移動済み」バッジを表示
- 移動済み行はクリック可能（詳細パネルで移動先情報を確認できる）

## 操作フロー

```
楽曲詳細「フォルダ移動」ボタン
  → ディレクトリ選択ダイアログ（OS ネイティブ）
  → 確認ダイアログ「[フォルダ名] を [移動先] に移動しますか？」
  → 移動実行（rename優先、EXDEV時コピー＋削除）
  → 結果ダイアログ（移動先パス、ファイル数）
  → ダイアログ閉じる → 楽曲一覧で黄色「移動済み」表示、詳細パネル閉じる
```

## スコープ外

- songdata.db の更新（beatoraja側の責務）
- 移動履歴の永続化（セッション中のUI表示のみ）
- 移動の取り消し機能
