<script setup>
import { ref, computed, watch } from 'vue'
import { useAuthStore } from '../stores/auth'
import { ApiError } from '../api/client'
import { useMediaUpload } from '../composables/useMediaUpload'
import { ACCEPTED_IMAGE_TYPES } from '../utils/compressImage'

/**
 * 頭像上傳（P5-2）：上傳 + 套用（PATCH /me { avatar_media_id }）。
 * 上傳成功後立即套用，不需要按設定頁的「儲存變更」（決策 #152）。
 */
defineProps({
  // 父元件正在儲存表單時禁止上傳：兩個 PATCH /me 同時進行，後到的會蓋掉先到的（決策 #152）
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['applied', 'busy'])

const auth = useAuthStore()

// 解構出來的是頂層 ref，template 裡會自動取 .value
const {
  stage,
  progress,
  error: uploadError,
  media,
  busy: uploadBusy,
  start,
  retry: retryUpload,
} = useMediaUpload('avatar')

const fileInput = ref(null)
const applying = ref(false)
const applyError = ref(null) // { message, retryable }
const applied = ref(false)

const busy = computed(() => uploadBusy.value || applying.value)
watch(busy, (value) => emit('busy', value))

const accept = ACCEPTED_IMAGE_TYPES.join(',')

const statusText = computed(() => {
  if (applying.value) return '套用中…'
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

// 套用失敗優先顯示：那時上傳本身已經成功了
const shownError = computed(() => {
  if (applyError.value) return applyError.value
  if (stage.value === 'error') return uploadError.value
  return null
})

function pickFile() {
  fileInput.value?.click()
}

async function onFileChange(event) {
  const file = event.target.files?.[0]
  // 清空，下次選同一個檔案才會再觸發 change
  event.target.value = ''
  if (!file) return

  applyError.value = null
  applied.value = false
  const m = await start(file)
  if (m) await apply(m.id)
}

async function onRetry() {
  // 上傳已成功、只是套用失敗 → 只重送 PATCH，不重新上傳
  if (applyError.value) {
    if (media.value) await apply(media.value.id)
    return
  }
  const m = await retryUpload()
  if (m) await apply(m.id)
}

async function apply(mediaId) {
  applying.value = true
  applyError.value = null
  try {
    const user = await auth.updateProfile({ avatar_media_id: mediaId })
    applied.value = true
    emit('applied', user)
  } catch (err) {
    const retryable = err instanceof ApiError && (err.status === 0 || err.status >= 500)
    applyError.value = {
      message: retryable ? '圖片已上傳，但套用頭像失敗，請重試' : (err.message ?? '套用頭像失敗'),
      retryable,
    }
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <div class="uploader">
    <!-- 原生的檔案選擇框很難改樣式，所以藏起來，用自己的按鈕觸發它 -->
    <input
      ref="fileInput"
      type="file"
      class="uploader__input"
      :accept="accept"
      @change="onFileChange"
    />
    <button type="button" class="btn btn--ghost" :disabled="busy || disabled" @click="pickFile">
      {{ busy ? '處理中…' : '上傳新頭像' }}
    </button>

    <div v-if="busy" class="uploader__status" role="status">
      <span>{{ statusText }}</span>
      <div
        v-if="stage === 'uploading'"
        class="uploader__bar"
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="Math.round(progress * 100)"
      >
        <div class="uploader__fill" :style="{ width: `${Math.round(progress * 100)}%` }"></div>
      </div>
    </div>

    <p v-else-if="shownError" class="uploader__error" role="alert">
      {{ shownError.message }}
      <button
        v-if="shownError.retryable"
        type="button"
        class="uploader__retry"
        :disabled="disabled"
        @click="onRetry"
      >
        重試
      </button>
    </p>

    <p v-else-if="applied" class="uploader__ok">頭像已更新。</p>
  </div>
</template>

<style scoped>
/* 直向排列：按鈕在上，狀態訊息在下 */
.uploader {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-2);
  min-width: 0; /* 在 flex 父容器裡允許縮小，長錯誤訊息才會換行而不是撐破版面 */
}

.uploader__input {
  display: none;
}

.uploader .btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.uploader__status {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

/* 進度條：外框是灰色軌道，裡面的 fill 用寬度百分比表示進度 */
.uploader__bar {
  width: 160px;
  height: 6px;
  border-radius: 3px;
  background: var(--color-border);
  overflow: hidden;
}

.uploader__fill {
  height: 100%;
  background: var(--color-accent);
  transition: width 0.15s linear;
}

.uploader__error {
  font-size: var(--text-sm);
  color: var(--color-danger);
}

/* 重試做成文字連結的樣子：去掉按鈕預設的框與底色 */
.uploader__retry {
  margin-left: var(--space-2);
  padding: 0;
  border: none;
  background: none;
  color: var(--color-accent);
  font: inherit;
  text-decoration: underline;
  cursor: pointer;
}

.uploader__retry:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.uploader__ok {
  font-size: var(--text-sm);
  color: var(--color-accent);
}
</style>