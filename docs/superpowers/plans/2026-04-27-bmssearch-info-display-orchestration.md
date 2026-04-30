# BMS Search 情報表示機能 — 実装オーケストレーション

このドキュメントは **subagent-driven-development の controller セッション** が読むためのインデックス兼ガイド。
実装計画本体（plan）は長大（3500行超）なため、controller は **このドキュメントだけを最初に通読** し、各タスクを subagent に依頼する際に plan の該当行範囲だけを Read して subagent に渡す。

---

## 関連ドキュメント

| 種類 | パス |
|---|---|
| 設計（spec） | `docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md` |
| 実装計画（plan）本体 | `docs/superpowers/plans/2026-04-27-bmssearch-info-display.md` |
| 事前調査レポート（Task 1 で生成） | `docs/superpowers/specs/2026-04-27-bmssearch-info-fallback-probe.md` |
| 事前調査の生 JSON（Task 1 で生成） | `docs/superpowers/specs/data/bmssearch-probe/2026-04-27/` |
| プロジェクト規約 | `CLAUDE.md`, `docs/style-guide.md`, `docs/manual.md`, `README.md` |

---

## このドキュメントの使い方（controller 向け）

### 全体フロー

1. このドキュメントを通読し、Task 一覧と共通コンテキストを把握する
2. plan 本体は **Read しない**（必要な範囲だけを Task 着手時に Read する）
3. TodoWrite で全 Task をリスト化
4. Task 1 から順に下記サイクルを回す:
   - **(a) implementer subagent 起動** → 「## Subagent 起動テンプレート — implementer」を使う
   - **(b) spec reviewer subagent 起動** → spec 該当セクションを渡して整合確認
   - **(c) code quality reviewer subagent 起動** → コミット SHA を渡して品質レビュー
   - 不備があれば implementer に修正依頼 → 該当 reviewer で再レビュー
5. すべての Task 完了後、ブランチ全体に対する最終 code reviewer を起動
6. `superpowers:finishing-a-development-branch` に引き継ぐ

### subagent への情報の渡し方

implementer に渡すパケットは固定構造:

1. **共通コンテキスト**（このドキュメントの「## 共通コンテキスト」セクションをコピー）
2. **Task 個別の前提・補足**（このドキュメントの該当 Task のメタ情報をコピー）
3. **plan 該当行の生テキスト**（plan を `Read(file_path, offset, limit)` で取得して貼り付け）
4. **要請ステータス**（DONE / DONE_WITH_CONCERNS / NEEDS_CONTEXT / BLOCKED で報告するよう明記）

subagent には plan 本体のパスを教えない。Read を許す範囲は実装ファイル・テストファイル・spec のみ。

### モデル選択の指針

| Task の特徴 | モデル目安 |
|---|---|
| 1〜2ファイル、純粋関数や単純な CRUD | cheap (haiku 系) |
| 複数ファイル横断・既存コードとの統合 | standard (sonnet 系) |
| 設計判断・広範な調査・最終レビュー | capable (opus 系) |

各 Task の推奨モデルは「## タスク目次」を参照。

---

## タスク目次

行範囲は plan 本体（`docs/superpowers/plans/2026-04-27-bmssearch-info-display.md`）の行番号。

| # | タイトル | 行範囲 | 依存 | モデル | 種別 |
|---|---|---|---|---|---|
| 1 | 事前調査スパイク（フォールバック検索仕様確定） | L55–L319 | なし | standard | Go cmd + 調査レポート |
| 2 | マイグレーション追加 | L325–L464 | なし | cheap | Go (persistence) |
| 3 | ドメインエンティティ追加 | L468–L573 | – | cheap | Go (model) |
| 4 | ElsaRepository 拡張（bms_search_source 対応） | L577–L736 | T3 | standard | Go (persistence) |
| 5 | BMSSearchRepository 実装 | L740–L1104 | T2, T3 | standard | Go (persistence) |
| 6 | BMSSearchClient 拡張（SearchBMSesByTitle） | L1110–L1281 | なし | cheap | Go (gateway) |
| 7 | 正規化・スコアリング pure functions | L1287–L1614 | T1 結果 | standard | Go (usecase) |
| 8 | BMSSearchResolver 実装 | L1618–L2088 | T3, T5, T6, T7 | capable | Go (usecase) |
| 9 | LookupBMSSearchUseCase 実装 | L2092–L2320 | T8, T13 | standard | Go (usecase) |
| 10 | UnlinkBMSSearchUseCase 実装 | L2323–L2476 | T5, T13 | cheap | Go (usecase) |
| 11 | SyncBMSSearchUseCase 改修 | L2479–L2649 | T8 | standard | Go (usecase + handler) |
| 12 | DTO 追加 + Phase 4 まとめコミット | L2654–L2747 | T9, T10, T11 | cheap | Go (dto) |
| 13 | SongdataReader 拡張（FolderResolver 系） | L2750–L2830 | なし | standard | Go (persistence) |
| 14 | BMSSearchHandler + DI 組み立て | L2833–L3004 | T9, T10, T12, T13 | standard | Go (app + main + Wails 再生成) |
| 15 | search アイコン追加 | L3009–L3038 | なし | cheap | TS |
| 16 | BMSSearchInfoCard.svelte 実装 | L3042–L3197 | T12 (DTO型生成), T14 (Handler型生成), T15 | standard | Svelte |
| 17 | SongDetail への配置 | L3201–L3286 | T14, T16 | standard | Svelte |
| 18 | ChartDetail への配置 | L3289–L3365 | T14, T16 | standard | Svelte |
| 19 | EntryDetail への配置 | L3368–L3468 | T14, T16 | standard | Svelte |
| 20 | マニュアル更新 | L3473–L3515 | 機能完成 | cheap | Markdown |
| 21 | 全体ビルド・テスト・手動 QA | L3518–L3563 | 全タスク | standard | 検証のみ |

### 推奨実行順序

依存関係を踏まえた直列実行順:

```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T13 → T9 → T10 → T11 → T12 → T14 → T15 → T16 → T17 → T18 → T19 → T20 → T21
```

T13（SongdataReader 拡張）は plan 本体では Phase 5 に置かれているが、T9 と T10 が依存するため **T8 直後に前倒し** したほうが安全。controller は TodoWrite に上記順で登録すること。

### コミット単位の注意

plan 内で以下の Task は単独コミットせず、まとめてコミットする指示が出ている。implementer にも明示すること:

- **T3 + T4** → T4 完了時にまとめて 1 コミット
- **T9 + T10 + T11 + T12** → T12 完了時にまとめて 1 コミット
- **T13 + T14** → T14 完了時にまとめて 1 コミット
- **T17 + T18 + T19** → T19 完了時にまとめて 1 コミット

各 reviewer はまとめコミット後の SHA を対象にする。途中段階では implementer に「次タスクと一緒にコミットするので git add まで」と明示する。

---

## 共通コンテキスト（全 subagent に渡すテキスト）

> 以下のブロックは implementer / reviewer の prompt 冒頭にそのまま貼り付ける。

```
## プロジェクトの基本

- リポジトリ: bms-elsa（BMS 譜面管理 Wails アプリ）
- 構成: Go バックエンド + Svelte/TypeScript フロントエンド + SQLite
- 設計ドキュメント: docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md（必要に応じて参照）

## 言語・コミュニケーション

- すべてのコミットメッセージ・ファイル出力・コード内コメントは **日本語**
- エラーメッセージや説明も日本語

## ビルド・テストコマンド

- Go ビルド: `go build ./...`（ルートに `.` でビルドするとバイナリが残るので必ず `./...`）
- Go テスト: `go test ./...` または特定パッケージ単位
- フロント型チェック: `cd frontend && npx tsc --noEmit`
- フロントビルド: `cd frontend && npm run build`
- Wails バインディング再生成: `wails generate module`（または `wails dev` を一度起動）

## コーディング規約

- TDD 厳守: 各 Task の Step は「失敗テスト追加 → 失敗確認 → 実装 → 成功確認 → コミット」を踏む。Step 順を守る
- コメント方針: 動作を逐一説明する冗長コメントは書かない。慣例に反する箇所・特殊仕様・「なぜ」を説明したい箇所のみコメント
- フロントエンド UI: `docs/style-guide.md` のデザイン規約に従う
- LSP 利用: Go/TypeScript/Python/PHP は LSP で定義参照・参照検索を優先（Grep より先）

## Git 運用

- 現在のブランチ: `feature/bmssearch-info-display`（main にコミットしない）
- コミットメッセージは既存の慣例に倣う（短い prefix + 日本語タイトル例: "feat: ..." "fix: ..." "docs: ..." "migration: ..." "spike: ..."）
- 各 Task の指示通りに `git add <具体的なパス>` を使う。`git add -A` や `git add .` は使わない
- push / PR はユーザー指示があるまで行わない

## 報告フォーマット

完了報告は以下のいずれかのステータスで返すこと:

- DONE: Task 完了、レビュー可能
- DONE_WITH_CONCERNS: 完了したが懸念あり（具体的に列挙）
- NEEDS_CONTEXT: 不足情報があり進められない（何が必要か明示）
- BLOCKED: 物理的に進められない（理由と原因を明示）

## 既存コードの確認方針

実装前に既存ファイルを Read / Grep で確認する。特に以下が必要なケース:

- 既存ハンドラー / リポジトリの命名規則
- DI 組み立てている app.go の既存構造
- テストヘルパー（newTestDB 等）の有無

ファイル全体を読まず、関連シンボル周辺だけ抽出する。
```

---

## Task 個別の前提・補足

各 Task について、implementer dispatch 時に共通コンテキストの後ろに追加するメモ。
**「補足」が空の Task は plan 該当行のみで自己完結する。**

### Task 1: 事前調査スパイク（L55–L319）

- **前提**: なし（最初の Task）
- **補足**:
  - 出力ディレクトリ名は実施日（=今日の日付）に合わせる。本ドキュメント作成時点では `2026-04-27`
  - スクリプト実行で外部 API（api.bmssearch.net）を叩く。100ms 程度のレートリミットは plan のコード通りに守る
  - 調査スクリプトは `cmd/probe-bmssearch/` 配下のため、`go build ./...` でルートにバイナリは残らない
  - 調査結果は plan の Step 5 で `*-design.md` の「フォールバック検索の正規化・スコアリング（暫定）」セクションを書き換える。タイトル末尾の「（暫定）」を削除すること
- **完了条件**: spike コミットが作成され、design.md が更新済み

### Task 2: マイグレーション追加（L325–L464）

- **前提**: なし
- **補足**:
  - `internal/adapter/persistence/migrations.go` の既存構造（`statements` スライスと `RunMigrations` 関数）に追記する形。既存の url_rewrite_rule マイグレーション周辺を Read して位置を特定すること
  - 冪等性確保のため `CREATE TABLE IF NOT EXISTS` と `pragma_table_info` チェックを使う
- **完了条件**: 単独コミット完了

### Task 3: ドメインエンティティ追加（L468–L573）

- **前提**: T2 完了（マイグレーション）
- **補足**:
  - **T4 と一緒にコミット**。T3 単独ではビルドが通らない（`MetaRepository` のメソッド不足）。implementer には「git add までで止めて、コミットは Task 4 完了後にまとめて」と明示
  - `internal/domain/model/song.go:57-62` の SongMeta 構造体を置換する。行番号が古い場合があるので Read で再確認
- **完了条件**: コードが揃い、T4 と合算でコミット待機

### Task 4: ElsaRepository 拡張（L577–L736）

- **前提**: T3 のコード変更が完了している
- **補足**:
  - `newTestDB` ヘルパーが既存テストにあるかを `internal/adapter/persistence/*_test.go` で確認。なければ既存パターン（例: `difficulty_table_repository_test.go`）から借用
  - 既存の `UpdateSongMetaEvent` も `bms_search_source = 'official'` を書き込むよう修正する。`grep -n "UpdateSongMetaEvent" internal/` で呼び出し側に副作用がないか念のため確認
  - `usecase_test.go` の `mockMetaRepo` に `UpdateSongMetaBMSSearch` メソッドを追加。後の T8 でフィールド付き fake に拡張するため、ここでは関数フィールド `updateSongMetaBMSSearchFn` を struct に追加し、メソッドはそれを呼ぶ実装にしておく（plan L1840-L1851 の指示と整合）
- **完了条件**: T3 + T4 を1コミット

### Task 5: BMSSearchRepository 実装（L740–L1104）

- **前提**: T2, T3, T4 完了
- **補足**:
  - `newTestDBWithMigration` テストヘルパーは新規作成。同名のヘルパーが既にある場合は重複定義回避
  - JSON カラムの空配列は `[]` で固定。`emptyIfNilURLEntries` / `emptyIfNilPreviews` で nil → `[]` 変換を必ず通す
- **完了条件**: 単独コミット

### Task 6: BMSSearchClient 拡張（L1110–L1281）

- **前提**: なし（独立）
- **補足**:
  - `BMSSearchBMS` 構造体への JSON フィールド追加は既存 `LookupBMS` の後方互換を壊さない。既存テスト `bmssearch_client_test.go` をまず実行して green を確認してから着手
  - `NewBMSSearchClientWithBaseURL` ヘルパーが既存にあるか要確認（plan の test コードが前提にしている）。ない場合は既存テストを Read してテスト構築パターンを揃えること
- **完了条件**: 単独コミット

### Task 7: 正規化・スコアリング pure functions（L1287–L1614）

- **前提**: T1 完了（調査結果が `2026-04-27-bmssearch-info-fallback-probe.md` に確定済み）
- **補足**:
  - **plan に記載のスコア配点・閾値・正規化ルールは初期案**。T1 の調査結果と異なる場合は **調査結果を優先** し、それに合わせてテストの期待値も更新する
  - `golang.org/x/text/unicode/norm` 依存追加が必要。`go get` 実行 → `go mod tidy` で確認 → コミット時に `go.mod` `go.sum` も add
- **完了条件**: 単独コミット

### Task 8: BMSSearchResolver 実装（L1618–L2088）

- **前提**: T3, T5, T6, T7 完了
- **補足**:
  - **モデルは capable 推奨**。複数 fake 実装と複合フローのため
  - `BMSSearchAPI` インターフェースを Resolver 用に切り出す。`BMSSearchClient` 自体はインターフェースを満たすため、本体は変更不要
  - T4 で追加した `mockMetaRepo.updateSongMetaBMSSearchFn` をテストで利用する
  - fake 実装（`fakeBMSClient` `fakeBMSSearchRepo`）は `internal/usecase/bmssearch_resolver_test.go` 内に閉じる（T9, T10 のテストで再利用するためテストパッケージ内で参照可能）
- **完了条件**: 単独コミット

### Task 13: SongdataReader 拡張（L2750–L2830）— ※前倒し

- **前提**: なし
- **補足**:
  - **plan 上は Phase 5 にあるが、T9/T10 が依存するため T8 の直後に前倒しで実行**
  - 既存メソッドの調査が必要。`grep -n "ListMD5sByFolder\|GetSongByFolder\|FolderResolver" internal/adapter/persistence/songdata_reader.go` を実行
  - 既に同等メソッドがある場合は新規追加せず再利用。シグネチャが合わなければ薄い wrapper だけ追加する判断
  - SQL 内の `songdata.song` という ATTACH スキーマ修飾は既存コードが採用しているパターン。`grep -n "songdata\." internal/adapter/persistence/songdata_reader.go` で既存例を確認してから揃える
- **完了条件**: T14 と一緒にコミット（git add まで）

### Task 9: LookupBMSSearchUseCase 実装（L2092–L2320）

- **前提**: T8, T13 完了
- **補足**:
  - DTO（`dto.BMSSearchInfoDTO`）はこの時点では未定義。implementer には「ビルドエラーは T12 完了後に解消する。git add まで実施しコミットは保留」と明示
  - テストは fake `ChartFolderResolver` を使うので、T8 の fake 実装と同居する
- **完了条件**: コミット保留（T12 で合算）

### Task 10: UnlinkBMSSearchUseCase 実装（L2323–L2476）

- **前提**: T5, T13 完了
- **補足**:
  - `FolderMD5sResolver` インターフェースを新設。T13 の `SongdataReader.ListMD5sInFolder` がこれを満たす
  - テストでは fake `FolderMD5sResolver` を使う
- **完了条件**: コミット保留（T12 で合算）

### Task 11: SyncBMSSearchUseCase 改修（L2479–L2649）

- **前提**: T8 完了
- **補足**:
  - 既存 `internal/usecase/sync_bmssearch.go` を **置換**。既存のロジック（並列実行・進捗・bmsCache）は plan の置換コードに集約されている
  - **plan に「既存テストがあれば更新」とあるが、新規挙動（フォールバック発動・新スキーマ書き込み）の最低1ケースは追加してほしい**。implementer に明示すること
  - `internal/app/event_handler.go` の呼び出し側修正:
    1. `grep -n "syncBMSSearch.Execute\|md5sByFolder" internal/app/event_handler.go` で該当箇所特定
    2. plan の「Before/After」コードに従って書き換え
    3. `h.songdataReader.GetSongByFolder` の存在確認（なければ T13 で追加したメソッドで代替）
  - bmsCache のメモリキャッシュは plan の置換コードでは省略されている。永続キャッシュ（`bmssearch_bms`）が同一同期実行内でもヒットするので機能的には等価。implementer に過剰な独自最適化はさせない
- **完了条件**: コミット保留（T12 で合算）

### Task 12: DTO 追加 + まとめコミット（L2654–L2747）

- **前提**: T9, T10, T11 のコード変更完了（コミット未実施）
- **補足**:
  - DTO 追加 → `go build ./...` 成功 → `go test ./internal/...` 全 PASS を確認 → T9〜T12 を1コミット
  - コミットメッセージ例: `feat: Lookup/Unlink/Sync ユースケース改修と BMSSearchInfoDTO 追加`
- **完了条件**: T9-T12 をまとめて1コミット

### Task 14: BMSSearchHandler + DI 組み立て（L2833–L3004）

- **前提**: T9, T10, T12, T13 完了
- **補足**:
  - **既存 app.go の構造を確認してから書く**。`grep -n "elsaRepo\|bmsSearchClient\|songdataReader\|BMSSearchHandler\|EventHandler" app.go` を実行し、既存変数名と Init() 内のコメント区分を把握
  - `syncBMSSearch` の生成も Resolver 委譲版に変える。`bmssearchRepo` `bmsResolver` の生成順序を `syncBMSSearch` より先に置く
  - `main.go` の `Bind:` スライスに `app.BMSSearchHandler` を追加
  - Wails バインディング再生成: `wails generate module` を試行。失敗する環境なら `wails dev` 起動で代替
  - 生成された `frontend/wailsjs/go/app/BMSSearchHandler.{ts,js}` がコミット対象に含まれているか確認
- **完了条件**: T13 + T14 をまとめて1コミット

### Task 15: search アイコン追加（L3009–L3038）

- **前提**: なし
- **補足**:
  - `frontend/src/components/icons.ts` の既存構造を Read して既存アイコン（folderMove 等）の形式に揃える
- **完了条件**: 単独コミット

### Task 16: BMSSearchInfoCard.svelte 実装（L3042–L3197）

- **前提**: T12（DTO 型生成）, T14（Handler 型生成）, T15
- **補足**:
  - `dto.BMSSearchInfoDTO` 型は T14 の `wails generate module` で `frontend/wailsjs/go/models.ts` に生成済みのはず。型解決できない場合は再度 generate
  - `applyRewriteRules` のシグネチャは既存実装（`frontend/src/lib/urlRewrite.ts` 推定）を Read して合わせる。plan の `applyRewriteRules(url, $rewriteRules)` で動かない場合は順序を入れ替える
  - daisyUI クラス（`btn`, `bg-base-200`, `link link-primary` 等）は既存 IRInfoCard.svelte と同じスケール感
- **完了条件**: 単独コミット

### Task 17: SongDetail への配置（L3201–L3286）

- **前提**: T14, T16 完了
- **補足**:
  - **既存 SongDetail.svelte の構造を Read して、`<!-- 譜面一覧 -->` コメントの実在を確認**。プレースホルダーが違う場合は楽曲ヘッダーと譜面一覧の境界を見つけて配置
  - `loadDetail` 関数の終端位置も実コードで確認
- **完了条件**: コミット保留（T19 で合算）

### Task 18: ChartDetail への配置（L3289–L3365）

- **前提**: T14, T16 完了
- **補足**: ChartInfoCard と IRInfoCard の間に配置。両者の存在を Read で確認
- **完了条件**: コミット保留

### Task 19: EntryDetail への配置（L3368–L3468）

- **前提**: T14, T16 完了
- **補足**:
  - 解除挙動は **常に md5 単位**（plan の判断による単純化）。spec とずれているが意図的
  - T17 + T18 + T19 を1コミット。コミットメッセージ例: `feat: 詳細画面に BMSSearchInfoCard を配置`
- **完了条件**: T17-T19 まとめて1コミット

### Task 20: マニュアル更新（L3473–L3515）

- **前提**: 機能完成（T19 まで）
- **補足**:
  - 「EntryDetail からの解除は md5 単位（楽曲フォルダ全体の解除は楽曲詳細から）」を**追記**するよう implementer に指示。plan 文言にこの記述がないので明示的に補う
- **完了条件**: 単独コミット

### Task 21: 全体ビルド・テスト・手動 QA（L3518–L3563）

- **前提**: 全 Task 完了
- **補足**:
  - implementer は手動 QA は実行できない。`go build ./...` `go test ./...` `cd frontend && npm run build` のみ実行し、手動 QA チェックリストは **ユーザーへの引き継ぎ** として整理して報告
- **完了条件**: 自動ビルド・テスト全 PASS、QA 引き継ぎを controller がユーザーに渡す

---

## Subagent 起動テンプレート

### implementer

```
あなたは bms-elsa リポジトリで Task N を実装するエンジニアです。

【共通コンテキスト】
（このドキュメント「## 共通コンテキスト」を貼り付け）

【Task N の前提・補足】
（このドキュメント「### Task N」の補足セクションを貼り付け）

【plan 該当行の生テキスト】
plan ファイル: docs/superpowers/plans/2026-04-27-bmssearch-info-display.md L<開始>-L<終了>

----- ここから plan 該当範囲 -----
（Read で取得した本文をそのまま貼り付け）
----- ここまで plan 該当範囲 -----

【作業指示】
- 上の plan 該当範囲に従い、Step を上から順に TDD で実装すること
- 各 Step の最後で指定されたコマンド実行と期待結果を満たしているか確認
- 「コミット保留」と本ドキュメントの補足にある場合は git add まで行いコミットは作成しない
- 完了後、ステータス（DONE / DONE_WITH_CONCERNS / NEEDS_CONTEXT / BLOCKED）と実施内容のサマリ、変更ファイル、テスト結果、コミット SHA（作成した場合）を報告
- 不足情報があれば実装着手前に質問する
```

### spec reviewer

```
あなたは bms-elsa リポジトリで Task N の spec 整合性レビューを行うレビュアーです。
コード品質ではなく **設計仕様との対応** だけを見ます。

【設計ドキュメント該当箇所】
ファイル: docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md

このタスクが対応する spec セクション:
（Task に応じて該当セクション名を列挙。例: Task 5 なら「データモデル」「ドメインエンティティ」「テーブル定義」）

【レビュー対象コミット】
SHA: <implementer の最新コミット>
（または「未コミット」の場合は git diff の対象範囲）

【レビュー観点】
- spec の各要件が実装に反映されているか（漏れの検出）
- spec が要求していない実装が含まれていないか（過剰実装の検出）
- 型・メソッド名・テーブル名・カラム名が spec と一致しているか
- spec に書かれた挙動と実装挙動が一致しているか

【出力】
- ✅ Spec compliant: 問題なし
- ❌ Issues:
  - Missing: 〜
  - Extra: 〜
  - Mismatch: 〜
```

### code quality reviewer

```
あなたは bms-elsa リポジトリで Task N のコード品質レビューを行うレビュアーです。
spec 整合性は別レビュアーが完了済みです。

【レビュー対象コミット】
SHA: <implementer の最新コミット>

【プロジェクト規約】
- コメントは日本語、動作の逐一説明は避ける（理由・特殊仕様のみ）
- TDD: 実装にテストが伴っているか
- 既存パターンへの準拠（既存ファイルの命名・構造との整合）
- 過剰な抽象化・YAGNI 違反の指摘
- セキュリティ・エラーハンドリングの妥当性
- フロントエンド変更がある場合は docs/style-guide.md への準拠

【出力フォーマット】
Strengths: 〜
Issues:
  - [Critical] 〜
  - [Important] 〜
  - [Nit] 〜
Approval: ✅ Approved / ❌ Needs fixes
```

### final reviewer（全 Task 完了後）

```
あなたは bms-elsa リポジトリで feature/bmssearch-info-display ブランチの最終レビュアーです。

【対象コミット範囲】
git log main..HEAD

【観点】
- ブランチ全体としての一貫性
- 仕様書（docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md）との総合的な整合
- 統合テスト観点（手動 QA 項目を含む）
- マニュアル（docs/manual.md）の更新妥当性
- マージ可能性

【出力】
- 総評
- マージブロッカーの有無
- 残課題リスト
```

---

## 完了条件

- 全 Task のチェックボックスが完了
- `go build ./...`, `go test ./...`, `cd frontend && npm run build` が PASS
- 手動 QA チェックリスト（Task 21）がユーザーに引き渡され、ユーザー実施待ちの状態
- ブランチ `feature/bmssearch-info-display` が最終 reviewer で承認済み
- `superpowers:finishing-a-development-branch` への引き継ぎ準備完了
