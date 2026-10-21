/**
 * 影片上傳前的檢查（P5-4，決策 #167）。
 *
 * 影片不在瀏覽器裡壓縮（沒有像 Canvas 那樣可靠又便宜的做法），所以上傳的是原檔。
 * 這裡只做「明顯不合格就不要浪費時間上傳」的檢查——真正的限制仍在後端反查（決策 #123、#138）。
 *
 * ⚠️ 與圖片壓縮的差別：讀不到影片長度時「放行」，不中止。
 *   - 圖片壓縮失敗必須中止，因為放行的話會改送體積大好幾倍的原檔（決策 #123）
 *   - 影片讀不到長度時，沒有「另一個更大的檔案」被送出去；後端反查仍會擋下超過 60 秒的影片
 *   - 而讀不到長度很常見：例如 iPhone 拍的 HEVC .mov，Windows 上的 Chrome 解不開
 */

// 與後端白名單一致：mp4、webm、mov（.mov 的 MIME type 是 video/quicktime）
export const ACCEPTED_VIDEO_TYPES = ['video/mp4', 'video/webm', 'video/quicktime']

// 與後端 purposeSpecs 的 post_video 相同（決策 #122）
const MAX_VIDEO_BYTES = 30 * 1024 * 1024
const MAX_VIDEO_SECONDS = 60

// 讀取長度最多等 10 秒，避免卡住
const METADATA_TIMEOUT_MS = 10_000

export class VideoCheckError extends Error {
  constructor(message) {
    super(message)
    this.name = 'VideoCheckError'
  }
}

/**
 * @param {File} file
 * @returns {Promise<{ duration: number | null }>} 讀不到長度時 duration 是 null
 */
export async function checkVideo(file) {
  if (!ACCEPTED_VIDEO_TYPES.includes(file.type)) {
    throw new VideoCheckError('只接受 MP4、WebM、MOV 格式的影片')
  }
  if (file.size > MAX_VIDEO_BYTES) {
    throw new VideoCheckError('影片太大，請選擇 30MB 以下的檔案')
  }

  const duration = await readVideoDuration(file)
  if (duration !== null && duration > MAX_VIDEO_SECONDS) {
    throw new VideoCheckError(`影片太長（${Math.ceil(duration)} 秒），上限 60 秒`)
  }
  return { duration }
}

// 用一個不放進頁面的 <video> 元素只讀 metadata（preload = 'metadata' 不會載入整部影片）
function readVideoDuration(file) {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file)
    const video = document.createElement('video')
    video.preload = 'metadata'
    video.muted = true

    let settled = false
    const finish = (value) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      video.removeAttribute('src')
      video.load() // 讓瀏覽器釋放這個檔案
      URL.revokeObjectURL(url)
      resolve(value)
    }

    const timer = setTimeout(() => finish(null), METADATA_TIMEOUT_MS)
    // 有些 WebM（例如瀏覽器錄影產生的）長度是 Infinity，當成讀不到
    video.onloadedmetadata = () => finish(Number.isFinite(video.duration) ? video.duration : null)
    video.onerror = () => finish(null)
    video.src = url
  })
}