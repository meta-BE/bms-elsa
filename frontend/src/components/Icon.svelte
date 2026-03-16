<script lang="ts">
  import { icons, type IconName, type IconData } from './icons'

  export let name: IconName
  export let cls = 'w-5 h-5'

  $: icon = icons[name] as IconData
</script>

{#if icon.type === 'stroke'}
  <svg xmlns="http://www.w3.org/2000/svg" class={cls} fill="none" viewBox={icon.viewBox} stroke="currentColor">
    {#each icon.paths as path}
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width={icon.strokeWidth ?? 2} d={path.d} />
    {/each}
  </svg>
{:else}
  <svg xmlns="http://www.w3.org/2000/svg" class={cls} viewBox={icon.viewBox} fill="currentColor">
    {#each icon.paths as path}
      <path d={path.d} fill-rule={path.fillRule} clip-rule={path.clipRule} />
    {/each}
  </svg>
{/if}
