<script setup>
import { ref, computed, watch } from 'vue'
import { useMediaUpload } from '../composables/useMediaUpload'
import { ACCEPTED_IMAGE_TYPES } from '../utils/compressImage'

/**
 * 文章圖片上傳（P5-3）：按鈕 + 狀態 + 重試。
 * 只負責「把圖片變成一筆 media」；插進內文、記錄 media id 由編輯器決定（決策 #163）。
 */
defineProps({
  // 編輯器正在存檔、或媒體數已達上限時停用
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['uploaded', 'busy'])

// 解構出來的是頂層 ref，template 裡會自動取 .value
const { stage, progress, error, busy, start, retry } = useMediaUpload('post_image')

const fileInput = ref(null)
// 重試成功時也要知道原本的檔名（拿來當 alt 文字）
let currentFileName = ''

watch(busy, (value) => emit('busy', value))

const accept = ACCEPTED_IMAGE_TYPES.join(',')

const statusText = computed(() => {
  switch (stage.value) {
    case 'compressing':
      return '壓縮中…'
    case 'signing':
      return '準備上傳…'
    case 'uploading':
      return `上傳中 ${Math.round(progress.value * 100)}%`
    case 'reporting':
      return '確認中…'
    default:
      return ''
  }
})

function pickFile() {
  fileInput.value?.click()
}

async function onFileChange(event) {
  const file = event.target.files?.[0]
  event.target.value = '' // 清空，下次選同一個檔案才會再觸發 change
  if (!file) return

  currentFileName = file.name
  const media = await start(file)
  if (media) emit('uploaded', media, currentFileName)
}

async function onRetry() {
  const media = await retry()
  if (media) emit('uploaded', media, currentFileName)
}
</script>

<template>
  <span class="image-upload">
    <input
      ref="fileInput"
      type="file"
      class="image-upload__input"
      :accept="accept"
      @change="onFileChange"
    />
    <button
      type="button"
      class="image-upload__button"
      :disabled="busy || disabled"
      @click="pickFile"
    >
      {{ busy ? '處理中…' : '插入圖片' }}
    </button>

    <span v-if="busy" class="image-upload__status" role="status">
      {{ statusText }}
      <span
        v-if="stage === 'uploading'"
        class="image-upload__bar"
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="Math.round(progress * 100)"
      >
        <span class="image-upload__fill" :style="{ width: `${Math.round(progress * 100)}%` }"></span>
      </span>
    </span>

    <span v-else-if="stage === 'error' && error" class="image-upload__error" role="alert">
      {{ error.message }}
      <button
        v-if="error.retryable"
        type="button"
        class="image-upload__retry"
        :disabled="disabled"
        @click="onRetry"
      >
        重試
      </button>
    </span>
  </span>
</template>

<style scoped>
/* 和「編輯／預覽」分頁放在同一列，所以整個元件是行內排列 */
.image-upload {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}

.image-upload__input {
  display: none;
}

/* 外觀比照分頁按鈕（.editor__tab），看起來是同一組工具列 */
.image-upload__button {
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.image-upload__button:hover:not(:disabled) {
  border-color: var(--color-accent);
  color: var(--color-accent);
}

.image-upload__button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.image-upload__status {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

/* 進度條：同 AvatarUploader，外框是軌道、內層寬度代表進度 */
.image-upload__bar {
  display: inline-block;
  width: 80px;
  height: 6px;
  border-radius: 3px;
  background: var(--color-border);
  overflow: hidden;
}

.image-upload__fill {
  display: block;
  height: 100%;
  background: var(--color-accent);
  transition: width 0.15s linear;
}

.image-upload__error {
  font-size: var(--text-sm);
  color: var(--color-danger);
}

.image-upload__retry {
  margin-left: var(--space-2);
  padding: 0;
  color: var(--color-accent);
  font: inherit;
  text-decoration: underline;
}

.image-upload__retry:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>