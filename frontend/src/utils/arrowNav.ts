import type { Row } from '@tanstack/svelte-table'

export function handleArrowNav(e: KeyboardEvent, opts: {
  selected: string | null,
  rows: Row<any>[],
  getKey: (original: any) => string,
  onSelect: (original: any, index: number) => void,
  scrollToIndex: (index: number) => void,
}): void {
  if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
  if (!opts.selected) return

  const el = document.activeElement
  if (el) {
    const tag = el.tagName.toLowerCase()
    if (tag === 'input' || tag === 'textarea' || tag === 'select' || el.hasAttribute('contenteditable')) return
  }

  const currentIndex = opts.rows.findIndex(r => opts.getKey(r.original) === opts.selected)
  if (currentIndex === -1) return

  const nextIndex = e.key === 'ArrowUp'
    ? Math.max(0, currentIndex - 1)
    : Math.min(opts.rows.length - 1, currentIndex + 1)

  if (nextIndex === currentIndex) return

  e.preventDefault()
  if (el instanceof HTMLElement) el.blur()
  opts.onSelect(opts.rows[nextIndex].original, nextIndex)
  opts.scrollToIndex(nextIndex)
}
