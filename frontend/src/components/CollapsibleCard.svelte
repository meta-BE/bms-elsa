<script lang="ts">
  import { cardCollapsed, toggleCard, type PaneId, type CardId } from '../stores/cardCollapsed'
  import Icon from './Icon.svelte'

  export let paneId: PaneId
  export let cardId: CardId

  $: collapsed = $cardCollapsed[paneId]?.[cardId] === true
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between" class:mb-2={!collapsed}>
    <div class="flex items-center gap-1 min-w-0">
      <button
        class="btn btn-ghost btn-xs btn-square"
        on:click={() => toggleCard(paneId, cardId)}
        title={collapsed ? '展開' : '最小化'}
      >
        <Icon name={collapsed ? 'chevronRight' : 'chevronDown'} cls="h-4 w-4" />
      </button>
      <h3 class="text-sm font-semibold truncate">
        <slot name="title" />
      </h3>
    </div>
    {#if !collapsed}
      <slot name="actions" />
    {/if}
  </div>
  {#if !collapsed}
    <slot />
  {/if}
</div>
