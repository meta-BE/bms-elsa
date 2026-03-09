# 設計判断の記録

## 見送り事項

### フロントエンド: 仮想テーブルロジックの共通化（2026-03-09）

ChartListView / SongTable / DifficultyTableView で createSvelteTable + createVirtualizer のボイラープレートが各20〜25行重複していたが、共通化を見送った。

理由:
- 各ビューのセル描画に差異が多い（2行セル、ステータス色分け等）
- Svelte 4ではジェネリックコンポーネントの型付けが難しく、共通化するとslot/props APIが複雑化する
- 抽象化コストが重複のコスト（各20行程度）を上回る

### バックエンド: `DifficultyTableHandler` のビジネスロジックusecase抽出を見送り（2026-03-10）

インストール状態判定の重複（`GetDifficultyTableEntry` と `ListDifficultyTableEntries` に同一ロジック）や `AddDifficultyTable` / `refreshTable` のエントリ変換コード重複は確認したが、見送りとした。

理由:
- 現状269行で機能として安定しており、バグの原因にはなっていない
- 難易度表機能は他機能ほど頻繁に変更が入る箇所ではない
- `DifficultyTable` / `DifficultyTableEntry` の model 移動＋リポジトリインターフェース化を同時に行う必要があり、工数に見合わない
- 将来難易度表周りに大きな機能追加が入る際に再検討する

### バックエンド: goroutine管理パターンの共通化（BackgroundTask）を見送り（2026-03-10）

IR/Scan/DiffImport の3ハンドラーで `mu+running+cancelFunc` パターンと `Stop` / `IsRunning` メソッドがコピペ同一であることを確認したが、`BackgroundTask` 構造体への抽出を見送りとした。

理由:
- DiffImportHandler は同期実行（goroutine なし）、IR/Scan は goroutine 起動と実行モデルが異なり、`TryStart` API の設計判断が難しい
- 設計を誤ると呼び出し側が逆に複雑化するリスクがある
- 各ハンドラーの Stop/IsRunning は各5行程度のボイラープレートで、実害は小さい
- 新たなバックグラウンドタスクが追加される段階で再検討する

### フロントエンド: 動作URL推定ロジックの共通化（2026-03-09）

ChartListView / SongTable の runInferWorkingURLs が約15行＋テンプレート6行ほぼ同一だが、共通化を見送った。

理由:
- 2箇所のみ、計20行程度の重複
- 差異はリロードコールバック1行のみだが、抽出するとかえってコードの追跡が困難になる
