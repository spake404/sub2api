import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'
import zh from '@/i18n/locales/zh/dashboard'
import en from '@/i18n/locales/en/dashboard'
import { seedanceModels, SEEDANCE_SOURCE_URL } from '../videoReferenceFixture'
import type { PlazaModel } from '@/api/modelPlaza'

enableAutoUnmount(afterEach)

function render(models: PlazaModel[], locale: 'zh' | 'en', rate = 0.5) {
  return mount(PlazaModelPricingTable, {
    props: { models, rateMultiplier: rate },
    global: { plugins: [createI18n({ legacy: false, locale, messages: { zh, en } })] }
  })
}

describe('PlazaModelPricingTable video reference prices', () => {
  it('shows CNY per video and per second without scaling to USD per million tokens', () => {
    const wrapper = render([seedanceModels[0], seedanceModels[4], seedanceModels[6]], 'zh', 0.5)
    const text = wrapper.text()
    expect(text).toContain('¥4.97')
    expect(text).toContain('¥0.37')
    expect(text).toContain('¥0.80')
    expect(text).toContain('¥0.46')
    expect(text).toContain('¥0.99')
    expect(text).toContain('¥2.48')
    expect(text).toContain('/ 条')
    expect(text).toContain('/ 秒')
    expect(text).toContain('720p')
    expect(text).toContain('1080p')
    expect(text).toContain('估算示例，不是固定官价')
    expect(text).toContain('16:9')
    expect(text).toContain('5 秒')
    expect(text).toContain('无输入视频')
    expect(text).toContain('不是固定收费标准')
    expect(text).toContain(SEEDANCE_SOURCE_URL)
    expect(text).toContain('2026-11-19')
    expect(text).toContain('2026-11-23')
    expect(text).toContain('Asia/Shanghai')
    expect(text).toContain('视频参考为人民币估算')
    expect(text).not.toContain('$4.97')
    expect(text).not.toContain('$4,970,000')
    // 客户价 10 × 0.5，参考价 4.97 不乘倍率。
    expect(wrapper.text()).toContain('$5.00')
    const references = wrapper.findAll('[data-video-reference]').map((cell) => cell.text())
    expect(references.join('\n')).toContain('¥4.97')
    expect(references.join('\n')).toContain('¥0.46')
    expect(references.every((cell) => !cell.includes('$'))).toBe(true)
  })

  it('keeps an unsupported video alias blank and keeps token official prices in USD', () => {
    const minimax = seedanceModels.find((model) => model.name === 'minimax-video-01')!
    const token = {
      ...seedanceModels[0],
      name: 'gpt-5.4',
      video_reference: null,
      pricing: {
        ...seedanceModels[0].pricing!,
        billing_mode: 'token' as const,
        input_price: 2e-6,
        output_price: 8e-6,
        per_request_price: null
      },
      official_pricing: {
        input_price: 2e-6,
        output_price: 8e-6,
        cache_write_price: null,
        cache_read_price: null
      }
    }
    const wrapper = render([minimax, token], 'en', 1)
    const rows = wrapper.findAll('tbody tr')
    expect(rows[1].text()).not.toContain('¥')
    expect(rows[1].text()).not.toContain('¥0')
    expect(rows[1].text()).toContain('-')
    expect(rows[0].text()).toContain('$2.00')
    expect(rows[0].text()).toContain('$8.00')
    expect(wrapper.text()).toContain('$ / 1M tokens')
    expect(wrapper.find('[data-video-reference-footnote]').exists()).toBe(false)
  })

  it('does not present a non-estimate or zero amount as an official CNY rate', () => {
    const hidden = {
      ...seedanceModels[0],
      video_reference: { ...seedanceModels[0].video_reference!, estimate: false, rows: [{ resolution: '720p', unit: 'per_video' as const, amount_cny: '0.00' }] }
    }
    const wrapper = render([hidden], 'zh')
    expect(wrapper.find('[data-video-reference]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('¥0.00')
    expect(wrapper.text()).not.toContain('¥0')
  })
})
