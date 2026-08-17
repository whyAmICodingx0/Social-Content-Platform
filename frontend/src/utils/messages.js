import { conversationsApi } from '../api'

// 一次補抓的上限，避免離線太久時無止境地打 API
const MAX_CATCHUP_PAGES = 20

/**
 * 補抓斷線期間漏掉的訊息。
 *
 * ⚠️ 這個迴圈是必須的，不是最佳化。
 *
 * after= 一次只回 limit 筆。離線一週後回來，若只呼叫一次，
 * 中間的訊息會「永遠遺失」—— 而且不會有任何錯誤訊息，
 * 使用者只會發現對話裡莫名缺了一段。
 *
 * 因此必須迴圈抓到 has_more 為 false 為止（決策 #68）。
 *
 * @param {string} conversationId
 * @param {string} afterId 本地已知的最後一則訊息 id
 * @param {number} limit
 * @returns {Promise<Array>} 依 (created_at, id) 遞增排序的訊息
 */
export async function fetchMessagesAfter(conversationId, afterId, limit = 100) {
  const collected = []
  let cursor = afterId

  for (let page = 0; page < MAX_CATCHUP_PAGES; page += 1) {
    const res = await conversationsApi.messages(conversationId, {
      after: cursor,
      limit,
    })

    collected.push(...res.data)

    if (!res.has_more || res.data.length === 0) break

    // 下一輪的錨點是這一批的最後一則（回應是遞增排序，所以取最後一個）
    cursor = res.data[res.data.length - 1].id
  }

  return collected
}