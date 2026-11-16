import { describe, expect, it } from 'vitest'
import type { Group, GroupPlatform } from '@/types'
import { buildMagpieImportLink } from '../magpieImport'

const endpointNames = ['chat', 'responses', 'anthropic', 'gemini']
const dashboardOrigin = 'https://dashboard.example'

function keyFor(platform: GroupPlatform, allowMessages = false) {
  return {
    id: 21,
    name: '开发密钥',
    key: 'sk-test-a+b&c=d%#',
    group_id: 9,
    group_ids: [9],
    group: { id: 9, platform, allow_messages_dispatch: allowMessages } as Group
  }
}

function parse(input: Parameters<typeof buildMagpieImportLink>[0]) {
  const result = buildMagpieImportLink(input)
  if (!('url' in result)) throw new Error(`No import link: ${result.error}`)
  const url = new URL(result.url)
  expect(url.protocol).toBe('magpie:')
  expect(url.host).toBe('import')
  return url.searchParams
}

const root = 'https://api.example/relay'
const openai = { chat: `${root}/v1`, responses: `${root}/v1` }
const compatible = { ...openai, anthropic: root }
const cases: Array<[GroupPlatform, boolean, Record<string, string>]> = [
  ['anthropic', false, { anthropic: root }],
  ['openai', false, openai],
  ['openai', true, compatible],
  ['gemini', false, { gemini: `${root}/v1beta` }],
  ['antigravity', false, { anthropic: `${root}/antigravity`, gemini: `${root}/antigravity/v1beta` }],
  ['grok', false, compatible],
  ['deepseek', false, compatible],
  ['opencode_go', false, compatible],
  ['command_code', false, compatible]
]

describe('Magpie import link', () => {
  it.each(cases)('sets endpoints for %s (Messages dispatch: %s)', (platform, allowMessages, expected) => {
    const row = keyFor(platform, allowMessages)
    const query = parse({ dashboardOrigin, baseUrl: `${root}/v1/`, siteName: '中转站', row })
    expect(Object.fromEntries([...query].filter(([k]) => endpointNames.includes(k)))).toEqual(expected)
    expect(query.get('name')).toBe('开发密钥 · 中转站')
    expect(query.get('key')).toBe(row.key)
    expect(query.get('id')).toMatch(/^sub2api-[a-f0-9]{16}-21$/)
  })

  it('imports every bound OpenAI group through the same gateway without changing identity', () => {
    const row = keyFor('openai', true)
    const groups = [row.group, { ...row.group, id: 10 }]
    const single = parse({ dashboardOrigin, baseUrl: root, row })
    const query = parse({ dashboardOrigin, baseUrl: root, row: { ...row, group_ids: [9, 10] }, groups })
    expect(Object.fromEntries([...query].filter(([k]) => endpointNames.includes(k)))).toEqual(compatible)
    expect(query.get('id')).toBe(single.get('id'))
    expect(query.getAll('key')).toEqual([row.key])
  })

  it('combines protocol endpoints from bound platforms instead of using only the primary group', () => {
    const row = keyFor('openai', true)
    const groups = [
      row.group,
      { ...keyFor('anthropic').group, id: 10 },
      { ...keyFor('gemini').group, id: 11 }
    ]
    const query = parse({ dashboardOrigin, baseUrl: root, row: { ...row, group_ids: [9, 10, 11] }, groups })
    expect(Object.fromEntries([...query].filter(([k]) => endpointNames.includes(k)))).toEqual({
      ...compatible, gemini: `${root}/v1beta`
    })
    expect(Object.fromEntries(parse({ dashboardOrigin, baseUrl: root, row: { ...row, group_ids: [11, 10, 9], group_id: 11, group: groups[2] }, groups }))).toEqual(Object.fromEntries(query))
  })

  it.each(['anthropic', 'gemini', 'openai', 'grok', 'antigravity'])('imports multiple %s groups without changing the single-group endpoints', platform => {
    const row = keyFor(platform)
    const single = parse({ dashboardOrigin, baseUrl: root, row })
    const query = parse({ dashboardOrigin, baseUrl: root, row: { ...row, group_ids: [9, 10] }, groups: [row.group, { ...row.group, id: 10 }] })
    expect(Object.fromEntries(query)).toEqual(Object.fromEntries(single))
  })

  it.each([[10, 9], [9, 9], [9, 0], [9, -1], [9, 1.5], [9, ...Array.from({ length: 20 }, (_, i) => i + 10)]])('rejects malformed bindings %j', (...ids) => {
    const row = keyFor('openai')
    expect(buildMagpieImportLink({ dashboardOrigin, row: { ...row, group_ids: ids }, groups: ids.map(id => ({ ...row.group, id })) })).toEqual({ error: 'unsupportedGroup' })
  })

  it('does not let stale primary metadata override current group restrictions', () => {
    const row = { ...keyFor('openai', true), group_ids: [9, 10] }
    const groups = [{ ...row.group, allow_messages_dispatch: false }, { ...row.group, id: 10 }]
    expect(buildMagpieImportLink({ dashboardOrigin, row, groups })).toEqual({ error: 'incompatibleGroups' })
  })

  it('does not include endpoints from an unbound group', () => {
    const row = { ...keyFor('openai'), group_ids: [9, 10] }
    const groups = [row.group, { ...row.group, id: 10 }, { ...keyFor('gemini').group, id: 11 }]
    const query = parse({ dashboardOrigin, baseUrl: root, row, groups })
    expect(Object.fromEntries([...query].filter(([k]) => endpointNames.includes(k)))).toEqual(openai)
  })

  it.each(['unknown', 'typesafe', 'composite'])('does not silently drop a bound %s group', platform => {
    const row = { ...keyFor('openai'), group_ids: [9, 10] }
    const groups = [row.group, { ...row.group, id: 10, platform }]
    expect(buildMagpieImportLink({ dashboardOrigin, row, groups })).toEqual({ error: 'unsupportedGroup' })
  })

  it('does not bypass a restricted secondary group or guess a missing group', () => {
    const row = { ...keyFor('openai'), group_ids: [9, 10] }
    expect(buildMagpieImportLink({ dashboardOrigin, row, groups: [row.group] })).toEqual({ error: 'unsupportedGroup' })
    expect(buildMagpieImportLink({ dashboardOrigin, row, groups: [row.group, { ...row.group, id: 10, claude_code_only: true }] })).toEqual({ error: 'unsupportedGroup' })
  })

  it.each(['anthropic', 'gemini'])('refuses conflicting Antigravity and %s routes in either binding order', platform => {
    const primary = keyFor(platform)
    const extra = { ...keyFor('antigravity').group, id: 10 }
    const row = { ...primary, group_ids: [9, 10] }
    const groups = [primary.group, extra]
    expect(buildMagpieImportLink({ dashboardOrigin, baseUrl: root, row, groups })).toEqual({ error: 'incompatibleGroups' })
    expect(buildMagpieImportLink({ dashboardOrigin, baseUrl: root, row: { ...row, group_id: 10, group: extra, group_ids: [10, 9] }, groups })).toEqual({ error: 'incompatibleGroups' })
  })

  it.each(['openai', 'anthropic'])('refuses Messages imports when a bound OpenAI group can reject dispatch (%s)', platform => {
    const primary = keyFor('openai')
    const extra = { ...keyFor(platform, true).group, id: 10 }
    expect(buildMagpieImportLink({ dashboardOrigin, row: { ...primary, group_ids: [9, 10] }, groups: [primary.group, extra] })).toEqual({ error: 'incompatibleGroups' })
  })

  it('keeps dedicated Antigravity imports for multiple Antigravity groups', () => {
    const row = { ...keyFor('antigravity'), group_ids: [9, 10] }
    const query = parse({ dashboardOrigin, baseUrl: root, row, groups: [row.group, { ...row.group, id: 10 }] })
    expect(query.get('anthropic')).toBe(`${root}/antigravity`)
    expect(query.get('gemini')).toBe(`${root}/antigravity/v1beta`)
  })

  it('rejects a dedicated Antigravity base for mixed platforms', () => {
    const row = { ...keyFor('openai'), group_ids: [9, 10] }
    expect(buildMagpieImportLink({ dashboardOrigin, baseUrl: `${root}/antigravity`, row, groups: [row.group, { ...keyFor('gemini').group, id: 10 }] })).toEqual({ error: 'incompatibleGroups' })
  })

  it('normalizes a dedicated Antigravity URL without adding its path twice', () => {
    const query = parse({ dashboardOrigin, baseUrl: `${root}/antigravity/v1beta/`, row: keyFor('antigravity') })
    expect(query.get('anthropic')).toBe(`${root}/antigravity`)
    expect(query.get('gemini')).toBe(`${root}/antigravity/v1beta`)
  })

  it('keeps provider IDs distinct by instance and row and stable across rename or rotation', () => {
    const row = keyFor('openai')
    const input = { dashboardOrigin, baseUrl: root, row }
    const id = parse(input).get('id')
    expect(parse({ ...input, row: { ...row, name: '新名字', key: 'sk-rotated' } }).get('id')).toBe(id)
    expect(parse({ ...input, row: { ...row, id: 22 } }).get('id')).not.toBe(id)
    expect(parse({ ...input, baseUrl: 'https://api.example/other' }).get('id')).not.toBe(id)
    expect(parse({ ...input, dashboardOrigin: 'https://other.example' }).get('id')).not.toBe(id)
  })

  it('uses the dashboard origin when there is no configured API address', () => {
    const query = parse({ dashboardOrigin, baseUrl: ' ', row: keyFor('openai') })
    expect(query.get('responses')).toBe(`${dashboardOrigin}/v1`)
  })

  it.each(['http://localhost:8484', 'http://127.0.0.1', 'http://192.168.1.50', 'http://relay.local', 'http://[::1]:8484', 'http://[fd12:3456::1]'])('allows local HTTP at %s', baseUrl => {
    const query = parse({ dashboardOrigin, baseUrl, row: keyFor('anthropic') })
    expect(query.get('anthropic')).toBe(baseUrl)
  })

  it.each([
    'http://api.example', 'http://10.example', 'http://172.32.0.1', 'http://[fe80::1]',
    'https://user:pass@api.example', 'https://a\tpi.example', `${root}?q=a`, `${root}?`, `${root}#section`,
    '//api.example', 'https:///api.example', 'javascript:void(0)'
  ])('rejects an invalid base at %s', baseUrl => {
    expect(buildMagpieImportLink({ dashboardOrigin, baseUrl, row: keyFor('openai') })).toEqual({ error: 'invalidBase' })
  })

  it.each([
    keyFor('typesafe'), keyFor('composite'), keyFor('unknown'),
    { ...keyFor('openai'), group_id: null, group: undefined },
    { ...keyFor('openai'), group_ids: [9, 10] },
    { ...keyFor('anthropic'), group: { ...keyFor('anthropic').group, claude_code_only: true } }
  ])('does not guess endpoints for an unsupported group', row => {
    expect(buildMagpieImportLink({ dashboardOrigin, row })).toEqual({ error: 'unsupportedGroup' })
  })

  it.each(['', ' \t', 'sk-test\nother', '密'.repeat(1400)])('rejects an empty, malformed or oversized key', key => {
    expect(buildMagpieImportLink({ dashboardOrigin, row: { ...keyFor('openai'), key } })).toEqual({ error: 'invalidKey' })
  })
})
