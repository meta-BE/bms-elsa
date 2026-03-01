<script lang="ts">
  import { createEventDispatcher } from 'svelte'

  const dispatch = createEventDispatcher<{ input: void; clear: void }>()

  export let value = ''
  export let placeholder = '検索...'

  function handleClear() {
    value = ''
    dispatch('clear')
  }
</script>

<div class="relative">
  <input
    type="text"
    {placeholder}
    class="input input-xs input-bordered w-48 pr-6"
    bind:value
    on:input={() => dispatch('input')}
  />
  {#if value}
    <button
      class="absolute right-1 top-1/2 -translate-y-1/2 btn btn-ghost btn-xs btn-circle h-4 w-4 min-h-0 p-0"
      on:click={handleClear}
    >✕</button>
  {/if}
</div>
