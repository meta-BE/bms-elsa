# OpenFolderButton コンポーネント化 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 「フォルダを開く」ボタンを共通コンポーネントに抽出し、DiffImportViewのファイル名・推定先セルに追加する

**Architecture:** `OpenFolderButton.svelte` を作成し、`path` と `size` propsでサイズ制御。既存4箇所を置換し、DiffImportViewに2箇所追加。

**Tech Stack:** Svelte, DaisyUI, Wails runtime (`OpenFolder`)

---

### Task 1: OpenFolderButton コンポーネントを作成

**Files:**
- Create: `frontend/src/components/OpenFolderButton.svelte`

**Step 1: コンポーネントを作成**

```svelte
<script lang="ts">
  import { OpenFolder } from '../../wailsjs/go/main/App'

  export let path: string = ''
  export let size: 'xs' | 'sm' = 'sm'
  export let title: string = 'フォルダを開く'

  const sizeMap = {
    xs: { btn: 'btn-xs', icon: 'h-3 w-3' },
    sm: { btn: 'btn-xs', icon: 'h-4 w-4' },
  }

  $: s = sizeMap[size]
</script>

{#if path}
  <button
    class="btn btn-ghost {s.btn}"
    {title}
    on:click|stopPropagation={() => OpenFolder(path)}
  >
    <svg xmlns="http://www.w3.org/2000/svg" class={s.icon} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
    </svg>
  </button>
{/if}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: ビルド成功

**Step 3: コミット**

```bash
git add frontend/src/components/OpenFolderButton.svelte
git commit -m "feat: OpenFolderButtonコンポーネントを作成"
```

---

### Task 2: 既存4箇所をコンポーネントに置換

**Files:**
- Modify: `frontend/src/views/SongDetail.svelte`
- Modify: `frontend/src/views/ChartDetail.svelte`
- Modify: `frontend/src/views/EntryDetail.svelte`
- Modify: `frontend/src/components/InstallCandidateCard.svelte`

**Step 1: SongDetail.svelte を修正**

importを変更:
```
- import { OpenFolder } from '../../wailsjs/go/main/App'
+ import OpenFolderButton from '../components/OpenFolderButton.svelte'
```

80-90行目のボタンブロックを置換:
```svelte
<!-- 変更前 -->
{#if detail.charts[0]?.path}
  <button
    class="btn btn-ghost btn-xs"
    title="インストール先フォルダを開く"
    on:click={() => OpenFolder(detail.charts[0].path)}
  >
    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
    </svg>
  </button>
{/if}

<!-- 変更後 -->
<OpenFolderButton path={detail.charts[0]?.path} title="インストール先フォルダを開く" />
```

注意: `OpenFolder` のimportが他で使われていないことを確認して削除する。

**Step 2: ChartDetail.svelte を同様に修正**

importを変更し、60-70行目のボタンブロックを置換:
```svelte
<OpenFolderButton path={chart?.path} title="インストール先フォルダを開く" />
```

**Step 3: EntryDetail.svelte を同様に修正**

importを変更し、81-91行目のボタンブロックを置換:
```svelte
<OpenFolderButton path={chart?.path} title="インストール先フォルダを開く" />
```

**Step 4: InstallCandidateCard.svelte を同様に修正**

importを変更し、66-74行目のボタンブロックを置換:
```svelte
<OpenFolderButton path={c.folderPath} />
```

注意: このファイルのボタンには `shrink-0` クラスがあるが、コンポーネント側で対応不要（親のflexで制御される）。

**Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: ビルド成功

**Step 6: コミット**

```bash
git add frontend/src/views/SongDetail.svelte frontend/src/views/ChartDetail.svelte frontend/src/views/EntryDetail.svelte frontend/src/components/InstallCandidateCard.svelte
git commit -m "refactor: 既存のフォルダを開くボタンをOpenFolderButtonコンポーネントに置換"
```

---

### Task 3: DiffImportView にフォルダを開くアイコンを追加

**Files:**
- Modify: `frontend/src/views/DiffImportView.svelte`

**Step 1: importを追加**

```svelte
import OpenFolderButton from '../components/OpenFolderButton.svelte'
```

**Step 2: ファイル名セルにアイコンを追加**

168行目を変更:
```svelte
<!-- 変更前 -->
<td class="text-sm font-mono truncate max-w-48" title={candidate.filePath}>{candidate.fileName}</td>

<!-- 変更後 -->
<td class="text-sm font-mono max-w-48">
  <span class="flex items-center gap-1">
    <OpenFolderButton path={candidate.filePath} size="xs" title="ファイルのフォルダを開く" />
    <span class="truncate" title={candidate.filePath}>{candidate.fileName}</span>
  </span>
</td>
```

**Step 3: 推定先セルにアイコンを追加**

175-181行目を変更:
```svelte
<!-- 変更前 -->
<td class="text-sm truncate max-w-64" title={candidate.destFolder}>
  {#if candidate.destFolder}
    <span class="text-success">{candidate.destFolder}</span>
  {:else}
    <span class="text-base-content/30">-</span>
  {/if}
</td>

<!-- 変更後 -->
<td class="text-sm max-w-64">
  {#if candidate.destFolder}
    <span class="flex items-center gap-1">
      <OpenFolderButton path={candidate.destFolder} size="xs" title="推定先フォルダを開く" />
      <span class="truncate text-success" title={candidate.destFolder}>{candidate.destFolder}</span>
    </span>
  {:else}
    <span class="text-base-content/30">-</span>
  {/if}
</td>
```

**Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: ビルド成功

**Step 5: コミット**

```bash
git add frontend/src/views/DiffImportView.svelte
git commit -m "feat: DiffImportViewのファイル名・推定先セルにフォルダを開くアイコンを追加"
```
