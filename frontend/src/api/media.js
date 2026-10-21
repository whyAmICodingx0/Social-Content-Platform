import { api } from './client'

/**
 * 媒體上傳（P5-2）。
 *
 * 這個檔案裡有兩種請求，刻意分開處理：
 *
 *  1. 對自己後端的請求（簽章、回報）→ 一律經 client.js（決策 #38）
 *  2. 直傳 Cloudinary → 用 XMLHttpRequest，不經 client.js（決策 #148）
 *     - fetch 沒有「上傳進度」事件，XHR 有（xhr.upload.onprogress）
 *     - 這不是我們的 API：沒有 { data } / { error } 的回應合約
 *     - 絕不能帶我們的 cookie 出去
 */
export const mediaApi = {
  sign: (purpose) => api.post('/media/signature', { purpose }),
  report: ({ purpose, asset_id, public_id }) =>
    api.post('/media', { purpose, asset_id, public_id }),
}

export class CloudinaryUploadError extends Error {
  constructor(message, { retryable, status = 0 }) {
    super(message)
    this.name = 'CloudinaryUploadError'
    this.retryable = retryable
    this.status = status
  }
}

// 圖片壓縮後通常不到 1MB，2 分鐘很寬裕。影片的逾時由呼叫端另外指定（P5-4）
const DEFAULT_TIMEOUT_MS = 120_000

/**
 * 把 blob 直接上傳到 Cloudinary。
 *
 * @param {object} ticket  POST /media/signature 回應的 data：{ upload_url, fields, max_bytes, ... }
 * @param {Blob} blob      要上傳的檔案（圖片是壓縮後的 JPEG；影片是原檔）
 * @param {object} [opts]
 * @param {(ratio: number) => void} [opts.onProgress]  0 ~ 1
 * @param {string} [opts.fileName]   送給 Cloudinary 的檔名（只影響 Media Library 顯示的原始檔名）
 * @param {number} [opts.timeoutMs]  逾時毫秒數
 * @returns {Promise<object>} Cloudinary 的回應（含 asset_id、public_id，重送時含 existing: true）
 */
export function uploadToCloudinary(
  ticket,
  blob,
  { onProgress, fileName = 'upload.jpg', timeoutMs = DEFAULT_TIMEOUT_MS } = {},
) {
  return new Promise((resolve, reject) => {
    const form = new FormData()

    // ★ fields 原封不動逐一放進去（P5-1 §0.13）。
    //   簽章涵蓋的每個參數都在裡面，前端不需要知道有哪些——少送或改一個字都會 Invalid Signature。
    for (const [key, value] of Object.entries(ticket.fields)) {
      form.append(key, value)
    }
    form.append('file', blob, fileName)

    const xhr = new XMLHttpRequest()
    xhr.open('POST', ticket.upload_url)
    xhr.timeout = timeoutMs

    // 跨來源的 XHR 預設就不帶 cookie。明確寫出來表達意圖：我們的 sid 絕不能送到第三方。
    xhr.withCredentials = false

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(e.loaded / e.total)
    }

    xhr.onload = () => {
      let body = null
      try {
        body = JSON.parse(xhr.responseText)
      } catch {
        // 不是 JSON（例如代理伺服器的錯誤頁），下面當成失敗處理
      }

      if (xhr.status >= 200 && xhr.status < 300 && body?.asset_id && body?.public_id) {
        resolve(body)
        return
      }

      // 5xx 與 429（Cloudinary 自己的限流）是暫時性的，可以重試；
      // 其他 4xx（格式不符、簽章錯誤）重試也不會變好。
      const retryable = xhr.status >= 500 || xhr.status === 429
      const message = body?.error?.message ?? `上傳失敗（HTTP ${xhr.status}）`
      reject(new CloudinaryUploadError(message, { retryable, status: xhr.status }))
    }

    // 網路中斷、CORS 被擋、請求被封鎖都會走這裡（瀏覽器不告訴 JS 是哪一種）
    xhr.onerror = () => {
      reject(new CloudinaryUploadError('上傳連線中斷', { retryable: true }))
    }
    xhr.ontimeout = () => {
      reject(new CloudinaryUploadError('上傳逾時', { retryable: true }))
    }

    xhr.send(form)
  })
}