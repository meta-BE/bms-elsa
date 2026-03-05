<script lang="ts">
  import { EstimateInstallLocation } from '../../wailsjs/go/app/DifficultyTableHandler'
  import { OpenFolder } from '../../wailsjs/go/main/App'

  export let md5: string
  export let tableID: number

  type Candidate = {
    folderPath: string
    title: string
    artist: string
    matchTypes: string[]
  }

  let candidates: Candidate[] = []
  let loading = false

  $: if (md5 && tableID) load(md5, tableID)

  async function load(hash: string, tid: number) {
    loading = true
    candidates = []
    try {
      candidates = (await EstimateInstallLocation(hash, tid)) || []
    } catch (e) {
      console.error('Failed to estimate install location:', e)
    } finally {
      loading = false
    }
  }

  function matchLabel(mt: string): string {
    return mt === 'title' ? 'タイトル一致' : 'URL一致'
  }
</script>

<div class="bg-base-200 rounded-lg p-3">
  <h3 class="text-sm font-semibold mb-2">導入先の推定</h3>

  {#if loading}
    <div class="flex justify-center py-2">
      <span class="loading loading-spinner loading-sm"></span>
    </div>
  {:else if candidates.length === 0}
    <p class="text-sm text-base-content/50">一致する導入済み楽曲が見つかりませんでした</p>
  {:else}
    <div class="space-y-2">
      {#each candidates as c}
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <p class="text-sm truncate">{c.title} / {c.artist}</p>
            <p class="text-xs text-base-content/50 truncate">{c.folderPath}</p>
            <div class="flex gap-1 mt-0.5">
              {#each c.matchTypes as mt}
                <span class="badge badge-xs">{matchLabel(mt)}</span>
              {/each}
            </div>
          </div>
          <button
            class="btn btn-ghost btn-xs shrink-0"
            title="フォルダを開く"
            on:click={() => OpenFolder(c.folderPath)}
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
            </svg>
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>
