# TODO

## ドメイン層
- [ ] domain/model: Song, Chart, BMSHeader, BMSDefinitions, IRMetadata, ValidationResult
- [ ] domain/repository: SongRepository, ChartRepository インターフェース
- [ ] domain/service: ChartValidator, MetadataMatcher

## ポート
- [ ] port: FileSystem, BMSParser, IRClient, Hasher, EventEmitter インターフェース

## アダプタ層
- [ ] adapter/parser: BMSパーサー実装
- [ ] adapter/persistence: SQLiteスキーマ定義・マイグレーション
- [ ] adapter/persistence: SQLiteリポジトリ実装
- [ ] adapter/filesystem: ディレクトリ走査、ファイル操作、MD5計算
- [ ] adapter/gateway: LR2IRクライアント（HTMLスクレイピング）

## ユースケース
- [ ] usecase: ScanSongs（フォルダ走査 + worker pool）
- [ ] usecase: ListSongs（ページング付き一覧）
- [ ] usecase: ImportSong, ImportChart（楽曲・差分導入）
- [ ] usecase: ValidateChart（差分正当性検証）
- [ ] usecase: LookupIR（LR2IR照合）
- [ ] usecase: RenameSong, MoveSong

## Wailsバインディング層
- [ ] app: ハンドラー（Scan, Song, Chart, IR）
- [ ] app/dto: フロントエンド向けDTO群
- [ ] app/event: WailsEventEmitter実装
- [ ] main.go: DI組み立て（全層の結合）

## フロントエンド
- [ ] Svelte 4 or 5 の最終決定
- [ ] TanStack Table + Virtual 導入
- [ ] shadcn-svelte or DaisyUI の最終決定・導入
- [ ] 楽曲一覧テーブル画面
- [ ] 走査進捗表示（Wailsイベント受信）

## 追加設計
- [ ] 大量ファイル一覧API詳細設計
- [ ] イベント駆動型進捗通知詳細設計
- [ ] 最小バイナリ構成検証
