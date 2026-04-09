import type { RewriteRule } from '../stores/rewriteRules'

/**
 * リライトルールをURLに適用する。
 * priority降順（ListRewriteRulesの返却順）で試行し、最初にマッチしたルールで置換。
 * マッチなしの場合は元URLをそのまま返す。
 */
export function applyRewriteRules(url: string, rules: RewriteRule[]): string {
  if (!url) return ''
  for (const rule of rules) {
    if (rule.ruleType === 'replace') {
      if (url.includes(rule.pattern)) {
        return url.replace(rule.pattern, rule.replacement)
      }
    } else if (rule.ruleType === 'regex') {
      try {
        const re = new RegExp(rule.pattern)
        if (re.test(url)) {
          return url.replace(re, rule.replacement)
        }
      } catch {
        // 不正な正規表現はスキップ
        continue
      }
    }
  }
  return url
}
