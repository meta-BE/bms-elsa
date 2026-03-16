<script lang="ts">
  import { ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime'
  import { onMount, onDestroy } from 'svelte'
  import { OpenURL } from '../../wailsjs/go/main/App'

  type MenuItem = {
    label: string
    action: () => void
    disabled: boolean
  }

  let visible = false
  let x = 0
  let y = 0
  let linkItems: MenuItem[] = []
  let editItems: MenuItem[] = []

  // メニュークリック時にはフォーカスが移動済みのため、事前に保存する
  let savedTarget: HTMLInputElement | HTMLTextAreaElement | null = null
  let savedSelectionStart = 0
  let savedSelectionEnd = 0
  let savedSelectedText = ''

  // WebKitはmousedown(button=2)→contextmenuの間にinput内テキストを全選択するため、
  // mousedown時点の選択状態を保存しておく
  let preRightClickSelStart = 0
  let preRightClickSelEnd = 0
  let preRightClickTarget: InputLike | null = null

  type InputLike = HTMLInputElement | HTMLTextAreaElement

  function isInputLike(el: Element | null): el is InputLike {
    return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement
  }

  // 編集可能な要素かどうか判定
  function isEditable(el: Element | null): boolean {
    if (!el) return false
    if (isInputLike(el)) return true
    if ((el as HTMLElement).isContentEditable) return true
    return false
  }

  // テキスト選択を取得（input/textareaは専用API、それ以外はwindow.getSelection）
  function getSelectedText(el: Element | null): string {
    if (isInputLike(el)) {
      const start = el.selectionStart ?? 0
      const end = el.selectionEnd ?? 0
      return el.value.substring(start, end)
    }
    const sel = window.getSelection()
    return sel ? sel.toString() : ''
  }

  // input/textareaの選択範囲を置換し、inputイベントを発火
  function replaceInputSelection(el: InputLike, start: number, end: number, replacement: string) {
    el.focus()
    el.setSelectionRange(start, end)
    // setRangeTextが使える場合はそちらを使用
    el.setRangeText(replacement, start, end, 'end')
    el.dispatchEvent(new Event('input', { bubbles: true }))
  }

  // 右クリックのmousedown時点でinputの選択状態を記録
  function handleMouseDown(e: MouseEvent) {
    if (e.button !== 2) return
    const el = document.activeElement
    if (isInputLike(el)) {
      preRightClickTarget = el
      preRightClickSelStart = el.selectionStart ?? 0
      preRightClickSelEnd = el.selectionEnd ?? 0
    } else {
      preRightClickTarget = null
    }
  }

  function handleContextMenu(e: MouseEvent) {
    // devモードではブラウザデフォルトメニューを表示
    if (import.meta.env.DEV) return

    e.preventDefault()

    // リンク検出
    const anchor = (e.target as Element).closest('a[href]')
    const href = anchor?.getAttribute('href') ?? ''

    const activeEl = document.activeElement

    // WebKitが全選択した場合、mousedown時の選択に復元
    if (preRightClickTarget && isInputLike(activeEl) && activeEl === preRightClickTarget) {
      activeEl.setSelectionRange(preRightClickSelStart, preRightClickSelEnd)
    }

    const editable = isEditable(activeEl)
    const selectedText = getSelectedText(activeEl)
    const selected = selectedText.length > 0

    // input/textareaの場合、状態を保存
    if (isInputLike(activeEl)) {
      savedTarget = activeEl
      savedSelectionStart = activeEl.selectionStart ?? 0
      savedSelectionEnd = activeEl.selectionEnd ?? 0
    } else {
      savedTarget = null
    }
    savedSelectedText = selectedText

    const newLinkItems: MenuItem[] = []
    if (href) {
      newLinkItems.push(
        {
          label: '開く',
          disabled: false,
          action: () => OpenURL(href),
        },
        {
          label: 'URLをコピー',
          disabled: false,
          action: async () => { await ClipboardSetText(href) },
        },
      )
    }

    const newEditItems: MenuItem[] = [
      {
        label: 'カット',
        disabled: !(selected && editable),
        action: async () => {
          await ClipboardSetText(savedSelectedText)
          if (savedTarget) {
            replaceInputSelection(savedTarget, savedSelectionStart, savedSelectionEnd, '')
          } else {
            document.execCommand('delete')
          }
        },
      },
      {
        label: 'コピー',
        disabled: !selected,
        action: async () => {
          await ClipboardSetText(savedSelectedText)
        },
      },
      {
        label: 'ペースト',
        disabled: !editable,
        action: async () => {
          const text = await ClipboardGetText()
          if (savedTarget) {
            replaceInputSelection(savedTarget, savedSelectionStart, savedSelectionEnd, text)
          } else {
            document.execCommand('insertText', false, text)
          }
        },
      },
      {
        label: '削除',
        disabled: !(selected && editable),
        action: () => {
          if (savedTarget) {
            replaceInputSelection(savedTarget, savedSelectionStart, savedSelectionEnd, '')
          } else {
            document.execCommand('delete')
          }
        },
      },
    ]

    const hasLinkItems = newLinkItems.length > 0
    const hasEditItems = newEditItems.some((i) => !i.disabled)

    // いずれも表示するものがなければメニューを出さない
    if (!hasLinkItems && !hasEditItems) return

    linkItems = newLinkItems
    editItems = hasEditItems ? newEditItems : []

    // 表示位置を計算（画面端ではみ出す場合は反転）
    const menuWidth = 160
    const totalItems = linkItems.length + editItems.length
    const separatorHeight = linkItems.length > 0 && editItems.length > 0 ? 9 : 0
    const menuHeight = totalItems * 32 + 8 + separatorHeight
    x = e.clientX + menuWidth > window.innerWidth ? e.clientX - menuWidth : e.clientX
    y = e.clientY + menuHeight > window.innerHeight ? e.clientY - menuHeight : e.clientY

    visible = true
  }

  function close() {
    visible = false
  }

  // captureフェーズでmousedownを検知してメニューを閉じる
  // （stopPropagationされたイベントでも確実に閉じるため）
  function handleGlobalMouseDown(e: MouseEvent) {
    if (!visible) return
    const menu = document.querySelector('[data-context-menu]')
    if (menu && menu.contains(e.target as Node)) return
    close()
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
  }

  // 全リスナーをdocument captureフェーズで登録
  // （showModal() top layer内のイベントも確実にキャッチするため）
  onMount(() => {
    document.addEventListener('mousedown', handleMouseDown, true)
    document.addEventListener('mousedown', handleGlobalMouseDown, true)
    document.addEventListener('contextmenu', handleContextMenu, true)
    document.addEventListener('keydown', handleKeyDown, true)
    document.addEventListener('scroll', close, true)
  })
  onDestroy(() => {
    document.removeEventListener('mousedown', handleMouseDown, true)
    document.removeEventListener('mousedown', handleGlobalMouseDown, true)
    document.removeEventListener('contextmenu', handleContextMenu, true)
    document.removeEventListener('keydown', handleKeyDown, true)
    document.removeEventListener('scroll', close, true)
  })

  function handleClick(item: MenuItem) {
    if (item.disabled) return
    item.action()
    close()
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div
    data-context-menu
    class="fixed z-[9999] bg-base-100 border border-base-300 rounded-box shadow-lg py-1 w-fit"
    style="left: {x}px; top: {y}px;"
  >
    {#each linkItems as item}
      <button
        class="block w-full text-left px-4 py-1.5 text-sm whitespace-nowrap transition-colors
          {item.disabled ? 'opacity-40 cursor-default' : 'hover:bg-primary/20 cursor-pointer'}"
        on:click={() => handleClick(item)}
        disabled={item.disabled}
      >
        {item.label}
      </button>
    {/each}
    {#if linkItems.length > 0 && editItems.length > 0}
      <div class="divider my-0 h-px"></div>
    {/if}
    {#each editItems as item}
      <button
        class="block w-full text-left px-4 py-1.5 text-sm whitespace-nowrap transition-colors
          {item.disabled ? 'opacity-40 cursor-default' : 'hover:bg-primary/20 cursor-pointer'}"
        on:click={() => handleClick(item)}
        disabled={item.disabled}
      >
        {item.label}
      </button>
    {/each}
  </div>
{/if}
