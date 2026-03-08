<script lang="ts">
  import { OpenFolder } from '../../wailsjs/go/main/App'

  export let path: string = ''
  export let size: 'xs' | 'sm' = 'sm'
  export let title: string = 'フォルダを開く'

  const sizeMap = {
    xs: { btn: 'btn-xs', icon: 'h-3 w-3' },
    sm: { btn: 'btn-xs', icon: 'h-4 w-4' },
  }

  $: s = sizeMap[size]
</script>

{#if path}
  <button
    class="btn btn-ghost {s.btn}"
    {title}
    on:click|stopPropagation={() => OpenFolder(path)}
  >
    <svg xmlns="http://www.w3.org/2000/svg" class={s.icon} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
    </svg>
  </button>
{/if}
