<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { commentsApi } from '../api'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'
import CommentItem from './CommentItem.vue'
import LoadingState from './LoadingState.vue'
import UserAvatar from './UserAvatar.vue'

const props = defineProps({
  postId: { type: String, required: true },
  postAuthorUsername: { type: String, required: true },
  initialCount: { type: Number, default: 0 },
})

const emit = defineEmits(['count-change'])

const auth = useAuthStore()
const router = useRouter()

const comments = ref([])
const total = ref(props.initialCount)
const loading = ref(true)
const loadError = ref(null)

const draft = ref('')
const submitting = ref(false)
const submitError = ref(null)

const MAX = 1000
// 用展開運算子計算長度，與後端的 rune 計數一致
const draftLength = computed(() => [...draft.value].length)
const overflow = computed(() => draftLength.value > MAX)
const canSubmit = computed(
  () => draft.value.trim() !== '' && !overflow.value && !submitting.value
)

async function load() {
  loading.value = true
  loadError.value = null
  try {
    // MVP：一次載入前 100 則，不做留言分頁
    const res = await commentsApi.list(props.postId, { limit: 100 })
    comments.value = res.data
    total.value = res.pagination.total
    emit('count-change', total.value)
  } catch (err) {
    loadError.value = err.message
    comments.value = []
  } finally {
    loading.value = false
  }
}

watch(() => props.postId, load, { immediate: true })

async function submit() {
  if (!canSubmit.value) return

  submitting.value = true
  submitError.value = null

  try {
    const res = await commentsApi.create(props.postId, draft.value)
    comments.value.push(res.data)   // 正序排列，新留言接在最後
    total.value += 1
    emit('count-change', total.value)
    draft.value = ''
  } catch (err) {
    if (err instanceof ApiError) {
      switch (err.code) {
        case 'VALIDATION_ERROR':
          submitError.value = err.details?.fields?.content ?? '內容不符合規則。'
          break
        case 'UNAUTHENTICATED':
          router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
          return
        case 'NOT_FOUND':
          submitError.value = '這篇文章已不存在。'
          break
        default:
          submitError.value = err.message
      }
    } else {
      submitError.value = '發生未預期的錯誤。'
    }
  } finally {
    submitting.value = false
  }
}

function onUpdated(updated) {
  const i = comments.value.findIndex((c) => c.id === updated.id)
  if (i !== -1) comments.value[i] = updated
}

function onDeleted(id) {
  comments.value = comments.value.filter((c) => c.id !== id)
  total.value = Math.max(0, total.value - 1)
  emit('count-change', total.value)
}

function goLogin() {
  router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
}
</script>

<template>
  <section class="comments">
    <h2 class="comments__title">
      留言
      <span class="comments__count">{{ total }}</span>
    </h2>

    <!-- 發表留言 -->
    <div v-if="auth.isAuthenticated" class="composer">
      <UserAvatar
        :src="auth.user.avatar_url"
        :name="auth.user.display_name || auth.user.username"
        :size="36"
      />
      <div class="composer__body">
        <textarea
          v-model="draft"
          class="composer__textarea"
          rows="3"
          placeholder="留下你的想法…"
        ></textarea>
        <div class="composer__footer">
          <span class="composer__counter" :class="{ 'composer__counter--over': overflow }">
            {{ draftLength }} / {{ MAX }}
          </span>
          <button class="btn btn--primary btn--sm" :disabled="!canSubmit" @click="submit">
            {{ submitting ? '送出中…' : '送出留言' }}
          </button>
        </div>
        <p v-if="submitError" class="composer__error">{{ submitError }}</p>
      </div>
    </div>

    <p v-else class="comments__login">
      <button class="comments__login-btn" @click="goLogin">登入</button>
      後即可留言。
    </p>

    <!-- 留言列表 -->
    <LoadingState v-if="loading" text="載入留言中" />
    <p v-else-if="loadError" class="state state--error">{{ loadError }}</p>
    <p v-else-if="comments.length === 0" class="state">還沒有留言，成為第一個吧。</p>
    <div v-else class="comments__list">
      <CommentItem
        v-for="c in comments"
        :key="c.id"
        :comment="c"
        :post-author-username="postAuthorUsername"
        @updated="onUpdated"
        @deleted="onDeleted"
      />
    </div>
  </section>
</template>

<style scoped>
.comments {
  margin-top: var(--space-16);
  padding-top: var(--space-8);
  border-top: 1px solid var(--color-border);
}

.comments__title {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  font-family: var(--font-serif);
  font-size: var(--text-xl);
  margin-bottom: var(--space-6);
}

.comments__count {
  font-family: var(--font-ui);
  font-size: var(--text-base);
  color: var(--color-text-muted);
}

.composer {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-6);
}

.composer__body {
  flex: 1;
  min-width: 0;
}

.composer__textarea {
  width: 100%;
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font: inherit;
  line-height: var(--leading-normal);
  resize: vertical;
}

.composer__textarea:focus {
  outline: none;
  border-color: var(--color-accent);
}

.composer__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-top: var(--space-2);
}

.composer__counter {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}

.composer__counter--over {
  color: var(--color-danger);
}

.composer__error {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-danger);
}

.comments__login {
  padding: var(--space-4);
  margin-bottom: var(--space-6);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  text-align: center;
}

.comments__login-btn {
  color: var(--color-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.comments__list {
  display: flex;
  flex-direction: column;
}
</style>