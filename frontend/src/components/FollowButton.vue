<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { usersApi } from '../api'
import { useAuthStore } from '../stores/auth'
import { ApiError } from '../api/client'

const props = defineProps({
  username: { type: String, required: true },
  following: { type: Boolean, default: false },
  followerCount: { type: Number, default: 0 },
})

const emit = defineEmits(['update'])

const auth = useAuthStore()
const router = useRouter()

const pending = ref(false)
const error = ref(null)
// hover 時把「追蹤中」換成「取消追蹤」，這是常見的互動慣例
const hovering = ref(false)

const label = computed(() => {
  if (!props.following) return '追蹤'
  return hovering.value ? '取消追蹤' : '追蹤中'
})

async function toggle() {
  if (!auth.isAuthenticated) {
    router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
    return
  }
  if (pending.value) return

  // 樂觀更新：先改畫面，失敗再回滾
  const prev = { following: props.following, followerCount: props.followerCount }
  emit('update', {
    following: !props.following,
    followerCount: props.following ? props.followerCount - 1 : props.followerCount + 1,
  })

  pending.value = true
  error.value = null

  try {
    const res = prev.following
      ? await usersApi.unfollow(props.username)
      : await usersApi.follow(props.username)

    // 以伺服器回傳的數字為準（可能有其他人同時追蹤）
    emit('update', {
      following: res.data.followed_by_me,
      followerCount: res.data.follower_count,
    })
  } catch (err) {
    emit('update', prev) // 回滾

    if (err instanceof ApiError) {
      if (err.status === 401) {
        router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
        return
      }
      error.value =
        err.code === 'VALIDATION_ERROR'
          ? (err.details?.fields?.username ?? '無法執行此操作。')
          : '操作失敗，請再試一次。'
    } else {
      error.value = '操作失敗，請再試一次。'
    }
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="follow">
    <button
      class="btn"
      :class="following ? 'btn--ghost follow__btn--following' : 'btn--primary'"
      :disabled="pending"
      @click="toggle"
      @mouseenter="hovering = true"
      @mouseleave="hovering = false"
    >
      {{ pending ? '處理中…' : label }}
    </button>
    <p v-if="error" class="follow__error">{{ error }}</p>
  </div>
</template>

<style scoped>
.follow {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--space-1);
}

.follow__btn--following:hover {
  border-color: var(--color-danger);
  color: var(--color-danger);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.follow__error {
  font-size: var(--text-sm);
  color: var(--color-danger);
}
</style>