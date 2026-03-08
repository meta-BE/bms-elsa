# 設計判断の記録

## 見送り事項

### フロントエンド: 仮想テーブルロジックの共通化（2026-03-09）

ChartListView / SongTable / DifficultyTableView で createSvelteTable + createVirtualizer のボイラープレートが各20〜25行重複していたが、共通化を見送った。

理由:
- 各ビューのセル描画に差異が多い（2行セル、ステータス色分け等）
- Svelte 4ではジェネリックコンポーネントの型付けが難しく、共通化するとslot/props APIが複雑化する
- 抽象化コストが重複のコスト（各20行程度）を上回る

### フロントエンド: 動作URL推定ロジックの共通化（2026-03-09）

ChartListView / SongTable の runInferWorkingURLs が約15行＋テンプレート6行ほぼ同一だが、共通化を見送った。

理由:
- 2箇所のみ、計20行程度の重複
- 差異はリロードコールバック1行のみだが、抽出するとかえってコードの追跡が困難になる
