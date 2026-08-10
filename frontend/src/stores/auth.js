import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api'
import { ApiError } from '../api/client'

export const useAuthStore = defineStore('auth', () => {
  const user = ref(null)
  // ready：是否已完成首次身分確認。用來避免畫面閃過錯誤狀態。
  const ready = ref(false)

  // initPromise 存在 store 外層（不是 ref）：
  // 它只是個「確認動作是否已啟動」的記號，不需要響應式。
  let initPromise = null

  const isAuthenticated = computed(() => user.value !== null)

  async function fetchMe() {
    try {
      const res = await authApi.me()
      user.value = res.data
    } catch (err) {
      // 401 代表未登入，是正常情況而非錯誤
      if (!(err instanceof ApiError && err.status === 401)) {
        console.error('取得使用者資訊失敗：', err)
      }
      user.value = null
    } finally {
      ready.value = true
    }
  }

  /**
   * init：確保「首次身分確認」全 App 只跑一次。
   * App.vue 與 router 守衛都會呼叫它，但共用同一個 promise，
   * 所以不會重複打 API，也保證守衛等得到結果。
   */
  function init() {
    if (!initPromise) initPromise = fetchMe()
    return initPromise
  }

  async function signup(payload) {
    // 後端 signup 成功後已建立 session 並回傳完整的 me，
    // 直接寫入 store，不必再打一次 /me。
    const res = await authApi.signup(payload)
    user.value = res.data
    ready.value = true
    return res.data
  }

  /**
   * 更新個人檔案。
   * 放在 store 而非元件裡，是因為 user 是全域狀態——
   * 存檔後 header 的名字要立刻跟著變。
   */
  async function updateProfile(payload) {
    const res = await authApi.updateMe(payload)
    user.value = res.data
    return res.data
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      // 後端登出是冪等的；就算請求失敗也要清掉前端狀態
      user.value = null
    }
  }

  return { user, ready, isAuthenticated, init, fetchMe, signup, updateProfile, logout }
})