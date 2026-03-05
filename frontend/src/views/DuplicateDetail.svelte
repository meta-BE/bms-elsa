<script lang="ts">
  import { GetSongDetail } from '../../wailsjs/go/app/SongHandler'
  import type { dto } from '../../wailsjs/go/models'

  export let group: {
    ID: number
    Score: number
    Members: {
      FolderHash: string
      Title: string
      Artist: string
      Genre: string
      MinBPM: number
      MaxBPM: number
      ChartCount: number
      Path: string
      Scores: { Title: number; Artist: number; Genre: number; BPM: number; Total: number }
    }[]
  } | null = null

  // メンバーごとの譜面詳細をキャッシュ
  let chartsMap: Record<string, dto.ChartDTO[]> = {}

  // groupが変わったら譜面詳細を取得
  $: if (group) {
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
</script>

{#if group}
  <div class="p-3 space-y-3">
    <div class="flex items-center gap-2 text-sm font-semibold">
      <span>グループ #{group.ID}</span>
      <span class="badge badge-sm badge-primary">{group.Score}%</span>
    </div>

    {#each group.Members as member, i}
      <div class="card card-compact bg-base-200">
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
          <div class="text-sm text-base-content/50 break-all">{folderPath(member.Path)}</div>

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
        </div>
      </div>
    {/each}

    {#if group.Members.length >= 2}
      {@const scores = group.Members[0].Scores}
      <div class="text-base-content/60 space-y-1">
        <div class="text-sm font-semibold">類似度内訳</div>
        <div class="text-sm flex gap-4">
          <span>title {scores.Title}%</span>
          <span>artist {scores.Artist}%</span>
          <span>genre {scores.Genre}%</span>
          <span>BPM {scores.BPM}%</span>
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="flex items-center justify-center h-full text-base-content/40 text-sm">
    グループを選択してください
  </div>
{/if}
