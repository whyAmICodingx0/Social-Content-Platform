import { ref, computed } from 'vue'
import { ApiError } from '../api/client'
import { mediaApi, uploadToCloudinary, CloudinaryUploadError } from '../api/media'
import { compressImage, CompressError } from '../utils/compressImage'
import { checkVideo, VideoCheckError } from '../utils/checkVideo'

/**
 * 一次上傳的完整流程（P5-0 §4.1）：
 *
 *   compressing（圖片）/ checking（影片）→ signing → uploading → reporting → done
 *                 ↘                           ↘          ↘            ↘
 *                                        error（可能可以重試）
 *
 * 重試時「從失敗的那一步接著做」，已完成的步驟不重做（決策 #149）：
 *   - 壓縮好的 blob 保留
 *   - 簽章保留（50 分鐘內）。同一個簽章重傳同一個檔案：
 *       上次其實沒傳成 → 正常上傳
 *       上次其實傳成了、只是回應沒收到 → Cloudinary 回 existing: true，不會多一個檔案（決策 #135）
 *     → 重試不會多扣配額，也不會產生重複檔案
 *   - Cloudinary 的回應保留，回報端點本身是冪等的（決策 #117）
 *
 * ⚠️ 選了「新的檔案」就必須用新簽章：舊簽章若其實已經上傳成功，
 *    拿它傳新檔案會得到 existing: true——也就是舊的那張圖。
 */

const PURPOSES = {
  avatar: { kind: 'image', maxEdge: 512 },
  post_image: { kind: 'image', maxEdge: 1600 },
  // P5-4：影片不壓縮、上傳原檔；30MB 在慢速網路上可能要好幾分鐘，所以逾時放寬到 10 分鐘
  post_video: { kind: 'video', timeoutMs: 10 * 60 * 1000 },
}

// Cloudinary 的簽章自 timestamp 起 1 小時有效（決策 #136），保留 10 分鐘餘裕
const TICKET_REUSE_MS = 50 * 60 * 1000

const BUSY_STAGES = ['compressing', 'checking', 'signing', 'uploading', 'reporting']

export function useMediaUpload(purpose) {
  const spec = PURPOSES[purpose]
  if (!spec) {
    throw new Error(`useMediaUpload: unsupported purpose "${purpose}"`)
  }

  const stage = ref('idle') // idle | compressing | checking | signing | uploading | reporting | done | error
  const progress = ref(0) // 0 ~ 1，只在 uploading 時有意義
  const error = ref(null) // { message, retryable }
  const media = ref(null) // 成功後的 Media shape

  const busy = computed(() => BUSY_STAGES.includes(stage.value))

  // 中間產物不需要響應式：畫面不顯示它們，只有重試時要用
  let file = null
  let blob = null
  let ticket = null
  let ticketAt = 0
  let uploaded = null

  /** 選了新檔案：所有中間產物歸零（含簽章，見檔頭的 ⚠️） */
  function start(newFile) {
    if (busy.value) return Promise.resolve(null)
    file = newFile
    blob = null
    ticket = null
    ticketAt = 0
    uploaded = null
    media.value = null
    return run()
  }

  /** 從失敗的那一步接著做 */
  function retry() {
    if (busy.value || stage.value !== 'error' || !error.value?.retryable) {
      return Promise.resolve(null)
    }
    return run()
  }

  async function run() {
    error.value = null
    let step = 'compressing'

    try {
      if (!blob) {
        if (spec.kind === 'image') {
          stage.value = step = 'compressing'
          const out = await compressImage(file, spec.maxEdge)
          blob = out.blob
          if (import.meta.env.DEV) {
            console.info(
              `[upload] ${purpose}: ${file.size} B → ${blob.size} B, ${out.width}×${out.height}`,
            )
          }
        } else {
          // 影片：檢查通過就直接上傳原檔（決策 #167）
          stage.value = step = 'checking'
          const { duration } = await checkVideo(file)
          blob = file
          if (import.meta.env.DEV) {
            console.info(`[upload] ${purpose}: ${file.size} B, duration ${duration ?? 'unknown'}`)
          }
        }
      }

      if (!uploaded) {
        if (!ticket || Date.now() - ticketAt > TICKET_REUSE_MS) {
          stage.value = step = 'signing'
          ticket = (await mediaApi.sign(purpose)).data
          ticketAt = Date.now()
        }

        // 前端只是提示，真正的檢查在後端反查（決策 #139）
        if (blob.size > ticket.max_bytes) {
          throw new CompressError(
            `壓縮後仍超過上限（${Math.floor(ticket.max_bytes / 1024 / 1024)}MB）`,
          )
        }

        stage.value = step = 'uploading'
        progress.value = 0
        uploaded = await uploadToCloudinary(ticket, blob, {
          onProgress: (ratio) => {
            progress.value = ratio
          },
          // 影片保留原檔名（只影響 Media Library 的顯示）；圖片一律是壓縮後的 JPEG
          fileName: spec.kind === 'video' ? file.name : 'upload.jpg',
          timeoutMs: spec.timeoutMs,
        })
        if (import.meta.env.DEV && uploaded.existing) {
          console.info('[upload] existing: true（同一個簽章重傳，沒有產生新檔案）')
        }
      }

      stage.value = step = 'reporting'
      const res = await mediaApi.report({
        purpose,
        asset_id: uploaded.asset_id,
        public_id: uploaded.public_id,
      })
      media.value = res.data
      stage.value = 'done'
      return res.data
    } catch (err) {
      error.value = describeError(err, step)
      stage.value = 'error'
      return null
    }
  }

  return { stage, progress, error, media, busy, start, retry }
}

function describeError(err, step) {
  if (err instanceof CompressError || err instanceof VideoCheckError) {
    return { message: err.message, retryable: false }
  }
  if (err instanceof CloudinaryUploadError) {
    return { message: err.message, retryable: err.retryable }
  }
  if (err instanceof ApiError) {
    if (err.status === 401) {
      return { message: '登入已過期，請重新登入後再試', retryable: false }
    }
    if (err.code === 'RATE_LIMITED') {
      return { message: '今天的上傳次數已達上限，請明天再試', retryable: false }
    }
    if (err.code === 'VALIDATION_ERROR') {
      // 回報被拒時，後端已經把 Cloudinary 上的檔案刪掉了（決策 #140），重試沒有意義
      const reason = err.details?.fields?.upload
      return { message: reason ? `上傳被拒絕：${reason}` : err.message, retryable: false }
    }
    // NETWORK_ERROR（status 0）與 5xx：暫時性問題
    if (err.status === 0 || err.status >= 500) {
      const message =
        step === 'reporting'
          ? '檔案已上傳，但伺服器暫時無法登記，請重試'
          : '暫時無法連線到伺服器，請重試'
      return { message, retryable: true }
    }
    return { message: err.message, retryable: false }
  }
  console.error('[upload] unexpected error', err)
  return { message: '發生未預期的錯誤', retryable: false }
}