import type { PlazaGroupInsertAfter } from '@/api/modelPlaza'

/** 排序只读取这些字段。groups.sort_order 不在其中，全 0 不会打乱旧顺序。 */
export interface PlazaGroupOrderInput {
  id: number
  name: string
  rate_multiplier: number
  /** 含显式 0。null / undefined 才回退到分组倍率。 */
  user_rate_multiplier?: number | null
}

/** 生效倍率 = 用户专属倍率 ?? 分组默认倍率。 */
export function plazaEffectiveRate(group: PlazaGroupOrderInput): number {
  return group.user_rate_multiplier ?? group.rate_multiplier
}

/**
 * 模型广场分组的展示顺序。
 * 未配置插入规则时与原先一致：生效倍率升序，同倍率按名称的 localeCompare，稳定排序保留并列输入顺序。
 * 规则只把指定分组挪到锚点后面，其余分组的相对顺序保持这次排序的结果。
 */
export function orderPlazaGroups<T extends PlazaGroupOrderInput>(
  groups: readonly T[],
  insertAfter?: readonly PlazaGroupInsertAfter[] | null
): T[] {
  const ordered = [...groups].sort(
    (a, b) => plazaEffectiveRate(a) - plazaEffectiveRate(b) || a.name.localeCompare(b.name)
  )
  for (const rule of insertAfter ?? []) {
    if (!rule || rule.group_id === rule.after_group_id) continue
    const from = ordered.findIndex((group) => group.id === rule.group_id)
    if (from < 0 || ordered.findIndex((group) => group.id === rule.after_group_id) < 0) continue
    const [item] = ordered.splice(from, 1)
    const anchor = ordered.findIndex((group) => group.id === rule.after_group_id)
    if (anchor < 0 || !item) continue
    ordered.splice(anchor + 1, 0, item)
  }
  return ordered
}
