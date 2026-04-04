<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import ProgressBar from './ProgressBar.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'

  // 開始APIを呼ぶ関数。呼び出し元が差し替える
  export let startFn: () => Promise<void>
  export let stopFn: () => void

  const dispatch = createEventDispatcher<{ done: void }>()

  let fetching = false
  let progress = { current: 0, total: 0 }
  let doneMessage = ''
  let doneTimer: ReturnType<typeof setTimeout> | null = null

  export function start() {
    fetching = true
    progress = { current: 0, total: 0 }
    doneMessage = ''
    if (doneTimer) { clearTimeout(doneTimer); doneTimer = null }
    startFn().catch((e: Error) => {
      console.error('[IR] BulkFetch failed:', e)
      fetching = false
    })
  }

  function stop() {
    stopFn()
  }

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  onMount(() => {
    offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
      if (!fetching) return
      progress = data
    })
    offDone = EventsOn('ir:done', (data: { total: number; fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
      if (!fetching) return
      fetching = false
      const parts: string[] = []
      if (data.total === 0) {
        doneMessage = '対象なし'
      } else {
        if (data.fetched > 0) parts.push(`${data.fetched}件取得`)
        if (data.notFound > 0) parts.push(`${data.notFound}件未登録`)
        if (data.failed > 0) parts.push(`${data.failed}件失敗`)
        if (data.cancelled) parts.push('中断')
        doneMessage = parts.join(', ') || '完了'
      }
      doneTimer = setTimeout(() => { doneMessage = '' }, 5000)
      dispatch('done')
    })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
    if (doneTimer) clearTimeout(doneTimer)
  })
</script>

{#if fetching}
  <div class="flex-1">
    <ProgressBar current={progress.current} total={progress.total} cancelable on:cancel={stop} />
  </div>
{:else if doneMessage}
  <span class="text-xs text-success">{doneMessage}</span>
{:else}
  <button class="btn btn-xs btn-outline" on:click|stopPropagation={start}>IR取得</button>
{/if}
