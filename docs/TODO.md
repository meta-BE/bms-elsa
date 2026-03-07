# TODO

## 実装済み
- [x] 初回表示時間の改善（folderインデックス追加で2.4秒→0.02秒に解決）
- [x] config.json / elsa.db のパス戦略（実行ファイル隣接）
- [x] 設定画面（songdata.dbパス設定、OSファイル選択ダイアログ）
- [x] BMS難易度表の取り込み・表示（URL登録→HTML→header→body取得、設定画面から管理）
- [x] 譜面詳細に難易度ラベルバッジ表示
- [x] 3タブ構成（楽曲一覧・譜面一覧・難易度表）
- [x] 各タブにインクリメンタル検索機能（楽曲はバックエンド検索、譜面・難易度表はフロントエンドフィルタ）
- [x] 難易度表のレベル数値順ソート（非数値レベルは末尾）
- [x] フロントエンドコンポーネントリファクタリング（SearchInput, SortableHeader, SplitPane抽出）
- [x] UI改善（deselect範囲拡大、行数表示統一、左右分割レイアウト）
- [x] IR一括取得（譜面一覧・難易度表から未取得譜面のLR2IR情報をバックグラウンド逐次取得、進捗表示、中断対応）
- [x] 難易度表の未導入譜面でもIR情報を表示（chart_metaから直接取得）
- [x] 楽曲メタデータ推測（event_mappingによるURL→EventName/ReleaseYear自動設定 + 手動確認フロー）
- [x] chart_meta PKをmd5単一キーに変更（sha256不要化）
- [x] 重複検知（タイトル・アーティスト類似度によるファジーマッチング、専用タブで一覧・詳細表示）
- [x] URL書き換えルール（url_rewrite_rulesテーブル、replace/regex対応、優先度付きルール適用、動作URL自動推定）
- [x] LR2IR情報のリンク化（詳細ビューのLR2IR情報ヘッダーをLR2IRページへの直接リンクに変更）
- [x] 外部リンクのシステムブラウザ表示（クロスプラットフォーム対応: macOS/Windows/Linux）
- [x] カラムフィルタ（楽曲・譜面一覧のEVENT/YEAR、難易度表のSTATUSをドロップダウンフィルタに変更）
- [x] フォルダを開く（楽曲詳細・譜面詳細・難易度表エントリ・重複詳細からインストール先フォルダを表示）
- [x] BMSパーサー実装（WAV定義抽出、MinHash署名計算）
- [x] MinHash計算・保存（フォルダ走査 + worker pool、進捗表示・中断対応）
- [x] 導入先推定（MinHashスコアリング + IR照合 + タイトル/アーティスト一致の統合スコアリング）
- [x] 差分導入（BMS/BME/BMLファイルD&D → 導入先自動推定 → ファイル移動）

## 難易度表関連
- [ ] 段位認定（course）データの取り込み
- [ ] url/url_diff → working_body_url の推定・反映
- [ ] 未所持譜面の導入機能
- [ ] 譜面一覧ビューでの難易度表フィルタ・ソート

## 楽曲管理
- [ ] フォルダ移動（都度指定で移動先選択、beatoraja再スキャンはユーザー手動）
- [ ] 楽曲導入（URL提示 + フォルダ取り込み）
- [ ] 楽曲マージ（重複検知から選択した2フォルダのファイルを1フォルダに統合し、移動元フォルダを削除）

## IR・メタデータ
- [ ] イベントページパース（イベント情報の自動取得）

## BMS基盤
- [ ] MD5/SHA256計算

## リファクタリング

### バックエンド（優先度: 高）
- [ ] usecase層のadapter依存を解消（`EstimateDiffInstallUseCase` が `*persistence.ElsaRepository` 具象型に直接依存 → `FindMostSimilarByMinHash` をインターフェースに追加）
- [ ] `DifficultyTableHandler` のビジネスロジックをusecase層に抽出（インストール状態判定・難易度表追加/更新フロー）
- [ ] `ScanHandler` のMinHashスキャンロジックをusecase層に抽出（BMSパース→MinHash計算→DB保存）

### バックエンド（優先度: 中）
- [ ] goroutine管理パターンの共通化（IR/Scan/DiffImportの `mu+running+cancelFunc` パターン → `BackgroundTask` 構造体に抽出）
- [ ] IR一括取得メソッドの統合（`StartBulkFetch` と `StartDifficultyTableBulkFetch` の共通部分を抽出）
- [ ] `app.go` の責務分離（Config型・設定関連関数を `config.go` に分離、`ScanDuplicates` をusecase化）
- [ ] persistence層の独自型を `domain/model` に移動（`ChartListItem`, `SongGroup`, `MinHashMatch` 等）

### バックエンド（優先度: 低）
- [ ] DTOの配置統一（`DiffImportHandler` のDTOを `dto/dto.go` に移動）
- [ ] エラー無視の修正（`UpsertChartMeta`/`UpsertSongMeta` の戻りエラーを適切に処理）
- [ ] `joinStrings` を `strings.Join` に置換

### フロントエンド
- [ ] 仮想テーブルロジックの共通化（ChartListView / SongTable / DifficultyTableView で重複）
- [ ] 動作URL推定ロジックの共通化（ChartListView / SongTable で重複）
- [ ] IR一括取得イベント処理パターンの共通化（ChartListView / DifficultyTableView で重複）
- [ ] Wails生成型の活用（DuplicateView でローカル型を再定義している箇所を解消）

## 改善
- [ ] BPM検知の改善（songdata.dbのminbpm/maxbpmにギミックBPMが含まれるケースへの対応）
