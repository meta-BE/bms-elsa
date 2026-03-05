<script lang="ts">
  export let showDetail = false
  export let splitRatio = 0.6

  let containerEl: HTMLDivElement
  let dragging = false

  function onDragStart(e: MouseEvent) {
    e.preventDefault()
    dragging = true
    window.addEventListener('mousemove', onDragMove)
    window.addEventListener('mouseup', onDragEnd)
  }

  function onDragMove(e: MouseEvent) {
    if (!dragging || !containerEl) return
    const rect = containerEl.getBoundingClientRect()
    splitRatio = Math.max(0.2, Math.min(0.8, (e.clientY - rect.top) / rect.height))
  }

  function onDragEnd() {
    dragging = false
    window.removeEventListener('mousemove', onDragMove)
    window.removeEventListener('mouseup', onDragEnd)
    // ドラッグ終了直後のclickイベントがdeselectを発火するのを防ぐ
    window.addEventListener('click', suppressClick, { capture: true, once: true })
  }

  function suppressClick(e: MouseEvent) {
    e.stopPropagation()
    e.preventDefault()
  }
</script>

<div bind:this={containerEl} class="h-full flex flex-col">
  <div class="overflow-hidden" style="flex: {showDetail ? splitRatio : 1}">
    <slot name="list" />
  </div>
  {#if showDetail}
    <!-- svelte-ignore a11y-no-noninteractive-tabindex a11y-no-noninteractive-element-interactions -->
    <div
      class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
      on:mousedown={onDragStart}
      role="separator"
      tabindex="0"
    ></div>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="overflow-y-auto" style="flex: {1 - splitRatio}" on:click|stopPropagation>
      <slot name="detail" />
    </div>
  {/if}
</div>
