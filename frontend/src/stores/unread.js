import { defineStore } from 'pinia'
import { ref } from 'vue'
import { conversationsApi } from '../api'

/**
 * 全域未讀計數（header 紅點）。
 *
 * 更新時機：
 *  - App 啟動、登入後
 *  - 收到 message.created（非自己傳的）→ 本地 +1（樂觀）
 *  - 標記已讀後 → 重新拉一次權威數字
 *
 * 為什麼收到訊息時是本地 +1 而不是重新拉：
 * 訊息可能很頻繁，每則都打一次 API 太浪費。本地 +1 可能因為
 * 「使用者正在看那個對話」而略微高估，但開啟對話時的標記已讀
 * 會拉回權威數字，誤差不會累積。
 */
export const useUnreadStore = defineStore('unread', () => {
  const count = ref(0)

  async function refresh() {
    try {
      const res = await conversationsApi.unreadCount()
      count.value = res.data.unread_count
    } catch {
      // 未讀數失敗不該影響任何功能，靜默略過
    }
  }

  function increment() {
    count.value += 1
  }

  function reset() {
    count.value = 0
  }

  return { count, refresh, increment, reset }
})