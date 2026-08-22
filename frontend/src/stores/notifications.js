import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { notificationsApi } from '../api'

/**
 * 通知狀態（紅點 + 列表）。
 *
 * 紅點的更新來源（決策 N-7，不可混用）：
 *  - WS notification.created → 本地 +1（樂觀）
 *  - WS 每次連線成功 / 頁面重新取得焦點 → 重抓 unread-count（權威，校正漂移）
 *  - read / read-all 的回應 → 以回應的 unread_count 為準
 *
 * 決策 #90：WS 不保證送達，Render 休眠期間的通知不會推到前端，
 * 所以「重連時重抓」是必要的，不是最佳化。
 */
export const useNotificationsStore = defineStore('notifications', () => {
  const unreadCount = ref(0)
  const items = ref([])
  const pagination = ref(null)

  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref(null)
  const loaded = ref(false)

  // 開啟面板當下「未讀」的 id 快照。
  // 用途：開啟面板會立刻全部標為已讀，若直接依 is_read 決定樣式，
  // 使用者會看到高亮在眼前消失。依快照高亮可以維持到面板關閉。
  const highlighted = ref(new Set())

  const hasMore = computed(() => pagination.value?.has_next ?? false)

  /** 權威的未讀數。失敗靜默 —— 紅點壞掉不該影響其他功能。 */
  async function refreshUnreadCount() {
    try {
      const res = await notificationsApi.unreadCount()
      unreadCount.value = res.data.unread_count
    } catch {
      /* 靜默 */
    }
  }

  async function loadList() {
    loading.value = true
    error.value = null
    try {
      const res = await notificationsApi.list({ page: 1, limit: 20 })
      items.value = res.data
      pagination.value = res.pagination
      loaded.value = true
    } catch (err) {
      error.value = err.message
    } finally {
      loading.value = false
    }
  }

  async function loadMore() {
    if (!hasMore.value || loadingMore.value) return

    loadingMore.value = true
    try {
      const res = await notificationsApi.list({
        page: pagination.value.page + 1,
        limit: 20,
      })

      // ⚠️ 依 id 去重後才附加。
      // 通知是 offset 分頁（決策 #26），而新通知會插在最前面 ——
      // 翻頁期間若有新通知進來，整個序列會位移，第 2 頁可能回傳
      // 已經在列表裡的項目。這與 messages 改用 cursor 分頁（決策 #67）
      // 是同一個問題，只是通知很少深翻，用去重處理即可。
      const seen = new Set(items.value.map((n) => n.id))
      items.value.push(...res.data.filter((n) => !seen.has(n.id)))
      pagination.value = res.pagination
    } catch (err) {
      error.value = err.message
    } finally {
      loadingMore.value = false
    }
  }

  /**
   * 收到 WS 事件時併入。
   *
   * 去重：id 已存在就忽略。重連後可能同時發生「重抓列表」與
   * 「收到補推的事件」，沒有去重會出現重複項目。
   */
  function prepend(n) {
    if (items.value.some((x) => x.id === n.id)) return

    // 列表沒載入過就不用維護它，等使用者開面板時再抓
    if (loaded.value) items.value.unshift(n)

    // 紅點無論列表有沒有載入都要更新
    if (!n.is_read) unreadCount.value += 1
  }

  /** 標記指定幾筆為已讀（面板開著時新進來的那一則會走這條） */
  async function markRead(ids) {
    if (!ids.length) return
    try {
      const res = await notificationsApi.markRead(ids)
      unreadCount.value = res.data.unread_count
      items.value.forEach((n) => {
        if (ids.includes(n.id)) n.is_read = true
      })
    } catch {
      /* 靜默：下次開啟面板的 read-all 會補上 */
    }
  }

  /** 全部標記已讀（開啟面板時呼叫） */
  async function markAllRead() {
    // 先快照，讓這些項目在本次開啟期間保持高亮（見 highlighted 的說明）
    highlighted.value = new Set(
      items.value.filter((n) => !n.is_read).map((n) => n.id)
    )

    try {
      const res = await notificationsApi.markAllRead()
      unreadCount.value = res.data.unread_count
      items.value.forEach((n) => { n.is_read = true })
    } catch {
      // 失敗就維持原狀，紅點還在，下次開啟再試
    }
  }

  function clearHighlight() {
    highlighted.value = new Set()
  }

  function reset() {
    unreadCount.value = 0
    items.value = []
    pagination.value = null
    loaded.value = false
    error.value = null
    highlighted.value = new Set()
  }

  return {
    unreadCount, items, pagination, loading, loadingMore, error, loaded,
    highlighted, hasMore,
    refreshUnreadCount, loadList, loadMore, prepend,
    markRead, markAllRead, clearHighlight, reset,
  }
})