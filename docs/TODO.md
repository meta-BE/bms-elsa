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
- [x] ~~楽曲メタデータ推測（event_mappingによるURL→EventName/ReleaseYear自動設定 + 手動確認フロー）~~ → BMS Search連携に置換
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
- [x] LR2IRパーサーの`<br>`改行対応（備考フィールドの改行が保持されるように修正）
- [x] LR2IR備考のURL自動リンク化（備考内のURLをクリック可能なリンクとして表示）
- [x] コンテキストメニューにリンクアクション追加（リンク上の右クリックで「開く」「URLをコピー」）
- [x] macOSクリップボード文字化け修正（LANG環境変数未設定時のpbcopy文字化け対策、wails#4132）
- [x] 難易度表の並列一括更新（最大5並列、進捗表示・キャンセル対応）と個別更新ボタン
- [x] 難易度表切り替え時の検索テキスト保持
- [x] BMS Search API連携によるイベント情報管理（eventマスターテーブル導入、楽曲→イベント一括同期、3並列・進捗表示・中断対応）
- [x] イベントマスター管理UI（BMS Searchからの更新、短縮名編集）
- [x] 楽曲詳細にオートコンプリート付きイベント選択
- [x] 楽曲詳細にBMS Search・イベント本家ページリンク追加
- [x] 旧event_mapping（URLパターンマッチ）方式を廃止
- [x] 起動時バックグラウンドタスク自動実行（MinHashスキャン・難易度表一括更新・動作URL推定を起動時に並列自動実行、設定モーダルで進捗・結果確認）
- [x] BMS Search 楽曲情報表示・連携（楽曲/譜面/難易度表詳細にBMSSearchInfoCard表示、md5ベース取得・解除、公式マッチ+テキスト検索フォールバック+スコアリング採用、非公式紐付け警告表示）

## 難易度表関連
- [x] url/url_diff → working_body_url の推定・反映
- [x] 譜面一覧ビューでの難易度表フィルタ・ソート
- [ ] 段位認定（course）データの取り込み
- [ ] 未所持譜面の導入機能
- [ ] 難易度表ビューでLEVELカラムの挙動をソート→多数選択可能フィルタに

## 楽曲管理
- [x] フォルダ移動（楽曲詳細から移動先ディレクトリを選択して移動、rename優先・クロスFSフォールバック、移動済み行の黄色表示）
- [ ] 楽曲導入（URL提示 + フォルダ取り込み）
- [x] 楽曲マージ（重複検知から選択した2フォルダのファイルを1フォルダに統合し、移動元フォルダを削除）

## IR・メタデータ
- [x] イベント情報の自動取得（BMS Search API連携で実現）
- [ ] LR2IR CardでYouTube URL取得・インライン表示

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
- [x] SVGアイコンをIcon コンポーネントに集約（icons.tsにHeroiconsデータを一元管理、全9ファイル13箇所のインラインSVGを置換）

## 改善
- [ ] BPM検知の改善（songdata.dbのminbpm/maxbpmにギミックBPMが含まれるケースへの対応）
- [ ] 詳細画面の各カードUIを最小化可能に（ペイン×カード種別ごとに開閉状態を保存、最小化ボタンは左上）
