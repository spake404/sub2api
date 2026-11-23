<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <ModelPlazaContent :response="response" :loading="false" />
    </main>
  </div>
</template>

<script setup lang="ts">
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import { modelPlazaFixtureResponse } from '@/components/modelPlaza/videoReferenceFixture'

const response = modelPlazaFixtureResponse()
</script>
