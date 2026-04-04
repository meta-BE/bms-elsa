<script lang="ts">
  import { createEventDispatcher } from 'svelte'

  export let current: number
  export let total: number
  export let cancelable = false

  const dispatch = createEventDispatcher<{ cancel: void }>()

  $: percent = total > 0 ? Math.min((current / total) * 100, 100) : 0
  $: label = `${current.toLocaleString()} / ${total.toLocaleString()}`
</script>

<div class="flex items-center gap-2 flex-1">
  <div class="relative h-4 rounded-full bg-base-300 overflow-hidden flex-1">
    <div
      class="absolute inset-y-0 left-0 bg-primary rounded-full"
      style="width: {percent}%"
    ></div>
    <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content/70 select-none">
      {label}
    </span>
    <span
      class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-primary-content select-none"
      style="clip-path: inset(0 {100 - percent}% 0 0)"
    >
      {label}
    </span>
  </div>
  {#if cancelable}
    <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={() => dispatch('cancel')}>停止</button>
  {/if}
</div>
