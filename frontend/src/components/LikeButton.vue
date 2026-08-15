<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { postsApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { ApiError } from '../api/client'

const props = defineProps({
  postId: { type: String, required: true },
  count: { type: Number, default: 0 },
  liked: { type: Boolean, default: false },
  // 'sm' 用於列表卡片，'md' 用於文章頁
  size: { type: String, default: 'sm' },
})

const emit = defineEmits(['update'])

const auth = useAuthStore()
const router = useRouter()

const pending = ref(false)
const error = ref(false)

const label = computed(() => (props.liked ? '取消讚' : '按讚'))

async function toggle() {
  // 未登入 → 導去登入頁，並記住現在的位置
  if (!auth.isAuthenticated) {
    router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
    return
  }
  if (pending.value) return

  // 樂觀更新：先改畫面，讓點擊有立即回饋。
  // 記下原值，失敗時回滾。
  const prev = { count: props.count, liked: props.liked }
  emit('update', {
    count: props.liked ? props.count - 1 : props.count + 1,
    liked: !props.liked,
  })

  pending.value = true
  error.value = false

  try {
    const res = prev.liked
      ? await postsApi.unlike(props.postId)
      : await postsApi.like(props.postId)

    // 以伺服器回傳的實際數字為準
    // （別人可能同時也按了讚，樂觀更新的數字未必正確）
    emit('update', {
      count: res.data.like_count,
      liked: res.data.liked_by_me,
    })
  } catch (err) {
    emit('update', prev) // 回滾
    error.value = true

    if (err instanceof ApiError && err.status === 401) {
      router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
    }
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <button
    class="like"
    :class="[`like--${size}`, { 'like--active': liked, 'like--error': error }]"
    :aria-pressed="liked"
    :aria-label="label"
    :title="error ? '操作失敗，請再試一次' : label"
    @click.stop.prevent="toggle"
  >
    <svg class="like__icon" viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M12 21s-6.7-4.35-9.33-8.02C.9 10.3 1.6 6.6 4.6 5.2c2.2-1.03 4.7-.2 5.9 1.63l1.5 2.27 1.5-2.27c1.2-1.83 3.7-2.66 5.9-1.63 3 1.4 3.7 5.1 1.93 7.78C18.7 16.65 12 21 12 21z"
        :fill="liked ? 'currentColor' : 'none'"
        stroke="currentColor"
        stroke-width="1.6"
        stroke-linejoin="round"
      />
    </svg>
    <span class="like__count">{{ count }}</span>
  </button>
</template>

<style scoped>
.like {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius);
  color: var(--color-text-muted);
  transition: color 0.15s ease, background-color 0.15s ease;
}

.like:hover {
  color: var(--color-accent);
  background: var(--color-border);
}

.like--active {
  color: var(--color-accent);
}

.like--error {
  color: var(--color-danger);
}

.like__icon {
  width: 18px;
  height: 18px;
}

.like--md .like__icon {
  width: 22px;
  height: 22px;
}

.like__count {
  font-size: var(--text-sm);
  font-variant-numeric: tabular-nums; /* 數字等寬，跳動時不會晃版面 */
  min-width: 1ch;
}

.like--md .like__count {
  font-size: var(--text-base);
}

/* 尊重「減少動態效果」的系統設定 */
@media (prefers-reduced-motion: no-preference) {
  .like--active .like__icon {
    animation: pop 0.25s ease;
  }
}

@keyframes pop {
  0% { transform: scale(1); }
  50% { transform: scale(1.25); }
  100% { transform: scale(1); }
}
</style>