import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn().mockResolvedValue(true) })
}))
import zh from '@/i18n/locales/zh/dashboard'
import en from '@/i18n/locales/en/dashboard'
import { seedanceModels, SEEDANCE_SOURCE_URL } from '../videoReferenceFixture'
import type { PlazaModel } from '@/api/modelPlaza'
import { defaultPlazaCurrencySettings, type PlazaPriceDisplay } from '@/utils/plazaCurrency'

enableAutoUnmount(afterEach)

function render(models: PlazaModel[], locale: 'zh' | 'en', rate = 0.5) {
  return mount(PlazaModelPricingTable, {
    props: { models, rateMultiplier: rate },
    global: { plugins: [createI18n({ legacy: false, locale, messages: { zh, en } })] }
  })
}

describe('PlazaModelPricingTable video reference prices', () => {
  it('shows the original estimate amounts with a dollar sign and does not scale them', () => {
    const priced = seedanceModels.filter((model) => model.video_reference)
    const wrapper = render(priced, 'zh', 0.5)
    const text = wrapper.text()
    for (const amount of ['4.97', '12.39', '7.56', '18.71', '0.37', '0.80', '0.23', '0.50', '0.46', '0.99', '2.48', '0.67', '1.51', '3.74']) {
      expect(text).toContain(`$${amount}`)
      expect(text).not.toContain(`¥${amount}`)
    }
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
    expect(text).toContain('视频参考按本站美元单位展示')
    expect(text).toContain('按本站美元单位展示，沿用原始人民币估算数值，未作汇率换算；非原厂美元报价。')
    expect(text).not.toContain('$4,970,000')
    // 客户价 10 × 0.5，参考数字 4.97 不乘倍率。
    expect(wrapper.text()).toContain('$5.00')
    const references = wrapper.findAll('[data-video-reference]').map((cell) => cell.text())
    expect(references.join('\n')).toContain('$4.97')
    expect(references.join('\n')).toContain('$0.46')
    expect(references.every((cell) => !cell.includes('¥'))).toBe(true)
    const english = render(priced.slice(0, 1), 'en', 1).text()
    expect(english).toContain("Shown in this site's US dollar unit using the original renminbi estimate figures, with no exchange-rate conversion; not the provider's US dollar price.")
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
    expect(wrapper.text()).toContain('/ 1M tokens')
    expect(wrapper.find('[data-video-reference-footnote]').exists()).toBe(false)
  })

  it('does not convert the CNY video estimate through the plaza USD rate', () => {
    const settings = {
      ...defaultPlazaCurrencySettings(),
      official_currency: 'CNY' as const,
      customer_currency: 'CNY' as const
    }
    const priceDisplay: PlazaPriceDisplay = { usdToCny: 7, settings }
    const token: PlazaModel = {
      ...seedanceModels[0],
      name: 'gpt-5.4',
      video_reference: null,
      pricing: {
        ...seedanceModels[0].pricing!,
        billing_mode: 'token',
        input_price: 1e-6,
        output_price: 1e-6,
        per_request_price: null
      },
      official_pricing: {
        input_price: 1e-6,
        output_price: 1e-6,
        cache_write_price: null,
        cache_read_price: null
      }
    }
    const wrapper = mount(PlazaModelPricingTable, {
      props: { models: [token, seedanceModels[0]], rateMultiplier: 1, priceDisplay },
      global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh, en } })] }
    })
    const text = wrapper.text()
    expect(text).toContain('¥7.00')
    expect(text).toContain('$4.97')
    expect(text).not.toContain('¥4.97')
    expect(text).not.toContain('$34.79')
    expect(text).not.toContain('¥34.79')
    expect(wrapper.find('[data-video-reference]').text()).toContain('$4.97')
    expect(wrapper.find('[data-video-reference]').text()).not.toContain('¥')
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
