<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { ApiError } from '../api/client'
import UserAvatar from '../components/UserAvatar.vue'

const auth = useAuthStore()

const displayName = ref('')
const bio = ref('')
const avatarUrl = ref('')

const saving = ref(false)
const saved = ref(false)
const formError = ref(null)
const fieldErrors = ref({})

const MAX_BIO = 500

onMounted(() => {
  // 後端沒填的欄位是 null，表單需要字串
  displayName.value = auth.user.display_name ?? ''
  bio.value = auth.user.bio ?? ''
  avatarUrl.value = auth.user.avatar_url ?? ''
})

// 用展開運算子計算長度，避免 emoji 等字元被算成兩個
const bioLength = computed(() => [...bio.value].length)
const bioOverflow = computed(() => bioLength.value > MAX_BIO)

const canSave = computed(() => !saving.value && !bioOverflow.value)

async function save() {
  if (!canSave.value) return

  saving.value = true
  saved.value = false
  formError.value = null
  fieldErrors.value = {}

  try {
    // ★ 只送三個允許的欄位。
    //   username 與 email 是唯讀，放進 payload 會被後端回 400（決策 #24）。
    //   空字串語意（spec 6.1.1）：bio / avatar_url = 清空；display_name = 不變更
    await auth.updateProfile({
      display_name: displayName.value.trim(),
      bio: bio.value.trim(),
      avatar_url: avatarUrl.value.trim(),
    })
    saved.value = true
  } catch (err) {
    if (err instanceof ApiError && err.code === 'VALIDATION_ERROR') {
      fieldErrors.value = err.details?.fields ?? {}
      formError.value = '有欄位不符合規則，請檢查後再儲存。'
    } else {
      formError.value = err.message ?? '儲存失敗，請稍後再試。'
    }
  } finally {
    saving.value = false
  }
}

function reset() {
  displayName.value = auth.user.display_name ?? ''
  bio.value = auth.user.bio ?? ''
  avatarUrl.value = auth.user.avatar_url ?? ''
  saved.value = false
  formError.value = null
  fieldErrors.value = {}
}
</script>

<template>
  <div class="container container--narrow">
    <h1 class="settings__heading">個人檔案設定</h1>

    <!-- 唯讀資訊 -->
    <section class="readonly">
      <div class="readonly__row">
        <span class="readonly__label">Username</span>
        <span class="readonly__value">@{{ auth.user.username }}</span>
      </div>
      <div class="readonly__row">
        <span class="readonly__label">Email</span>
        <span class="readonly__value">{{ auth.user.email }}</span>
      </div>
      <p class="readonly__note">
        Username 一經設定即無法變更（它是你的公開網址）；Email 來自 Google 帳號，
        也不能在此修改。
      </p>
    </section>

    <!-- 可編輯欄位 -->
    <div class="field">
      <label class="field__label" for="display-name">顯示名稱</label>
      <input
        id="display-name"
        v-model="displayName"
        class="field__input"
        type="text"
        maxlength="50"
        placeholder="留空則維持原本的名稱"
      />
      <p v-if="fieldErrors.display_name" class="field__error">
        {{ fieldErrors.display_name }}
      </p>
      <p v-else class="field__hint">最多 50 個字元。</p>
    </div>

    <div class="field">
      <label class="field__label" for="bio">個人簡介</label>
      <textarea
        id="bio"
        v-model="bio"
        class="field__input field__textarea"
        placeholder="介紹一下自己"
      ></textarea>
      <p v-if="fieldErrors.bio" class="field__error">{{ fieldErrors.bio }}</p>
      <p v-else class="field__hint" :class="{ 'field__hint--error': bioOverflow }">
        {{ bioLength }} / {{ MAX_BIO }} 字元{{ bioOverflow ? '（已超過上限）' : '' }}．留空即清除
      </p>
    </div>

    <div class="field">
      <label class="field__label" for="avatar">頭像網址</label>
      <div class="avatar-row">
        <UserAvatar :src="avatarUrl" :name="displayName || auth.user.username" :size="56" />
        <input
          id="avatar"
          v-model="avatarUrl"
          class="field__input"
          type="url"
          placeholder="https://..."
        />
      </div>
      <p v-if="fieldErrors.avatar_url" class="field__error">{{ fieldErrors.avatar_url }}</p>
      <p v-else class="field__hint">貼上圖片網址即可，左側會即時預覽。留空即清除。</p>
    </div>

    <p v-if="formError" class="form__error">{{ formError }}</p>
    <p v-if="saved" class="form__success">已儲存。</p>

    <div class="settings__actions">
      <button class="btn btn--primary" :disabled="!canSave" @click="save">
        {{ saving ? '儲存中…' : '儲存變更' }}
      </button>
      <button class="btn btn--ghost" :disabled="saving" @click="reset">還原</button>
      <RouterLink :to="`/@${auth.user.username}`" class="btn btn--ghost">
        看看我的個人頁
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.settings__heading {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  margin-bottom: var(--space-8);
}

.readonly {
  padding: var(--space-4);
  margin-bottom: var(--space-8);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
}

.readonly__row {
  display: flex;
  gap: var(--space-4);
  padding: var(--space-2) 0;
  font-size: var(--text-sm);
}

.readonly__label {
  width: 90px;
  flex-shrink: 0;
  color: var(--color-text-muted);
}

.readonly__value {
  font-weight: 550;
  word-break: break-all;
}

.readonly__note {
  margin-top: var(--space-3);
  padding-top: var(--space-3);
  border-top: 1px solid var(--color-border);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
}

.field {
  margin-bottom: var(--space-8);
}

.field__label {
  display: block;
  margin-bottom: var(--space-2);
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

.field__textarea {
  min-height: 120px;
  resize: vertical;
  line-height: var(--leading-normal);
}

.avatar-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.field__hint {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.field__hint--error,
.field__error {
  color: var(--color-danger);
}

.field__error {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
}

.form__error,
.form__success {
  margin-bottom: var(--space-4);
  padding: var(--space-3);
  border-radius: var(--radius);
  font-size: var(--text-sm);
}

.form__error {
  border: 1px solid var(--color-danger);
  color: var(--color-danger);
}

.form__success {
  border: 1px solid var(--color-accent);
  color: var(--color-accent);
}

.settings__actions {
  display: flex;
  gap: var(--space-3);
  padding-top: var(--space-4);
  border-top: 1px solid var(--color-border);
}

.settings__actions .btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

@media (max-width: 640px) {
  .settings__actions {
    flex-wrap: wrap;
  }

  .readonly__row {
    flex-direction: column;
    gap: var(--space-1);
  }

  .readonly__label {
    width: auto;
  }
}
</style>