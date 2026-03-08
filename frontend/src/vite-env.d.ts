/// <reference types="svelte" />
/// <reference types="vite/client" />

import type { RowData } from '@tanstack/table-core'

declare module '@tanstack/table-core' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    flex?: boolean
    align?: 'left' | 'center' | 'right'
    filterType?: string
    filterSort?: 'asc' | 'desc'
    filterOptions?: string[]
  }
}
