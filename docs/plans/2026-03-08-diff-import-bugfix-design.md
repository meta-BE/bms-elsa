# 差分導入バグ修正 設計

## バグ1: クロスドライブ移動の失敗

### 現象
- `C:\Users\meta\Downloads\...` → `G:\BMS\SONGS\...` へのファイル移動が失敗
- エラー: `The system cannot move the file to a different disk drive.`

### 原因
- `internal/domain/fileutil/move.go` の `MoveFileToFolder` が `os.Rename` を使用
- `os.Rename` はドライブをまたぐ移動ができない（WindowsのMoveFile API制約）

### 修正
- `os.Rename` を `io.Copy` + `os.Remove` に置き換え
- 処理フロー:
  1. 移動先フォルダの存在確認（既存）
  2. 移動先の同名ファイル重複チェック（既存）
  3. srcをopenし、destを作成して `io.Copy` でコピー
  4. dest を `Close` してエラーチェック
  5. コピー成功後にsrcを `os.Remove` で削除
  6. コピー途中のエラー時はdestファイルを削除してクリーンアップ

### 注意点
- `os.Remove`（src削除）は `io.Copy` 完了後にのみ実行すること
- コピー失敗時はdest側のゴミファイルを確実にクリーンアップすること

### 対象ファイル
- `internal/domain/fileutil/move.go`
- `internal/domain/fileutil/move_test.go`（既存テストはそのまま通るはず）

---

## バグ2: BMSパーサーの文字化け

### 現象
- 差分導入画面のTITLE/ARTISTカラムに文字化け（◆◆◆）が表示される
- Shift-JISエンコードのBMSファイルが対象

### 原因
- `internal/domain/bms/parser.go` の `ParseBMSFile` が `os.ReadFile` で読んだバイト列をそのままUTF-8として処理
- BMSファイルの事実上の標準エンコーディングはShift-JIS（CP932）

### 修正
- `os.ReadFile` 後にUTF-8バリデーションを行い、非UTF-8ならShift-JIS → UTF-8に変換
- `golang.org/x/text/encoding/japanese` を使用（既にgo.modに含まれている）

### 対象ファイル
- `internal/domain/bms/parser.go`
- `internal/domain/bms/parser_test.go`（Shift-JISファイルのテストケース追加が望ましい）
