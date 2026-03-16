# フロントエンドコンポーネントリファクタ 設計ドキュメント

## 目的

App.svelteと各テーブル/詳細コンポーネント間の重複コードを整理し、保守性を向上させる。

## 方針

ボトムアップアプローチで、小さい共通部品から段階的に抽出する。

### スコープ内

1. ユーティリティ関数の抽出
2. SearchInputコンポーネント
3. SortableHeaderコンポーネント
4. SplitPane + App.svelteリファクタ

### スコープ外（意図的に除外）

- **VirtualTableジェネリックコンポーネント**: 各テーブルの行描画・データ取得・行高さなどの差異が大きく、スロット/propsで吸収すると設定項目だらけになるリスクがある。重複しているのはTanstack Table/Virtualのボイラープレートで、安定しており変更頻度が低いため重複コストが低い。
- **DetailHeader/IrInfoSection**: 詳細コンポーネントは今後仕様変更が入る可能性があり、安定してから共通化する方が合理的。

## 設計

### 1. ユーティリティ関数の抽出

**新規:** `frontend/src/utils/chartLabels.ts`

SongDetail, ChartDetail, EntryDetailに完全コピーされている`modeLabel()`と`diffLabel()`を1箇所に集約する。

```typescript
export function modeLabel(mode: number): string {
  const labels: Record<number, string> = { 5: '5K', 7: '7K', 9: 'PMS', 10: '10K', 14: '14K', 25: '24K' }
  return labels[mode] || `${mode}K`
}

export function diffLabel(diff: number): string {
  const labels = ['', 'BEG', 'NOR', 'HYP', 'ANO', 'INS']
  return labels[diff] || ''
}
```

**変更対象:** SongDetail.svelte, ChartDetail.svelte, EntryDetail.svelte（ローカル定義をimportに置換）

### 2. SearchInputコンポーネント

**新規:** `frontend/src/SearchInput.svelte`

3つのテーブルで重複している「検索窓 + オーバーレイクリアボタン」を1コンポーネントに。

**Props:**
- `value: string` (bind可能、双方向バインディング)
- `placeholder?: string` (デフォルト: "検索...")

**Events:**
- `input`: テキスト入力時（親でdebounce等のハンドラを接続）
- `clear`: クリアボタン押下時（親で検索リセット処理を実行）

**使用例:**
```svelte
<SearchInput bind:value={searchText} on:input={handleSearchInput} on:clear={doSearch} />
```

**変更対象:** SongTable.svelte, ChartListView.svelte, DifficultyTableView.svelte（インラインの検索UIをSearchInputに置換）

### 3. SortableHeaderコンポーネント

**新規:** `frontend/src/SortableHeader.svelte`

3つのテーブルで完全に同一のソート可能テーブルヘッダーを1コンポーネントに。

**Props:**
- `table`: Tanstack Tableインスタンス（`$table`を渡す）

**描画内容:**
- `bg-base-200 border-b border-base-300`のコンテナ
- 各ヘッダーセルにソートハンドラ + ▲▼インジケーター
- `on:click|stopPropagation`でのソートトグル
- キーボード操作（Enter/Space）対応

**使用例:**
```svelte
<SortableHeader table={$table} />
```

**変更対象:** SongTable.svelte, ChartListView.svelte, DifficultyTableView.svelte（ヘッダーHTMLをSortableHeaderに置換）

### 4. SplitPane + App.svelteリファクタ

**新規:** `frontend/src/SplitPane.svelte`

App.svelteで3回繰り返されている「上部リスト + ドラッグセパレーター + 下部詳細」のレイアウトパターンを1コンポーネントに。

**Props:**
- `showDetail: boolean` (詳細パネルの表示/非表示)
- `splitRatio: number` (bind可能、0.2-0.8の範囲)

**Slots:**
- `list`: 上部のテーブルコンポーネント
- `detail`: 下部の詳細コンポーネント

**内部ロジック:**
- ドラッグリサイズ（onDragStart/Move/End）をSplitPane内に閉じ込め
- `showDetail`がfalseの場合、listがflex:1で全体表示
- セパレーターのHTML/スタイルを1箇所に集約

**App.svelteの変更後イメージ:**
```svelte
{#if activeTab === 'songs'}
  <SplitPane showDetail={!!selectedFolderHash} bind:splitRatio>
    <SongTable slot="list" selected={selectedFolderHash} on:select={handleSelect} on:deselect={handleDeselect} />
    {#if selectedFolderHash}
      <SongDetail slot="detail" folderHash={selectedFolderHash} on:close={handleClose} />
    {/if}
  </SplitPane>
{/if}
```

App.svelteからドラッグ関連の変数・関数（dragging, containerEl, splitRatio, onDragStart, onDragMove, onDragEnd）が削除される。
