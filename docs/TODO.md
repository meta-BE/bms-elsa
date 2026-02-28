# TODO

## 優先度高
- [x] 初回表示時間の改善（folderインデックス追加で2.4秒→0.02秒に解決）

## 優先度中
- [ ] config.json / elsa.db のパス戦略（Windows/Mac非依存、バイナリ同梱 or 実行ファイル隣接）

## 優先度低
- [ ] BPM検知の改善（songdata.dbのminbpm/maxbpmにギミックBPMが含まれるケースへの対応）

## 未実装機能
- [ ] BMSパーサー実装
- [ ] フォルダ走査（ScanSongs: worker pool）
- [ ] 楽曲・差分の導入（ImportSong, ImportChart）
- [ ] 差分正当性検証（ValidateChart）
- [ ] 楽曲リネーム・移動（RenameSong, MoveSong）
- [ ] 走査進捗表示（Wailsイベント受信）
