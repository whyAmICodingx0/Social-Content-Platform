/**
 * 圖片的 Canvas 壓縮（決策 #122、#123、#150）。
 *
 * ⚠️ 這只是「最佳化」，不是限制的執行者。真正的大小與格式限制在 upload preset 與後端反查。
 * ⚠️ 失敗時一律丟 CompressError，呼叫端必須中止上傳——絕不可默默改送原檔。
 */

// 與後端白名單一致（不含 SVG：可夾帶 script；不含 GIF：Canvas 會毀掉動畫）
export const ACCEPTED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp']

// ⚠️ 需確認：20MB 是推測值。目的是避免手機解碼超大原圖時記憶體不足，不是額度限制（決策 #151）
const MAX_INPUT_BYTES = 20 * 1024 * 1024

// 一律輸出 JPEG：所有瀏覽器都支援編碼。
// 若改成 'image/webp'，不支援的瀏覽器會「默默」輸出 PNG，檔案反而更大——下面的 type 檢查就是防這個。
const OUTPUT_TYPE = 'image/jpeg'
const OUTPUT_QUALITY = 0.85

export class CompressError extends Error {
  constructor(message) {
    super(message)
    this.name = 'CompressError'
  }
}

/**
 * @param {File} file
 * @param {number} maxEdge  輸出的最長邊（px）。原圖比它小就不放大。
 * @returns {Promise<{ blob: Blob, width: number, height: number }>}
 */
export async function compressImage(file, maxEdge) {
  if (!ACCEPTED_IMAGE_TYPES.includes(file.type)) {
    throw new CompressError('只接受 JPG、PNG、WebP 格式的圖片')
  }
  if (file.size > MAX_INPUT_BYTES) {
    throw new CompressError('圖片太大，請選擇 20MB 以下的檔案')
  }

  const objectUrl = URL.createObjectURL(file)
  try {
    const img = new Image()
    img.src = objectUrl
    try {
      // decode() 會等到圖片真的解碼完成；檔案損毀或其實不是圖片時會 reject
      await img.decode()
    } catch {
      throw new CompressError('無法讀取這張圖片，檔案可能已損毀或格式不符')
    }

    // naturalWidth / naturalHeight 是原始尺寸。
    // 現代瀏覽器會先套用手機照片的 EXIF 方向，所以直拍照片不會躺著。
    const srcW = img.naturalWidth
    const srcH = img.naturalHeight
    if (!srcW || !srcH) {
      throw new CompressError('無法讀取這張圖片的尺寸')
    }

    const scale = Math.min(1, maxEdge / Math.max(srcW, srcH))
    const width = Math.max(1, Math.round(srcW * scale))
    const height = Math.max(1, Math.round(srcH * scale))

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      throw new CompressError('瀏覽器無法處理這張圖片')
    }

    // JPEG 沒有透明度：不先鋪底色的話，PNG 的透明區域會變成黑色
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, width, height)
    ctx.imageSmoothingEnabled = true
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(img, 0, 0, width, height)

    // 重新編碼的副作用：原檔的 EXIF（含手機照片的 GPS 位置）不會被帶出瀏覽器
    const blob = await new Promise((resolve) => canvas.toBlob(resolve, OUTPUT_TYPE, OUTPUT_QUALITY))
    if (!blob || blob.size === 0 || blob.type !== OUTPUT_TYPE) {
      throw new CompressError('圖片壓縮失敗')
    }

    return { blob, width, height }
  } finally {
    URL.revokeObjectURL(objectUrl)
  }
}