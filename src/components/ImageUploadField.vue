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
          <a-button size="small" :loading="uploading">替换</a-button>
        </a-upload>
        <a-button size="small" type="text" danger @click="clear">移除</a-button>
      </div>
    </div>
    <a-upload
      v-else
      :show-upload-list="false"
      :accept="IMAGE_ACCEPT"
      :before-upload="beforeUpload"
      :custom-request="customRequest"
    >
      <a-button size="small" :loading="uploading">上传图片</a-button>
    </a-upload>
    <div v-if="hint" class="img-field-hint">{{ hint }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { message } from 'ant-design-vue'
import type { UploadRequestOption } from 'ant-design-vue/es/vc-upload/interface'
import QuestionImage from './QuestionImage.vue'
import { IMAGE_ACCEPT, MAX_IMAGE_BYTES, imagesApi } from '@/api/images'

withDefaults(defineProps<{ modelValue?: string; hint?: string }>(), {
  modelValue: '',
  hint: '',
})
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const uploading = ref(false)

function beforeUpload(file: File) {
  if (!IMAGE_ACCEPT.split(',').includes(file.type)) {
    message.warning('只支持 PNG、JPEG、WebP、GIF 图片')
    return false
  }
  if (file.size > MAX_IMAGE_BYTES) {
    message.warning('图片不能超过 8 MB')
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
    message.success(info.deduped ? '图片已存在，已直接复用' : '图片已上传')
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
