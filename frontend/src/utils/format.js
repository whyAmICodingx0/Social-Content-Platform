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