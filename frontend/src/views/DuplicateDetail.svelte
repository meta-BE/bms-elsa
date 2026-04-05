<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetSongDetail } from '../../wailsjs/go/app/SongHandler'
  import { MergeFolders } from '../../wailsjs/go/app/DuplicateHandler'
  import type { dto, similarity } from '../../wailsjs/go/models'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'
  import AlertModal from '../components/AlertModal.svelte'

  const dispatch = createEventDispatcher()

  export let group: similarity.DuplicateGroup | null = null

  // メンバーごとの譜面詳細をキャッシュ
  let chartsMap: Record<string, dto.ChartDTO[]> = {}

  // マージ先として選択されたメンバーのFolderHash
  let mergeTargetHash: string | null = null
  let merging = false

  // 確認ダイアログ
  let confirmDialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let pendingSrcMember: similarity.DuplicateMember | null = null
  let confirmSrcPath = ''
  let confirmDestPath = ''
  let alertModal: AlertModal

  // groupが変わったらマージ先選択をリセット
  $: if (group) {
    mergeTargetHash = null
    for (const member of group.Members) {
      if (!chartsMap[member.FolderHash]) {
        fetchCharts(member.FolderHash)
      }
    }
  }

  async function fetchCharts(folderHash: string) {
    try {
      const detail = await GetSongDetail(folderHash)
      if (detail?.charts) {
        chartsMap = { ...chartsMap, [folderHash]: detail.charts }
      }
    } catch {
      // 取得失敗は無視
    }
  }

  function formatBPM(min: number, max: number): string {
    if (min === max) return String(Math.round(min))
    return `${Math.round(min)}-${Math.round(max)}`
  }

  function folderPath(path: string): string {
    const sep = path.includes('\\') ? '\\' : '/'
    const parts = path.split(sep)
    parts.pop()
    return parts.join(sep)
  }

  function fileName(path: string): string {
    const sep = path.includes('\\') ? '\\' : '/'
    const parts = path.split(sep)
    return parts[parts.length - 1] || path
  }

  function selectMergeTarget(folderHash: string) {
    mergeTargetHash = mergeTargetHash === folderHash ? null : folderHash
  }

  function requestMerge(srcMember: similarity.DuplicateMember) {
    if (!group || !mergeTargetHash) return
    const targetMember = group.Members.find(m => m.FolderHash === mergeTargetHash)
    if (!targetMember) return

    pendingSrcMember = srcMember
    confirmSrcPath = folderPath(srcMember.Path)
    confirmDestPath = folderPath(targetMember.Path)
    confirmDialog.showModal()
  }

  async function executeMerge() {
    if (!pendingSrcMember) return
    const srcMember = pendingSrcMember
    confirmDialog.close()

    merging = true
    try {
      const result = await MergeFolders(confirmSrcPath, confirmDestPath)
      if (result.success) {
        dispatch('memberMerged', { folderHash: srcMember.FolderHash })
      } else {
        alertModal.open(result.errorMsg || 'マージに失敗しました')
      }
    } catch (err) {
      alertModal.open(String(err))
    } finally {
      merging = false
      pendingSrcMember = null
    }
  }

  function cancelMerge() {
    confirmDialog.close()
    pendingSrcMember = null
  }
</script>

{#if group}
  <div class="p-3 space-y-3">
    <div class="flex items-center gap-2 text-sm font-semibold">
      <span>グループ #{group.ID}</span>
      <span class="badge badge-sm badge-primary">{group.Score}%</span>
    </div>

    {#each group.Members as member, i}
      <div class="card card-compact bg-base-200 {mergeTargetHash === member.FolderHash ? 'ring-2 ring-primary' : ''}">
        <div class="card-body">
          <div class="flex items-start justify-between">
            <div>
              <div class="text-lg font-bold">{member.Title}</div>
              <div class="text-sm text-base-content/70">{member.Artist}</div>
            </div>
            <div class="text-right text-sm text-base-content/50">
              <div>{member.Genre}</div>
              <div>BPM {formatBPM(member.MinBPM, member.MaxBPM)}</div>
              <div>{member.ChartCount}譜面</div>
            </div>
          </div>
          <div class="text-sm text-base-content/50 break-all flex items-center gap-1">
            <span>{folderPath(member.Path)}</span>
            <OpenFolderButton path={member.Path} size="xs" />
          </div>

          {#if chartsMap[member.FolderHash]}
            <div class="mt-1 space-y-0.5">
              {#each chartsMap[member.FolderHash] as chart}
                <div class="text-sm flex gap-2">
                  <span class="text-base-content/70">{chart.subtitle || fileName(chart.path || '')}</span>
                  <span class="text-xs text-base-content/50 break-all">{fileName(chart.path || '')}</span>
                </div>
              {/each}
            </div>
          {/if}

          <div class="mt-2 flex gap-2">
            <button
              class="btn btn-xs {mergeTargetHash === member.FolderHash ? 'btn-primary' : 'btn-outline btn-primary'}"
              on:click={() => selectMergeTarget(member.FolderHash)}
            >
              {mergeTargetHash === member.FolderHash ? 'マージ先 ✓' : 'マージ先に指定'}
            </button>
            {#if mergeTargetHash && mergeTargetHash !== member.FolderHash}
              <button
                class="btn btn-xs btn-warning"
                disabled={merging}
                on:click={() => requestMerge(member)}
              >
                {merging ? '処理中...' : '→ マージ'}
              </button>
            {/if}
          </div>
        </div>
      </div>
    {/each}

    {#if group.Members.length >= 2}
      {@const hasMD5Match = group.Members.some(m => m.MD5Match)}
      {@const fuzzyMembers = group.Members.filter(m => !m.MD5Match)}
      <div class="text-base-content/60 space-y-1">
        {#if hasMD5Match}
          <div class="text-sm"><span class="badge badge-sm badge-success">MD5一致</span></div>
        {/if}
        {#if fuzzyMembers.length > 0}
          {@const scores = fuzzyMembers[0].Scores}
          <div class="text-sm font-semibold">類似度内訳</div>
          <div class="text-sm flex gap-4">
            <span>WAV定義 {scores.WAV}%</span>
            <span>title {scores.Title}%</span>
            <span>artist {scores.Artist}%</span>
            <span>genre {scores.Genre}%</span>
            <span>BPM {scores.BPM}%</span>
          </div>
        {/if}
      </div>
    {/if}
  </div>
{:else}
  <div class="flex items-center justify-center h-full text-base-content/40 text-sm">
    グループを選択してください
  </div>
{/if}

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={confirmDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) cancelMerge(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">フォルダマージの確認</h3>
    <div class="space-y-2 text-sm">
      <p>移動元のフォルダを移動先にマージします。移動元は削除されます。</p>
      <div class="bg-base-200 rounded p-3 space-y-1">
        <div><span class="text-base-content/50">移動元:</span> <span class="break-all">{confirmSrcPath}</span></div>
        <div><span class="text-base-content/50">移動先:</span> <span class="break-all">{confirmDestPath}</span></div>
      </div>
    </div>
    <div class="modal-action">
      <button class="btn" on:click={cancelMerge}>キャンセル</button>
      <button class="btn btn-warning" on:click={executeMerge}>マージ実行</button>
    </div>
  </div>
</dialog>

<AlertModal bind:this={alertModal} />
