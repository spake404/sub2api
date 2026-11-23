import { describe, expect, it } from 'vitest'
import { orderPlazaGroups, plazaEffectiveRate } from '../orderPlazaGroups'

interface Row {
  id: number
  name: string
  rate_multiplier: number
  user_rate_multiplier?: number | null
  sort_order?: number
}

const site: Row[] = [
  { id: 90, name: '高价', rate_multiplier: 2, sort_order: 0 },
  { id: 55, name: '视频分组', rate_multiplier: 1, sort_order: 0 },
  { id: 46, name: 'Grok分组', rate_multiplier: 0.2, sort_order: 0 },
  { id: 53, name: 'GPT特惠分组0.2', rate_multiplier: 0.2, sort_order: 0 },
  { id: 7, name: '低价', rate_multiplier: 0.1, sort_order: 50 }
]

const ids = (rows: Row[]) => rows.map((row) => row.id)

describe('orderPlazaGroups', () => {
  it('keeps rate then name order when presentation rules are unset', () => {
    expect(ids(orderPlazaGroups(site))).toEqual([7, 53, 46, 55, 90])
    expect(ids(orderPlazaGroups(site, []))).toEqual([7, 53, 46, 55, 90])
    expect(ids(orderPlazaGroups(site, null))).toEqual([7, 53, 46, 55, 90])
    expect(site.map((row) => row.rate_multiplier)).toEqual([2, 1, 0.2, 0.2, 0.1])
  })

  it('places group 55 directly after group 53 and before Grok without moving anyone else', () => {
    const ordered = orderPlazaGroups(site, [{ group_id: 55, after_group_id: 53 }])
    expect(ids(ordered)).toEqual([7, 53, 55, 46, 90])
    expect(ordered.map((row) => row.rate_multiplier)).toEqual([0.1, 0.2, 1, 0.2, 2])
    expect(ordered.find((row) => row.id === 55)?.sort_order).toBe(0)
  })

  it('applies a personal multiplier before the insertion and keeps an explicit zero', () => {
    const rows: Row[] = site.map((row) =>
      row.id === 46 ? { ...row, user_rate_multiplier: 0.05 } : row.id === 7 ? { ...row, user_rate_multiplier: 0 } : row
    )
    expect(plazaEffectiveRate(rows.find((row) => row.id === 7)!)).toBe(0)
    expect(ids(orderPlazaGroups(rows))).toEqual([7, 46, 53, 55, 90])
    expect(ids(orderPlazaGroups(rows, [{ group_id: 55, after_group_id: 53 }]))).toEqual([7, 46, 53, 55, 90])
  })

  it('keeps tied names in their incoming order and skips a missing anchor', () => {
    const tied: Row[] = [
      { id: 2, name: 'same', rate_multiplier: 1 },
      { id: 1, name: 'same', rate_multiplier: 1 }
    ]
    expect(ids(orderPlazaGroups(tied))).toEqual([2, 1])
    const withoutAnchor = site.filter((row) => row.id !== 53)
    expect(ids(orderPlazaGroups(withoutAnchor, [{ group_id: 55, after_group_id: 53 }]))).toEqual([7, 46, 55, 90])
  })
})
