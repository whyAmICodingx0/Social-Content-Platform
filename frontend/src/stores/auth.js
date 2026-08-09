import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api'
import { ApiError } from '../api/client'

export const useAuthStore = defineStore('auth', () => {
  const user = ref(null)
  // ready：是否已完成首次身分確認。
  // 沒有這個旗標，畫面會在載入瞬間閃過「未登入」狀態。
  const ready = ref(false)

  const isAuthenticated = computed(() => user.value !== null)

  async function fetchMe() {
    try {
      const res = await authApi.me()
      user.value = res.data
    } catch (err) {
      // 401 是正常情況（未登入），不是錯誤
      if (!(err instanceof ApiError && err.status === 401)) {
        console.error('取得使用者資訊失敗：', err)
      }
      user.value = null
    } finally {
      ready.value = true
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      // 後端登出是冪等的，就算失敗也要清掉前端狀態
      user.value = null
    }
  }

  return { user, ready, isAuthenticated, fetchMe, logout }
})