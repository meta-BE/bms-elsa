<script lang="ts">
  import { OpenFolder } from '../../wailsjs/go/main/App'
  import Icon from './Icon.svelte'

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
    <Icon name="folder" cls={s.icon} />
  </button>
{/if}
