<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { postsApi } from '../api'
import PostCard from '../components/PostCard.vue'
import PaginationNav from '../components/PaginationNav.vue'
import LoadingState from '../components/LoadingState.vue'

const route = useRoute()
const router = useRouter()

const posts = ref([])
const pagination = ref(null)
const loading = ref(true)
const error = ref(null)

const currentPage = computed(() => Number(route.query.page) || 1)

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await postsApi.feed({ page: currentPage.value })
    posts.value = res.data
    pagination.value = res.pagination
  } catch (err) {
    error.value = err.message
    posts.value = []
    pagination.value = null
  } finally {
    loading.value = false
  }
}

watch(() => route.query.page, load, { immediate: true })

function goToPage(page) {
  router.push({ path: '/feed', query: { page } })
}

function onLikeUpdate({ id, count, liked }) {
  const p = posts.value.find((x) => x.id === id)
  if (p) {
    p.like_count = count
    p.liked_by_me = liked
  }
}
</script>

<template>
  <div class="container container--narrow">
    <header class="feed__header">
      <h1 class="feed__title">追蹤動態</h1>
      <p class="feed__desc">你追蹤的人與自己的文章。</p>
    </header>

    <LoadingState v-if="loading" />

    <p v-else-if="error" class="state state--error">{{ error }}</p>

    <!--
      空狀態由前端處理（決策 #45）：
      API 誠實回空陣列，不 fallback 全站文章——
      否則使用者會誤以為追蹤的人發了那些內容。
    -->
    <div v-else-if="posts.length === 0" class="empty">
      <h2 class="empty__title">這裡還很安靜</h2>
      <p class="empty__desc">
        你追蹤的人還沒有發布文章，或者你還沒開始追蹤任何人。<br />
        到首頁看看有什麼有趣的內容吧。
      </p>
      <RouterLink to="/" class="btn btn--primary">探索文章</RouterLink>
    </div>

    <template v-else>
      <div class="post-list">
        <PostCard
          v-for="post in posts"
          :key="post.id"
          :post="post"
          @like-update="onLikeUpdate"
        />
      </div>
      <PaginationNav v-if="pagination" :pagination="pagination" @change="goToPage" />
    </template>
  </div>
</template>

<style scoped>
.feed__header {
  padding-bottom: var(--space-4);
  margin-bottom: var(--space-2);
  border-bottom: 1px solid var(--color-border);
}

.feed__title {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  margin-bottom: var(--space-1);
}

.feed__desc {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.empty {
  padding: var(--space-16) var(--space-4);
  text-align: center;
}

.empty__title {
  font-family: var(--font-serif);
  font-size: var(--text-xl);
  margin-bottom: var(--space-3);
}

.empty__desc {
  color: var(--color-text-muted);
  line-height: var(--leading-relaxed);
  margin-bottom: var(--space-8);
}

.post-list {
  display: flex;
  flex-direction: column;
}
</style>