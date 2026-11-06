<template>
  <div class="img-field">
    <div v-if="modelValue" class="img-field-preview">
      <QuestionImage :name="modelValue" :thumb="true" :size="72" />
      <div class="img-field-actions">
        <a-upload
          :show-upload-list="false"
          :accept="IMAGE_ACCEPT"
          :before-upload="beforeUpload"
          :custom-request="customRequest"
        >
          <a-button size="small" :loading="uploading">{{ $t('image.replace') }}</a-button>
        </a-upload>
        <a-button size="small" type="text" danger @click="clear">{{ $t('image.remove') }}</a-button>
      </div>
    </div>
    <a-upload
      v-else
      :show-upload-list="false"
      :accept="IMAGE_ACCEPT"
      :before-upload="beforeUpload"
      :custom-request="customRequest"
    >
      <a-button size="small" :loading="uploading">{{ $t('image.upload') }}</a-button>
    </a-upload>
    <div v-if="hint" class="img-field-hint">{{ hint }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import type { UploadRequestOption } from 'ant-design-vue/es/vc-upload/interface'
import QuestionImage from './QuestionImage.vue'
import { IMAGE_ACCEPT, MAX_IMAGE_BYTES, imagesApi } from '@/api/images'

const { t } = useI18n()

withDefaults(defineProps<{ modelValue?: string; hint?: string }>(), {
  modelValue: '',
  hint: '',
})
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const uploading = ref(false)

function beforeUpload(file: File) {
  if (!IMAGE_ACCEPT.split(',').includes(file.type)) {
    message.warning(t('image.onlyTypes'))
    return false
  }
  if (file.size > MAX_IMAGE_BYTES) {
    message.warning(t('image.tooLarge'))
    return false
  }
  return true
}

async function customRequest(options: UploadRequestOption) {
  uploading.value = true
  try {
    const info = await imagesApi.upload(options.file as File)
    emit('update:modelValue', info.filename)
    options.onSuccess?.(info as unknown as Record<string, unknown>)
    message.success(info.deduped ? t('image.exists') : t('image.uploaded'))
  } catch (err) {
    options.onError?.(err as Error)
  } finally {
    uploading.value = false
  }
}

function clear() {
  emit('update:modelValue', '')
}
</script>

<style scoped>
.img-field {
  display: inline-flex;
  flex-direction: column;
  gap: 6px;
}
.img-field-preview {
  display: flex;
  align-items: flex-end;
  gap: 10px;
}
.img-field-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}
.img-field-hint {
  font-size: 12px;
  color: var(--text2);
}
</style>
