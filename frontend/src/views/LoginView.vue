<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { authApi } from '../api'

const route = useRoute()

// 後端 OAuth 失敗時導向 /login?error=xxx
const errorMessages = {
  access_denied: '你取消了 Google 授權。若要使用本站，請重新登入並允許存取。',
  google_exchange_failed: '與 Google 溝通時發生問題，請稍後再試一次。',
  email_not_verified: '這個 Google 帳號的 email 尚未通過驗證，無法用於註冊。',
  account_unavailable: '此帳號目前無法使用，請聯繫管理者。',
  invalid_state: '登入請求已逾時或無效，請重新登入。',
  missing_code: 'Google 沒有回傳授權碼，請重新登入一次。',
  service_unavailable: '服務暫時無法使用，請稍後再試。',
}

const errorMessage = computed(() => {
  const code = route.query.error
  if (!code) return null
  return errorMessages[code] ?? '登入時發生未預期的問題，請再試一次。'
})

function login() {
  // 若是被守衛導過來的，記住原本要去的頁面。
  // 用 sessionStorage 是因為 OAuth 會整頁跳轉，前端的記憶體狀態會全部消失。
  const redirect = route.query.redirect
  if (redirect) sessionStorage.setItem('post_login_redirect', redirect)

  window.location.href = authApi.loginUrl
}
</script>

<template>
  <div class="container container--narrow login">
    <h1 class="login__title">登入 Inkwell</h1>
    <p class="login__desc">使用 Google 帳號登入，即可開始寫作與閱讀。</p>

    <p v-if="errorMessage" class="login__error">{{ errorMessage }}</p>

    <button class="btn btn--primary login__btn" @click="login">
      使用 Google 登入
    </button>
  </div>
</template>

<style scoped>
.login {
  text-align: center;
  padding-top: var(--space-16);
}

.login__title {
  font-family: var(--font-serif);
  font-size: var(--text-3xl);
  margin-bottom: var(--space-3);
}

.login__desc {
  color: var(--color-text-muted);
  margin-bottom: var(--space-8);
}

.login__error {
  max-width: 420px;
  margin: 0 auto var(--space-8);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius);
  color: var(--color-danger);
  font-size: var(--text-sm);
  text-align: left;
}

.login__btn {
  font-size: var(--text-base);
  padding: var(--space-3) var(--space-6);
}
</style>