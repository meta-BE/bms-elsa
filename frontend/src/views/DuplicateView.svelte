<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { handleArrowNav } from '../utils/arrowNav'
  import { ScanDuplicates } from '../../wailsjs/go/app/DuplicateHandler'
  import type { similarity } from '../../wailsjs/go/models'

  const dispatch = createEventDispatcher()

  export let active = false

  let groups: similarity.DuplicateGroup[] = []
  let scanning = false
  let scanned = false
  let selectedGroupID: number | null = null

  async function handleScan() {
    scanning = true
    try {
      const result = await ScanDuplicates()
      groups = (result || []).sort((a, b) => b.Score - a.Score)
      scanned = true
    } finally {
      scanning = false
    }
  }

  function handleSelect(group: similarity.DuplicateGroup) {
    selectedGroupID = group.ID
    dispatch('select', group)
  }

  function handleKeyNav(e: KeyboardEvent) {
    if (!active || !scanned) return
    handleArrowNav(e, {
      selected: selectedGroupID !== null ? String(selectedGroupID) : null,
      items: groups,
      getKey: (g: similarity.DuplicateGroup) => String(g.ID),
      onSelect: (g: similarity.DuplicateGroup) => handleSelect(g),
    })
  }

  $: selectedGroup = groups.find(g => g.ID === selectedGroupID) || null

  // App.svelte から呼び出される公開メソッド
  export function removeMember(folderHash: string) {
    for (const group of groups) {
      const idx = group.Members.findIndex(m => m.FolderHash === folderHash)
      if (idx !== -1) {
        group.Members.splice(idx, 1)
        groups = groups // リアクティビティ発火
        if (group.Members.length <= 1) {
          groups = groups.filter(g => g.ID !== group.ID)
          if (selectedGroupID === group.ID) {
            selectedGroupID = null
            dispatch('select', null)
          }
        }
        break
      }
    }
  }
</script>

<svelte:window on:keydown={handleKeyNav} />

{#if !scanned}
  <div class="flex items-center justify-center h-full">
    <button class="btn btn-primary" on:click={handleScan} disabled={scanning}>
      {scanning ? 'スキャン中...' : 'スキャン実行'}
    </button>
  </div>
{:else}
  <div class="flex items-center gap-2 px-2 py-1 text-sm text-base-content/60 border-b border-base-300">
    <button class="btn btn-xs btn-ghost" on:click={handleScan} disabled={scanning}>
      {scanning ? '...' : '再スキャン'}
    </button>
    <span>{groups.length} グループ</span>
  </div>
  <div class="overflow-y-auto h-full">
    <table class="table table-xs table-pin-rows">
      <thead>
        <tr>
          <th class="w-16">類似度</th>
          <th>タイトル</th>
          <th class="w-16">件数</th>
        </tr>
      </thead>
      <tbody>
        {#each groups as group}
          <tr
            class="cursor-pointer hover:bg-base-200 {selectedGroupID === group.ID ? 'bg-primary/10' : ''}"
            on:click={() => handleSelect(group)}
          >
            <td class="text-sm font-mono">{group.Score}%</td>
            <td class="text-sm">{group.Members[0]?.Title || ''}</td>
            <td class="text-sm">{group.Members.length}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
