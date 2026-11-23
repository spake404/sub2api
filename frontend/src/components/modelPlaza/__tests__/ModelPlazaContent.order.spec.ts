import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import { modelPlazaFixtureResponse } from '../videoReferenceFixture'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: false })
}))

vi.mock('../PlazaFilterBar.vue', () => ({
  default: {
    name: 'PlazaFilterBar',
    props: ['groups'],
    template: '<div data-filter-groups>{{ groups.map((group) => group.id).join(",") }}</div>'
  }
}))

vi.mock('../PlazaGroupSection.vue', () => ({
  default: {
    name: 'PlazaGroupSection',
    props: ['group'],
    template: '<section :data-plaza-group-id="group.id" />'
  }
}))

describe('ModelPlazaContent presentation order', () => {
  it('uses the same order for the group filter and the group sections', () => {
    const wrapper = mount(ModelPlazaContent, {
      props: { response: modelPlazaFixtureResponse(), loading: false }
    })
    const expected = '7,53,55,46,90'
    expect(wrapper.get('[data-filter-groups]').text()).toBe(expected)
    expect(wrapper.findAll('[data-plaza-group-id]').map((node) => node.attributes('data-plaza-group-id')).join(',')).toBe(expected)
    const video = modelPlazaFixtureResponse().groups.find((group) => group.id === 55)
    expect(video?.rate_multiplier).toBe(1)
    expect(video?.models.find((model) => model.name === 'seedance-2.0-720p')?.pricing?.per_request_price).toBe(10)
  })

  it('falls back to rate then name when no insertion rule is set', () => {
    const response = modelPlazaFixtureResponse()
    response.group_insert_after = []
    const wrapper = mount(ModelPlazaContent, { props: { response, loading: false } })
    expect(wrapper.get('[data-filter-groups]').text()).toBe('7,53,46,55,90')
    expect(wrapper.findAll('[data-plaza-group-id]').map((node) => node.attributes('data-plaza-group-id')).join(',')).toBe('7,53,46,55,90')
  })
})
