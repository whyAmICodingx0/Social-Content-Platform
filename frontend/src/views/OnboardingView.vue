<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ApiError } from '../api/client'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const displayName = ref('')
const submitting = ref(false)
const serverError = ref(null)
const fieldError = ref(null)
const expired = ref(false)

// 前端驗證規則與後端一致（後端仍會再驗一次，這裡只是即時回饋）
const USERNAME_RE = /^[a-z0-9_]{3,30}$/

const localError = computed(() => {
  const v = username.value
  if (v === '') return null
  if (v !== v.toLowerCase()) return '只能使用小寫字母'
  if (!USERNAME_RE.test(v)) return '需為 3-30 個字元，僅限小寫字母、數字與底線'
  return null
})

const canSubmit = computed(
  () => USERNAME_RE.test(username.value) && !submitting.value
)

async function submit() {
  if (!canSubmit.value) return

  submitting.value = true
  serverError.value = null
  fieldError.value = null

  try {
    const payload = { username: username.value }
    const d = displayName.value.trim()
    if (d) payload.display_name = d

    await auth.signup(payload)
    router.replace('/') // 成功：已登入，回首頁
  } catch (err) {
    if (!(err instanceof ApiError)) {
      serverError.value = '發生未預期的錯誤，請稍後再試。'
      return
    }

    switch (err.code) {
      case 'USERNAME_TAKEN':
        fieldError.value = '這個 username 已經有人使用了，換一個吧。'
        break
      case 'EMAIL_TAKEN':
        serverError.value =
          '這個 email 已經有帳號了，請改用原本註冊時的 Google 帳號登入。'
        break
      case 'VALIDATION_ERROR':
        // 後端的欄位錯誤放在 details.fields
        fieldError.value =
          err.details?.fields?.username ??
          err.details?.fields?.display_name ??
          '輸入內容不符合規則。'
        break
      case 'UNAUTHENTICATED':
        // pending signup 已過期（30 分鐘）或已被使用
        expired.value = true
        break
      default:
        serverError.value = err.message
    }
  } finally {
    submitting.value = false
  }
}

function restart() {
  window.location.href = '/api/v1/auth/google/login'
}
</script>

<template>
  <div class="container container--narrow onboarding">
    <template v-if="expired">
      <h1 class="onboarding__title">註冊連結已逾時</h1>
      <p class="onboarding__desc">
        註冊流程需要在 30 分鐘內完成。請重新登入一次。
      </p>
      <button class="btn btn--primary" @click="restart">重新登入</button>
    </template>

    <template v-else>
      <h1 class="onboarding__title">選擇你的 username</h1>
      <p class="onboarding__desc">
        這是你的公開代號，會出現在網址中（例如
        <code>/@yourname</code>）。<strong>設定後無法變更</strong>，請謹慎選擇。
      </p>

      <form class="form" @submit.prevent="submit">
        <div class="field">
          <label class="field__label" for="username">Username</label>
          <div class="field__prefix-wrap">
            <span class="field__prefix">@</span>
            <input
              id="username"
              v-model="username"
              class="field__input field__input--prefixed"
              type="text"
              autocomplete="off"
              autocapitalize="none"
              spellcheck="false"
              placeholder="yourname"
              maxlength="30"
            />
          </div>
          <p v-if="localError" class="field__hint field__hint--error">
            {{ localError }}
          </p>
          <p v-else-if="fieldError" class="field__hint field__hint--error">
            {{ fieldError }}
          </p>
          <p v-else class="field__hint">3-30 個字元，僅限小寫字母、數字與底線</p>
        </div>

        <div class="field">
          <label class="field__label" for="display-name">顯示名稱（選填）</label>
          <input
            id="display-name"
            v-model="displayName"
            class="field__input"
            type="text"
            placeholder="留空則使用 Google 帳號的名稱"
            maxlength="50"
          />
        </div>

        <p v-if="serverError" class="form__error">{{ serverError }}</p>

        <button class="btn btn--primary form__submit" type="submit" :disabled="!canSubmit">
          {{ submitting ? '建立中…' : '完成註冊' }}
        </button>
      </form>
    </template>
  </div>
</template>

<style scoped>
.onboarding {
  padding-top: var(--space-12);
}

.onboarding__title {
  font-family: var(--font-serif);
  font-size: var(--text-3xl);
  margin-bottom: var(--space-3);
}

.onboarding__desc {
  color: var(--color-text-muted);
  margin-bottom: var(--space-8);
  line-height: var(--leading-relaxed);
}

.onboarding__desc code {
  font-size: 0.9em;
  padding: 0 var(--space-1);
  background: var(--color-border);
  border-radius: 3px;
}

.form {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.field__label {
  font-size: var(--text-sm);
  font-weight: 600;
}

.field__input {
  width: 100%;
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
}

.field__input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.field__prefix-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.field__prefix {
  position: absolute;
  left: var(--space-3);
  color: var(--color-text-muted);
}

.field__input--prefixed {
  padding-left: calc(var(--space-3) + 14px);
}

.field__hint {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.field__hint--error {
  color: var(--color-danger);
}

.form__error {
  padding: var(--space-3);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius);
  color: var(--color-danger);
  font-size: var(--text-sm);
}

.form__submit {
  align-self: flex-start;
  padding: var(--space-3) var(--space-6);
}

.form__submit:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>