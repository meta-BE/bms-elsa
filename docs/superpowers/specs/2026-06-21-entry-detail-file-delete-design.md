# 難易度表詳細 ファイルパス一覧へのファイル削除機能 設計

## 背景

直前の作業で、難易度表エントリ詳細ビュー（`EntryDetail.svelte`）の重複ステータス時に、同一MD5を持つ全BMSファイルのパス一覧（「ファイルパス一覧」セクション）を表示するようにした。

このパス一覧の各行から、対象のBMSファイルを削除できるようにする。重複ファイルの整理を詳細ビュー上で完結させることが目的。

## 要件

- 各パス行の**右端にゴミ箱アイコン**を表示し、クリックで削除を開始する。
- 削除前に**確認**を行う。
- 削除対象は**パスが指す単一のBMSファイル**（`.bms`/`.bme`/`.bml` 等）。同じフォルダ内のWAV・BGA・他難易度の譜面は削除しない。
- 削除方式は**完全削除（`os.Remove`）**。OSのゴミ箱には入れず、復元不可。songdata等のDBは更新しない。
- **削除後のUI更新は不要**（一覧からの除去・再読込は行わない）。
- 削除しようとした**ファイルが既に存在しない場合は何もしない**（エラーにしない）。
- 上記以外の削除失敗（権限エラー等）は**AlertModalで通知**する。

## アーキテクチャ

### バックエンド（Go）

`internal/app/chart_handler.go` に新メソッドを追加する。パス一覧を返す `ListChartPathsByMD5` と同じハンドラに置き、一覧取得と削除を同居させる。

```go
// DeleteChartFile は指定パスのBMSファイルを削除する。
// ファイルが既に存在しない場合は何もせず成功扱いとする。
func (h *ChartHandler) DeleteChartFile(path string) error {
    if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
        return err
    }
    return nil
}
```

- `os.Remove` で完全削除。
- `os.IsNotExist(err)` の場合は `nil` を返す（要件「既に無ければ何もしない」）。
- それ以外のエラーはそのまま呼び出し元へ返す。
- `os` パッケージの import を追加する。
- Wails バインディングを再生成し、`frontend/wailsjs/go/app/ChartHandler` に `DeleteChartFile` を公開する。

### フロントエンド（`frontend/src/views/EntryDetail.svelte`）

#### パス行のレイアウト

各パス行の右端にゴミ箱ボタンを追加する。

```
[📁フォルダ]  パス文字列 .........................  [🗑️]
```

```svelte
<div class="text-xs text-base-content/70 flex items-center gap-1">
  <OpenFolderButton path={p} size="xs" title="フォルダを開く" />
  <span class="flex-1 break-all">{p}</span>
  <button
    class="btn btn-ghost btn-xs shrink-0"
    title="ファイルを削除"
    on:click={() => requestDelete(p)}
  >
    <Icon name="trash" cls="h-3 w-3" />
  </button>
</div>
```

- `trash` アイコンは `icons.ts` に既存。
- パス文字列を `flex-1 break-all` にしてゴミ箱を右端に寄せる。

#### 削除フロー

1. ゴミ箱クリック → `requestDelete(p)` が削除対象パスを状態に保持し、確認ダイアログを `showModal()` で開く。
2. 確認ダイアログ（`<dialog class="modal">`、`DuplicateDetail.svelte` の confirmDialog と同じパターン）に対象パスを表示し、「削除」「キャンセル」ボタンを置く。
3. 「削除」→ `DeleteChartFile(path)` を呼び、ダイアログを閉じる。**一覧の更新・再読込は行わない。**
4. ファイルが既に無い場合はバックエンドが `nil` を返すため何も起きない。
5. `DeleteChartFile` が例外を投げた場合（権限エラー等）は `AlertModal` でエラーメッセージを表示する。

#### import 追加

- `DeleteChartFile` を `../../wailsjs/go/app/ChartHandler` から import。
- `AlertModal` コンポーネントを import し、`alertModal` 参照を保持。

## エラーハンドリング

| ケース | 振る舞い |
|--------|----------|
| ファイルが既に存在しない | 何もしない（バックエンドが `nil`） |
| 権限エラー等の削除失敗 | AlertModal でエラー通知 |
| 確認ダイアログでキャンセル | 何もしない |

## テスト方針

- バックエンド: `DeleteChartFile` の単体テスト。
  - 存在するファイルを削除できること。
  - 存在しないパスでもエラーを返さないこと（`nil`）。
  - （任意）削除不可ケースでエラーを返すこと。
- フロントエンド: 手動確認。重複エントリ詳細でゴミ箱→確認→削除が動作し、確認キャンセルで削除されないこと、削除後に一覧が更新されないこと。

## 変更ファイル

- `internal/app/chart_handler.go` — `DeleteChartFile` 追加（`os` import 追加）
- `internal/app/chart_handler_test.go`（または既存テストファイル） — `DeleteChartFile` のテスト
- `frontend/wailsjs/go/app/ChartHandler.*` — バインディング再生成
- `frontend/src/views/EntryDetail.svelte` — ゴミ箱ボタン・確認ダイアログ・AlertModal・削除処理
- `docs/manual.md` — 機能説明の追記

## スコープ外（YAGNI）

- OSゴミ箱への移動（サードパーティライブラリ追加）。
- 削除後の一覧自動更新・再読込。
- DB（songdata）からのレコード削除。
- フォルダ単位の削除。
