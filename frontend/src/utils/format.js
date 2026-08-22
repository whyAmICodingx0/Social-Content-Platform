/**
 * 把後端的 RFC 3339 時間（如 2026-07-12T08:30:00Z）轉成易讀格式。
 * 【事實】Intl.DateTimeFormat 是瀏覽器內建的國際化 API，
 * 會自動依時區轉換——後端存 UTC，這裡顯示成使用者當地時間。
 */
export function formatDate(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return new Intl.DateTimeFormat('zh-TW', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  }).format(d)
}

/**
 * 相對時間：剛剛 / N 分鐘前 / N 小時前 / N 天前 / 日期。
 * 通知列表用 —— 「3 分鐘前」比「2026年8月18日」有用得多。
 */
export function formatRelativeTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''

  const diffMs = Date.now() - d.getTime()
  const min = Math.floor(diffMs / 60000)

  if (min < 1) return '剛剛'
  if (min < 60) return `${min} 分鐘前`

  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr} 小時前`

  const day = Math.floor(hr / 24)
  if (day < 7) return `${day} 天前`

  return formatDate(iso)
}