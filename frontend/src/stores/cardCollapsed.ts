import { writable, get } from 'svelte/store'
import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App'

export type PaneId = 'song' | 'chart' | 'entry'
export type CardId = 'chartInfo' | 'irInfo' | 'bmsSearch' | 'installCandidate'
export type CollapsedMap = Partial<Record<PaneId, Partial<Record<CardId, boolean>>>>

export const cardCollapsed = writable<CollapsedMap>({})

let initialized = false

// 起動時に config.json から読み込み、ストアへ反映する
export async function initCardCollapsed(): Promise<void> {
  if (initialized) return
  initialized = true
  const cfg = await GetConfig()
  cardCollapsed.set(((cfg as any).cardCollapsed as CollapsedMap | undefined) ?? {})
}

// paneId × cardId の最小化状態をトグルし、config.json へ即時保存する。
// 値が false 相当（=展開）になるケースはキー自体を削除して JSON を肥大化させない。
export async function toggleCard(paneId: PaneId, cardId: CardId): Promise<void> {
  cardCollapsed.update(curr => {
    const pane = { ...(curr[paneId] ?? {}) }
    if (pane[cardId]) {
      delete pane[cardId]
    } else {
      pane[cardId] = true
    }
    const updated: CollapsedMap = { ...curr }
    if (Object.keys(pane).length === 0) {
      delete updated[paneId]
    } else {
      updated[paneId] = pane
    }
    return updated
  })
  const cfg = await GetConfig()
  await SaveConfig({ ...cfg, cardCollapsed: get(cardCollapsed) } as any)
}
