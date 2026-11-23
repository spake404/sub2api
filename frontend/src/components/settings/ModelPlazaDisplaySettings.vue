<template>
  <div class="space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700">
    <div>
      <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.settings.features.modelPlaza.referenceTitle') }}</h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.modelPlaza.referenceHint') }}</p>
    </div>

    <p v-if="loadError" class="text-sm text-red-600 dark:text-red-300">{{ t('admin.settings.features.modelPlaza.referenceFailed') }}</p>
    <template v-else-if="view">
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.features.modelPlaza.referenceSource') }}：
        <span class="font-medium text-gray-800 dark:text-gray-100">{{ sourceLabel }}</span>
        <span v-if="view.invalid_reason"> — {{ view.invalid_reason }}</span>
      </p>
      <p v-if="view.video_reference_source === 'invalid'" class="text-xs text-amber-700 dark:text-amber-300">
        {{ t('admin.settings.features.modelPlaza.referenceInvalid') }}
      </p>

      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="useBuiltin" type="checkbox" class="rounded border-gray-300" />
        {{ t('admin.settings.features.modelPlaza.referenceUseBuiltin') }}
      </label>

      <div v-if="!useBuiltin">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.features.modelPlaza.referenceCatalog') }}</label>
        <textarea v-model="catalogText" rows="8" class="input mt-1 font-mono text-xs" spellcheck="false"></textarea>
      </div>

      <div>
        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.features.modelPlaza.referenceInsert') }}</div>
        <p class="mb-2 mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.features.modelPlaza.referenceInsertHint') }}</p>
        <div v-for="(rule, index) in rules" :key="index" class="mb-2 flex flex-wrap items-center gap-2">
          <input v-model.number="rule.group_id" type="number" min="1" class="input w-32" :placeholder="t('admin.settings.features.modelPlaza.referenceGroup')" />
          <input v-model.number="rule.after_group_id" type="number" min="1" class="input w-32" :placeholder="t('admin.settings.features.modelPlaza.referenceAfter')" />
          <button type="button" class="text-xs text-red-600" @click="rules.splice(index, 1)">{{ t('admin.settings.features.modelPlaza.referenceRemove') }}</button>
        </div>
        <button type="button" class="text-sm text-primary-600" @click="rules.push({ group_id: 0, after_group_id: 0 })">
          {{ t('admin.settings.features.modelPlaza.referenceAdd') }}
        </button>
      </div>

      <div class="flex items-center gap-3">
        <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? t('admin.settings.features.modelPlaza.referenceSaving') : t('admin.settings.features.modelPlaza.referenceSave') }}
        </button>
        <span v-if="saved" class="text-xs text-green-600 dark:text-green-300">{{ t('admin.settings.features.modelPlaza.referenceSaved') }}</span>
        <span v-if="saveError" class="text-xs text-red-600 dark:text-red-300">{{ saveError }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getModelPlazaDisplay,
  updateModelPlazaDisplay,
  type ModelPlazaDisplayView,
  type PlazaGroupInsertAfterInput,
  type PlazaVideoReferenceCatalogInput
} from '@/api/admin/modelPlazaDisplay'

const { t } = useI18n()
const view = ref<ModelPlazaDisplayView | null>(null)
const useBuiltin = ref(true)
const catalogText = ref('')
const rules = ref<PlazaGroupInsertAfterInput[]>([])
const loadError = ref(false)
const saving = ref(false)
const saved = ref(false)
const saveError = ref('')

const sourceLabel = computed(() => {
  if (view.value?.video_reference_source === 'override') return t('admin.settings.features.modelPlaza.referenceOverride')
  if (view.value?.video_reference_source === 'invalid') return t('admin.settings.features.modelPlaza.referenceInvalid')
  return t('admin.settings.features.modelPlaza.referenceBuiltin')
})

function applyView(next: ModelPlazaDisplayView) {
  view.value = next
  useBuiltin.value = next.video_reference_source !== 'override'
  catalogText.value = JSON.stringify(next.video_references, null, 2)
  rules.value = (next.insert_after ?? []).map((rule) => ({ ...rule }))
}

onMounted(async () => {
  if (import.meta.env.MODE === 'test') return
  try {
    applyView(await getModelPlazaDisplay())
  } catch {
    loadError.value = true
  }
})

async function save() {
  saving.value = true
  saved.value = false
  saveError.value = ''
  try {
    const insertAfter = rules.value.filter((rule) => rule.group_id > 0 && rule.after_group_id > 0)
    let override: PlazaVideoReferenceCatalogInput | null = null
    if (!useBuiltin.value) {
      override = JSON.parse(catalogText.value) as PlazaVideoReferenceCatalogInput
    }
    applyView(await updateModelPlazaDisplay({
      reset_video_references: useBuiltin.value,
      video_reference_override: override,
      insert_after: insertAfter
    }))
    saved.value = true
  } catch (error) {
    const message = error && typeof error === 'object' && 'message' in error ? String((error as { message: unknown }).message) : ''
    saveError.value = message || t('admin.settings.features.modelPlaza.referenceFailed')
  } finally {
    saving.value = false
  }
}
</script>
