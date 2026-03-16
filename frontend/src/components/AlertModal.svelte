<script lang="ts">
  export type AlertLevel = 'error' | 'warning'

  let dialog: HTMLDialogElement
  let message = ''
  let level: AlertLevel = 'error'
  let mouseDownOnBackdrop = false

  const config = {
    error:   { title: 'エラー',  badge: 'badge-error' },
    warning: { title: '警告', badge: 'badge-warning' },
  } as const

  export function open(msg: string, lv: AlertLevel = 'error') {
    message = msg
    level = lv
    dialog.showModal()
  }

  function handleClose() {
    dialog.close()
  }

  $: c = config[level]
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) handleClose(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl border {c.border} {c.bg}">
    <div class="mb-3">
      <span class="badge {c.badge} badge-sm">{c.title}</span>
    </div>
    <p class="text-sm whitespace-pre-wrap">{message}</p>
    <div class="modal-action">
      <button class="btn btn-sm" on:click={handleClose}>OK</button>
    </div>
  </div>
</dialog>
