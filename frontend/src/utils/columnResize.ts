import type { ColumnSizingState } from '@tanstack/svelte-table'
import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App'

/** ビュー識別キー */
export type ViewId = 'chartList' | 'songList' | 'difficultyTable' | 'diffImport'

interface ColumnResizeConfig {
  /** ビュー識別キー */
  viewId: ViewId
  /** リサイズ可能（= flex）カラムのIDリスト */
  resizableColumnIds: string[]
  /** リサイズ不可カラムの固定幅合計（px） */
  fixedColumnsWidth: number
}

/**
 * config.jsonからカラム幅の割合を読み込み、ColumnSizingStateに変換する。
 * - configにキーがない場合: null（flex初期化にフォールバック）
 * - キー集合が不一致の場合: configから削除してnull
 */
export async function loadColumnWidths(
  config: ColumnResizeConfig,
  containerWidth: number,
): Promise<ColumnSizingState | null> {
  const cfg = await GetConfig()
  const saved = cfg.columnWidths?.[config.viewId]
  if (!saved) return null

  // キー集合の整合チェック
  const savedKeys = Object.keys(saved).sort()
  const expectedKeys = [...config.resizableColumnIds].sort()
  if (savedKeys.length !== expectedKeys.length || savedKeys.some((k, i) => k !== expectedKeys[i])) {
    const columnWidths = { ...cfg.columnWidths }
    delete columnWidths[config.viewId]
    await SaveConfig({ ...cfg, columnWidths })
    return null
  }

  const available = Math.max(0, containerWidth - config.fixedColumnsWidth)
  const sizing: ColumnSizingState = {}
  for (const [id, ratio] of Object.entries(saved)) {
    sizing[id] = Math.round(ratio * available)
  }
  return sizing
}

/**
 * 現在のColumnSizingStateを割合に変換してconfig.jsonに保存する。
 */
export async function saveColumnWidths(
  viewId: ViewId,
  columnSizing: ColumnSizingState,
  resizableColumnIds: string[],
  fixedColumnsWidth: number,
  containerWidth: number,
): Promise<void> {
  const available = Math.max(1, containerWidth - fixedColumnsWidth)
  const ratios: Record<string, number> = {}
  for (const id of resizableColumnIds) {
    const px = columnSizing[id]
    if (px != null) {
      ratios[id] = Math.round((px / available) * 10000) / 10000
    }
  }

  const cfg = await GetConfig()
  const columnWidths = { ...cfg.columnWidths, [viewId]: ratios }
  await SaveConfig({ ...cfg, columnWidths })
}

/**
 * 保存済み割合とコンテナ幅からColumnSizingStateを再計算する。
 * ウィンドウリサイズ時に使用。
 */
export function recalcFromRatios(
  ratios: Record<string, number>,
  fixedColumnsWidth: number,
  containerWidth: number,
): ColumnSizingState {
  const available = Math.max(0, containerWidth - fixedColumnsWidth)
  const sizing: ColumnSizingState = {}
  for (const [id, ratio] of Object.entries(ratios)) {
    sizing[id] = Math.round(ratio * available)
  }
  return sizing
}

/**
 * ColumnSizingStateから割合マップを算出する（メモリ上の保持用）。
 */
export function toRatios(
  columnSizing: ColumnSizingState,
  resizableColumnIds: string[],
  fixedColumnsWidth: number,
  containerWidth: number,
): Record<string, number> {
  const available = Math.max(1, containerWidth - fixedColumnsWidth)
  const ratios: Record<string, number> = {}
  for (const id of resizableColumnIds) {
    const px = columnSizing[id]
    if (px != null) {
      ratios[id] = Math.round((px / available) * 10000) / 10000
    }
  }
  return ratios
}
