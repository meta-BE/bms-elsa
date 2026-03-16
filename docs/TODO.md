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
- [x] 重複検知にMinHash統合（WAV定義類似度50%の重みでスコアリング、詳細画面にWAV定義類似度表示）
- [x] URL書き換えルール（url_rewrite_rulesテーブル、replace/regex対応、優先度付きルール適用、動作URL自動推定）
- [x] LR2IR情報のリンク化（詳細ビューのLR2IR情報ヘッダーをLR2IRページへの直接リンクに変更）
- [x] 外部リンクのシステムブラウザ表示（クロスプラットフォーム対応: macOS/Windows/Linux）
- [x] カラムフィルタ（楽曲・譜面一覧のEVENT/YEAR、難易度表のSTATUSをドロップダウンフィルタに変更）
- [x] フォルダを開く（楽曲詳細・譜面詳細・難易度表エントリ・重複詳細からインストール先フォルダを表示）
- [x] BMSパーサー実装（WAV定義抽出、MinHash署名計算）
- [x] MinHash計算・保存（フォルダ走査 + worker pool、進捗表示・中断対応）
- [x] 導入先推定（MinHashスコアリング + IR照合 + タイトル/アーティスト一致の統合スコアリング）
- [x] 差分導入（BMS/BME/BMLファイルD&D → 導入先自動推定 → ファイル移動）
- [x] `ScanDuplicatesUseCase` を `DuplicateHandler` 経由に変更（クリーンアーキテクチャの一貫性確保）
- [x] 難易度表設定を独立モーダルに分離（Settings.svelte → DifficultyTableSettings.svelte）
- [x] 難易度表のドラッグ&ドロップ並び替え（sort_orderカラム追加 + svelte-dnd-action）
- [x] arrowNav汎用化 + DuplicateViewにキーボードナビゲーション追加
- [x] フォルダマージ（重複検知から2フォルダを1つに統合、競合は作成日時判定、ログ記録対応、設定画面にファイル別ログ出力チェックボックス追加）
- [x] パス検索（楽曲一覧・譜面一覧でフォルダパスによる検索、「パス」トグルで切り替え）

## 難易度表関連
- [ ] 段位認定（course）データの取り込み
- [ ] url/url_diff → working_body_url の推定・反映
- [ ] 未所持譜面の導入機能
- [ ] 譜面一覧ビューでの難易度表フィルタ・ソート

## 楽曲管理
- [ ] フォルダ移動（都度指定で移動先選択、beatoraja再スキャンはユーザー手動）
- [ ] 楽曲導入（URL提示 + フォルダ取り込み）
- [x] 楽曲マージ（重複検知から選択した2フォルダのファイルを1フォルダに統合し、移動元フォルダを削除）

## IR・メタデータ
- [ ] イベントページパース（イベント情報の自動取得）

## BMS基盤
- [ ] MD5/SHA256計算
- [ ] 文字エンコーディングの汎用検出（`chardet`等のライブラリ導入でShift-JIS以外のエンコーディングにも対応、EUC-KR等の韓国語BMSなど）

## リファクタリング

### バックエンド（優先度: 高）
- [x] usecase層のadapter依存を解消（`EstimateDiffInstallUseCase` が `*persistence.ElsaRepository` 具象型に直接依存 → `FindMostSimilarByMinHash` をインターフェースに追加）

### バックエンド（優先度: 中）
- [x] IR一括取得メソッドの統合（`StartBulkFetch` と `StartDifficultyTableBulkFetch` の共通部分を抽出）
- [x] persistence層の独自型を `domain/model` に移動（`ChartScanTarget`, `SongGroup`。`MinHashMatch` は移動済み）
- [x] `ScanHandler` のMinHashスキャンロジックをusecase層に抽出（BMSパース→MinHash計算→DB保存）
- [x] `ScanDuplicates` をusecase化（`app.go` から `similarity` 直接参照を解消）
- [x] `ScanDuplicatesUseCase` を `DuplicateHandler` 経由に変更（Handler層の一貫性）
- [ ] `app.go` の Config型・設定関連関数を `config.go` に分離

### バックエンド（優先度: 低）
- [ ] DTOの配置統一（`DiffImportHandler` のDTOを `dto/dto.go` に移動）
- [ ] エラー無視の修正（`UpsertChartMeta`/`UpsertSongMeta` の戻りエラーを適切に処理）
- [ ] `joinStrings` を `strings.Join` に置換

### フロントエンド
- [x] IR一括取得イベント処理パターンの共通化（ChartListView / DifficultyTableView で重複）
- [x] Wails生成型の活用（DuplicateView でローカル型を再定義している箇所を解消）
- [x] arrowNav.ts を汎用化（tanstack-table依存を除去、ジェネリクス化）

## 改善
- [ ] BPM検知の改善（songdata.dbのminbpm/maxbpmにギミックBPMが含まれるケースへの対応）
