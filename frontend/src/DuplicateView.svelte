<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { ScanDuplicates } from '../wailsjs/go/main/App'

  const dispatch = createEventDispatcher()

  export let active = false

  type ScoreResult = {
    Title: number
    Artist: number
    Genre: number
    BPM: number
    Total: number
  }

  type DuplicateMember = {
    FolderHash: string
    Title: string
    Artist: string
    Genre: string
    MinBPM: number
    MaxBPM: number
    ChartCount: number
    Path: string
    Scores: ScoreResult
  }

  type DuplicateGroup = {
    ID: number
    Members: DuplicateMember[]
    Score: number
  }

  let groups: DuplicateGroup[] = []
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

  function handleSelect(group: DuplicateGroup) {
    selectedGroupID = group.ID
    dispatch('select', group)
  }

  $: selectedGroup = groups.find(g => g.ID === selectedGroupID) || null
</script>

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
