import type { ModelPlazaGroup, ModelPlazaResponse, PlazaModel, PlazaVideoReference, PlazaVideoReferencePrice } from '@/api/modelPlaza'

export const SEEDANCE_SOURCE_URL = 'https://ark.volcengine.com/region:cn-beijing/docs/model-pricing'

const assumptions = {
  assumptions_zh: '按 16:9、输出 5 秒、无输入视频估算。官方按 token 计费，这些数字是官方公布的等价示例，不是固定收费标准。货币为人民币，未做汇率换算。',
  assumptions_en: 'Estimated for a 16:9 frame, 5 seconds of output, and no input video. The provider bills tokens; these figures are published equivalent examples, not a fixed tariff. Amounts are CNY and were not converted from another currency.'
}

export function seedanceReference(rows: PlazaVideoReferencePrice[]): PlazaVideoReference {
  return {
    currency: 'CNY',
    estimate: true,
    official_bills_tokens: true,
    source_url: SEEDANCE_SOURCE_URL,
    source_updated_on: '2026-11-19',
    checked_at: '2026-11-23',
    checked_timezone: 'Asia/Shanghai',
    ...assumptions,
    rows
  }
}

function videoRow(resolution: string, unit: PlazaVideoReferencePrice['unit'], amount: string): PlazaVideoReferencePrice {
  return { resolution, unit, amount_cny: amount }
}

/** 与后端内置目录相同的 8 个别名、14 行。金额都是人民币估算，不是客户售价。 */
export const seedanceModels: PlazaModel[] = [
  model('seedance-2.0-720p', 'per_request', 10, seedanceReference([videoRow('720p', 'per_video', '4.97')])),
  model('seedance-2.0-1080p', 'per_request', 20, seedanceReference([videoRow('1080p', 'per_video', '12.39')])),
  model('seedance-2.5-720p', 'per_request', 16, seedanceReference([videoRow('720p', 'per_video', '7.56')])),
  model('seedance-2.5-1080p', 'per_request', 30, seedanceReference([videoRow('1080p', 'per_video', '18.71')])),
  model('seedance-2.0-fast', 'video', 3, seedanceReference([
    videoRow('480p', 'per_second', '0.37'),
    videoRow('720p', 'per_second', '0.80')
  ])),
  model('seedance-2.0-mini', 'video', 2, seedanceReference([
    videoRow('480p', 'per_second', '0.23'),
    videoRow('720p', 'per_second', '0.50')
  ])),
  model('seedance-2.0-per-second', 'video', 4, seedanceReference([
    videoRow('480p', 'per_second', '0.46'),
    videoRow('720p', 'per_second', '0.99'),
    videoRow('1080p', 'per_second', '2.48')
  ])),
  model('seedance-2.5-per-second', 'video', 5, seedanceReference([
    videoRow('480p', 'per_second', '0.67'),
    videoRow('720p', 'per_second', '1.51'),
    videoRow('1080p', 'per_second', '3.74')
  ])),
  model('minimax-video-01', 'per_request', 6, null)
]

function model(
  name: string,
  billingMode: 'per_request' | 'video' | 'token',
  perRequest: number | null,
  reference: PlazaVideoReference | null
): PlazaModel {
  return {
    name,
    platform: 'openai',
    official_pricing: billingMode === 'token'
      ? { input_price: 2e-6, output_price: 8e-6, cache_write_price: null, cache_read_price: null }
      : { input_price: null, output_price: null, cache_write_price: null, cache_read_price: null },
    video_reference: reference,
    pricing: {
      billing_mode: billingMode,
      input_price: billingMode === 'token' ? 3e-6 : null,
      output_price: billingMode === 'token' ? 1.2e-5 : null,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: perRequest,
      intervals: []
    }
  }
}

function group(partial: Pick<ModelPlazaGroup, 'id' | 'name' | 'rate_multiplier' | 'models'> & Partial<ModelPlazaGroup>): ModelPlazaGroup {
  return {
    description: '',
    platform: 'openai',
    subscription_type: 'standard',
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    long_context_pricing_enabled: false,
    ...partial
  }
}

/** 本地夹具：分组 55 紧跟分组 53、位于 Grok 之前。倍率保持原值。 */
export function modelPlazaFixtureResponse(): ModelPlazaResponse {
  return {
    description: '',
    group_insert_after: [{ group_id: 55, after_group_id: 53 }],
    groups: [
      group({ id: 90, name: '高价', rate_multiplier: 2, models: [model('gpt-expensive', 'token', null, null)] }),
      group({ id: 55, name: '视频分组', rate_multiplier: 1, models: seedanceModels }),
      group({ id: 46, name: 'Grok分组', rate_multiplier: 0.2, models: [model('grok-4', 'token', null, null)] }),
      group({ id: 53, name: 'GPT特惠分组0.2', rate_multiplier: 0.2, models: [model('gpt-5.4', 'token', null, null)] }),
      group({ id: 7, name: '低价', rate_multiplier: 0.1, models: [model('gpt-cheap', 'token', null, null)] })
    ]
  }
}
