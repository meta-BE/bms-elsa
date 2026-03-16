<script lang="ts">
  let dialog: HTMLDialogElement
  let message = ''
  let mouseDownOnBackdrop = false

  export function open(msg: string) {
    message = msg
    dialog.showModal()
  }

  function handleClose() {
    dialog.close()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) handleClose(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <p class="text-sm whitespace-pre-wrap">{message}</p>
    <div class="modal-action">
      <button class="btn" on:click={handleClose}>OK</button>
    </div>
  </div>
</dialog>
