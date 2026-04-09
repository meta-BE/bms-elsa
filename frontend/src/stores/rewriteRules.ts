import { writable } from 'svelte/store'

export type RewriteRule = {
  id: number
  ruleType: string
  pattern: string
  replacement: string
  priority: number
}

export const rewriteRules = writable<RewriteRule[]>([])
