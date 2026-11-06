<template>
  <div v-if="name" class="q-media" :class="{ thumb }">
    <a-image
      :src="url"
      :fallback="fallback"
      :width="thumb ? size : undefined"
      :height="thumb ? size : undefined"
      :preview="preview ? { src: url } : false"
      class="q-media-img"
    />
  </div>
</template>

<script setup lang="ts">
import { translate } from '@/i18n'
import { computed } from 'vue'
import { resolveImageUrl } from '@/api/request'

const props = withDefaults(
  defineProps<{
    name?: string | null
    thumb?: boolean
    size?: number
    preview?: boolean
  }>(),
  { thumb: false, size: 72, preview: true },
)

const url = computed(() => resolveImageUrl(props.name))

function escapeXml(value: string) {
  return value.replace(/[<>&"']/g, (c) => `&#${c.charCodeAt(0)};`)
}

const fallback = computed(() => {
  const raw = props.name ?? ''
  const limit = props.thumb ? 10 : 22
  const label = raw.length > limit ? `${raw.slice(0, limit)}…` : raw
  const width = props.thumb ? 72 : 320
  const height = props.thumb ? 72 : 180
  const fontSize = props.thumb ? 10 : 14
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}"><rect width="100%" height="100%" fill="#f4f6f5"/><text x="50%" y="47%" font-size="${fontSize}" fill="#8a938e" text-anchor="middle">${escapeXml(translate('image.missing'))}</text><text x="50%" y="${props.thumb ? 68 : 64}%" font-size="${props.thumb ? 8 : 11}" fill="#a5ada9" text-anchor="middle">${escapeXml(label)}</text></svg>`
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
})
</script>

<style scoped>
.q-media {
  display: inline-block;
  max-width: 100%;
  vertical-align: middle;
}
.q-media :deep(.ant-image) {
  max-width: 100%;
}
.q-media :deep(.ant-image-img) {
  max-width: 100%;
  border-radius: 8px;
  object-fit: contain;
}
.q-media:not(.thumb) :deep(.ant-image-img) {
  max-height: 420px;
  width: auto;
}
.q-media.thumb :deep(.ant-image-img) {
  border-radius: 6px;
  object-fit: cover;
  cursor: zoom-in;
}
</style>
