<script lang="ts">
  import type { dto } from '../wailsjs/go/models'
  import { modeLabel, diffLabel } from './utils/chartLabels'

  export let chart: dto.ChartDTO
</script>

<div class="bg-base-200 rounded-lg p-3">
  <h3 class="text-sm font-semibold mb-2">譜面情報</h3>
  <div class="text-xs space-y-1">
    <div class="flex items-center gap-4">
      <span><span class="font-semibold">Mode:</span> {modeLabel(chart.mode)}</span>
      <span><span class="font-semibold">Difficulty:</span> {diffLabel(chart.difficulty)}</span>
      <span><span class="font-semibold">Level:</span> ☆{chart.level}</span>
    </div>
    <p>
      <span class="font-semibold">BPM:</span>
      {#if chart.minBpm === chart.maxBpm}
        {Math.round(chart.minBpm)}
      {:else}
        {Math.round(chart.minBpm)}-{Math.round(chart.maxBpm)}
      {/if}
    </p>
    {#if chart.difficultyLabels?.length}
      <div class="flex items-center gap-1 flex-wrap">
        <span class="font-semibold">難易度表:</span>
        {#each chart.difficultyLabels as label}
          <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
        {/each}
      </div>
    {/if}
    {#if chart.path}
      <p class="truncate">
        <span class="font-semibold">パス:</span>
        <span class="text-base-content/50">{chart.path}</span>
      </p>
    {/if}
  </div>
</div>
