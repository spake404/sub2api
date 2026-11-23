import { apiClient } from '../client'

export interface PlazaVideoReferenceRowInput {
  model: string
  resolution: string
  unit: 'per_video' | 'per_second'
  amount_cny: string
}

export interface PlazaVideoReferenceCatalogInput {
  currency: 'CNY'
  estimate: boolean
  official_bills_tokens: boolean
  source_url: string
  source_updated_on: string
  checked_at: string
  checked_timezone: string
  assumptions_zh: string
  assumptions_en: string
  rows: PlazaVideoReferenceRowInput[]
}

export interface PlazaGroupInsertAfterInput {
  group_id: number
  after_group_id: number
}

export interface ModelPlazaDisplayView {
  video_reference_source: 'builtin' | 'override' | 'invalid'
  invalid_reason?: string
  video_references: PlazaVideoReferenceCatalogInput
  insert_after: PlazaGroupInsertAfterInput[]
}

export interface ModelPlazaDisplayUpdate {
  reset_video_references: boolean
  video_reference_override?: PlazaVideoReferenceCatalogInput | null
  insert_after: PlazaGroupInsertAfterInput[]
}

export async function getModelPlazaDisplay(): Promise<ModelPlazaDisplayView> {
  const { data } = await apiClient.get<ModelPlazaDisplayView>('/admin/settings/model-plaza-display')
  return data
}

export async function updateModelPlazaDisplay(update: ModelPlazaDisplayUpdate): Promise<ModelPlazaDisplayView> {
  const { data } = await apiClient.put<ModelPlazaDisplayView>('/admin/settings/model-plaza-display', update)
  return data
}
