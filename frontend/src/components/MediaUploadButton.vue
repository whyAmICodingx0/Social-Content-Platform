<script setup>
import { ref, computed, watch } from 'vue'
import { useMediaUpload } from '../composables/useMediaUpload'
import { ACCEPTED_IMAGE_TYPES } from '../utils/compressImage'
import { ACCEPTED_VIDEO_TYPES } from '../utils/checkVideo'

/**
 * 文章媒體上傳（P5-3 的 ImageUploadButton 一般化，P5-4）：按鈕 + 狀態 + 重試。
 * 只負責「把檔案變成一筆 media」；圖片插進內文、影片列為附件，由編輯器決定（決策 #163）。
 */
const props = defineProps({
  // 'post_image' | 'post_video'。只在建立時讀一次（composable 綁定一個 purpose）
  purpose: { type: String, required: true },
  // 編輯器正在存檔、或媒體數已達上限時停用
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['uploaded', 'busy'])

const isVideo = props.purpose === 'post_video'
const label = isVideo ? '附加影片' : '插入圖片'
const accept = (isVideo ? ACCEPTED_VIDEO_TYPES : ACCEPTED_IMAGE_TYPES).join(',')

// 解構出來的是頂層 ref，template 裡會自動取 .value
const { stage, progress, error, busy, start, retry } = useMediaUpload(props.purpose)

const fileInput = ref(null)
// 重試成功時也要知道原本的檔名（圖片拿來當 alt 文字）
let currentFileName = ''

watch(busy, (value) => emit('busy', value))

const statusText = computed(() => {
  switch (stage.value) {
    case 'compressing':
      return '壓縮中…'
    case 'checking':
      return '檢查影片…'
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
  <span class="media-upload">
    <input
      ref="fileInput"
      type="file"
      class="media-upload__input"
      :accept="accept"
      @change="onFileChange"
    />
    <button
      type="button"
      class="media-upload__button"
      :disabled="busy || disabled"
      @click="pickFile"
    >
      {{ busy ? '處理中…' : label }}
    </button>

    <span v-if="busy" class="media-upload__status" role="status">
      {{ statusText }}
      <span
        v-if="stage === 'uploading'"
        class="media-upload__bar"
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="Math.round(progress * 100)"
      >
        <span class="media-upload__fill" :style="{ width: `${Math.round(progress * 100)}%` }"></span>
      </span>
    </span>

    <span v-else-if="stage === 'error' && error" class="media-upload__error" role="alert">
      {{ error.message }}
      <button
        v-if="error.retryable"
        type="button"
        class="media-upload__retry"
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
.media-upload {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}

.media-upload__input {
  display: none;
}

/* 外觀比照分頁按鈕（.editor__tab），看起來是同一組工具列 */
.media-upload__button {
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.media-upload__button:hover:not(:disabled) {
  border-color: var(--color-accent);
  color: var(--color-accent);
}

.media-upload__button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.media-upload__status {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

/* 進度條：同 AvatarUploader，外框是軌道、內層寬度代表進度 */
.media-upload__bar {
  display: inline-block;
  width: 80px;
  height: 6px;
  border-radius: 3px;
  background: var(--color-border);
  overflow: hidden;
}

.media-upload__fill {
  display: block;
  height: 100%;
  background: var(--color-accent);
  transition: width 0.15s linear;
}

.media-upload__error {
  font-size: var(--text-sm);
  color: var(--color-danger);
}

.media-upload__retry {
  margin-left: var(--space-2);
  padding: 0;
  color: var(--color-accent);
  font: inherit;
  text-decoration: underline;
}

.media-upload__retry:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>