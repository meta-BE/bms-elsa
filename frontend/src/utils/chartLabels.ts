export function modeLabel(mode: number): string {
  const labels: Record<number, string> = { 5: '5K', 7: '7K', 9: 'PMS', 10: '10K', 14: '14K', 25: '24K' }
  return labels[mode] || `${mode}K`
}

export function diffLabel(diff: number): string {
  const labels = ['', 'BEG', 'NOR', 'HYP', 'ANO', 'INS']
  return labels[diff] || ''
}
