<script lang="ts">
  import { EstimateInstallLocation } from '../../wailsjs/go/app/DifficultyTableHandler'
  import type { PaneId } from '../stores/cardCollapsed'
  import OpenFolderButton from './OpenFolderButton.svelte'
  import CollapsibleCard from './CollapsibleCard.svelte'

  export let md5: string
  export let tableID: number
  export let paneId: PaneId

  type Candidate = {
    folderPath: string
    title: string
    artist: string
    matchTypes: string[]
    score: number
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
    switch (mt) {
      case 'title': return 'タイトル一致'
      case 'base_title': return 'タイトル類似'
      case 'body_url': return 'URL一致'
      case 'artist': return 'アーティスト一致'
      default: return mt
    }
  }
</script>

<CollapsibleCard {paneId} cardId="installCandidate">
  <span slot="title">導入先の推定</span>
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
          <OpenFolderButton path={c.folderPath} />
        </div>
      {/each}
    </div>
  {/if}
</CollapsibleCard>
