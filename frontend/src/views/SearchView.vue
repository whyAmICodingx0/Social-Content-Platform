<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { searchApi } from '../api'
import { ApiError } from '../api/client'
import { setTitle } from '../utils/title'
import PostCard from '../components/PostCard.vue'
import UserAvatar from '../components/UserAvatar.vue'
import PaginationNav from '../components/PaginationNav.vue'
import LoadingState from '../components/LoadingState.vue'

const route = useRoute()
const router = useRouter()

const results = ref([])
const pagination = ref(null)
const loading = ref(false)
const error = ref(null)
const searched = ref(false)

// 查詢狀態全部放網址（決策 #40）：可分享、返回鍵可用、重整不遺失
const q = computed(() => route.query.q ?? '')
const tab = computed(() => (route.query.type === 'users' ? 'users' : 'posts'))
const currentPage = computed(() => Number(route.query.page) || 1)

// 輸入框的本地狀態（與網址分離，避免每打一個字就改網址）
const draft = ref(q.value)

const MIN = 2
// 用展開運算子計算長度，與後端的 rune 計數一致
const tooShort = computed(() => {
  const t = q.value.trim()
  return t.length > 0 && [...t].length < MIN
})

async function search() {
  const term = q.value.trim()
  if (!term) {
    results.value = []
    pagination.value = null
    searched.value = false
    return
  }

  loading.value = true
  error.value = null

  try {
    const params = { q: term, page: currentPage.value }
    const res = tab.value === 'users'
      ? await searchApi.users(params)
      : await searchApi.posts(params)

    results.value = res.data
    pagination.value = res.pagination
    searched.value = true
    setTitle(`搜尋「${term}」`)
  } catch (err) {
    results.value = []
    pagination.value = null
    searched.value = true

    // api.FailWithFields 產生的是 { details: { fields: {...} } }，
    // client.js 把整個 details 原樣塞進 err.details ——
    // 所以完整路徑是 err.details.fields.q（與 Onboarding / Editor / Settings 一致）
    if (err instanceof ApiError && err.code === 'VALIDATION_ERROR') {
      error.value = err.details?.fields?.q ?? '搜尋關鍵字不符合規則。'
    } else {
      error.value = '搜尋失敗，請稍後再試。'
    }
  } finally {
    loading.value = false
  }
}

// 監聽網址上的三個參數（不是整個 query 物件——物件每次都是新的參考，
// 會造成不必要的重複請求）
watch(() => [route.query.q, route.query.type, route.query.page], () => {
  draft.value = q.value
  search()
}, { immediate: true })

function submit() {
  const term = draft.value.trim()
  if (!term) return
  // 換關鍵字時回到第 1 頁
  router.push({ path: '/search', query: { q: term, type: tab.value } })
}

function switchTab(next) {
  if (next === tab.value) return
  router.push({ path: '/search', query: { q: q.value, type: next } })
}

function goToPage(page) {
  router.push({ path: '/search', query: { ...route.query, page } })
}

function onLikeUpdate({ id, count, liked }) {
  const p = results.value.find((x) => x.id === id)
  if (p) {
    p.like_count = count
    p.liked_by_me = liked
  }
}

function userName(u) {
  return u.display_name || u.username
}
</script>

<template>
  <div class="container container--narrow">
    <header class="search__header">
      <h1 class="search__title">搜尋</h1>

      <div class="search__box">
        <input
          v-model="draft"
          class="search__input"
          type="search"
          placeholder="搜尋文章或使用者…"
          @keydown.enter="submit"
        />
        <button class="btn btn--primary btn--sm" @click="submit">搜尋</button>
      </div>

      <p v-if="tooShort" class="search__hint search__hint--error">
        關鍵字至少需要 2 個字元。
      </p>

      <nav v-if="q" class="search__tabs">
        <button
          class="search__tab"
          :class="{ 'search__tab--active': tab === 'posts' }"
          @click="switchTab('posts')"
        >
          文章
        </button>
        <button
          class="search__tab"
          :class="{ 'search__tab--active': tab === 'users' }"
          @click="switchTab('users')"
        >
          使用者
        </button>
      </nav>
    </header>

    <LoadingState v-if="loading" text="搜尋中" />

    <p v-else-if="error" class="state state--error">{{ error }}</p>

    <div v-else-if="!q" class="state">
      輸入關鍵字開始搜尋。
    </div>

    <div v-else-if="results.length === 0 && searched" class="state">
      找不到符合「{{ q }}」的{{ tab === 'users' ? '使用者' : '文章' }}。
    </div>

    <template v-else-if="results.length > 0">
      <!-- 文章：重用 PostCard -->
      <div v-if="tab === 'posts'" class="post-list">
        <PostCard
          v-for="post in results"
          :key="post.id"
          :post="post"
          @like-update="onLikeUpdate"
        />
      </div>

      <!-- 使用者 -->
      <ul v-else class="user-list">
        <li v-for="u in results" :key="u.id">
          <RouterLink :to="`/@${u.username}`" class="user">
            <UserAvatar :src="u.avatar_url" :name="userName(u)" :size="44" />
            <div class="user__body">
              <span class="user__name">{{ userName(u) }}</span>
              <span class="user__handle">@{{ u.username }}</span>
              <p v-if="u.bio" class="user__bio">{{ u.bio }}</p>
            </div>
          </RouterLink>
        </li>
      </ul>

      <PaginationNav v-if="pagination" :pagination="pagination" @change="goToPage" />
    </template>
  </div>
</template>

<style scoped>
.search__header {
  padding-bottom: var(--space-4);
  margin-bottom: var(--space-2);
  border-bottom: 1px solid var(--color-border);
}

.search__title {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  margin-bottom: var(--space-4);
}

.search__box {
  display: flex;
  gap: var(--space-2);
}

.search__input {
  flex: 1;
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font: inherit;
}

.search__input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.search__hint {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.search__hint--error {
  color: var(--color-danger);
}

.search__tabs {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-4);
}

.search__tab {
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.search__tab--active {
  background: var(--color-border);
  color: var(--color-text);
  font-weight: 600;
}

.post-list {
  display: flex;
  flex-direction: column;
}

.user {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-2);
  border-bottom: 1px solid var(--color-border);
  transition: background-color 0.15s ease;
}

.user:hover {
  background: var(--color-surface);
}

.user__body {
  flex: 1;
  min-width: 0;
}

.user__name {
  font-weight: 600;
}

.user__handle {
  margin-left: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.user__bio {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>