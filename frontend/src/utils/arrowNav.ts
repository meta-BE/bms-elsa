export function handleArrowNav<T>(e: KeyboardEvent, opts: {
  selected: string | null,
  items: T[],
  getKey: (item: T) => string,
  onSelect: (item: T, index: number) => void,
  scrollToIndex?: (index: number) => void,
}): void {
  if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
  if (!opts.selected) return

  const el = document.activeElement
  if (el) {
    const tag = el.tagName.toLowerCase()
    if (tag === 'input' || tag === 'textarea' || tag === 'select' || el.hasAttribute('contenteditable')) return
  }

  const currentIndex = opts.items.findIndex(item => opts.getKey(item) === opts.selected)
  if (currentIndex === -1) return

  const nextIndex = e.key === 'ArrowUp'
    ? Math.max(0, currentIndex - 1)
    : Math.min(opts.items.length - 1, currentIndex + 1)

  if (nextIndex === currentIndex) return

  e.preventDefault()
  if (el instanceof HTMLElement) el.blur()
  opts.onSelect(opts.items[nextIndex], nextIndex)
  opts.scrollToIndex?.(nextIndex)
}
