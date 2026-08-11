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

// 篩選同樣放網址（與 V-3 相同原則）
const currentStatus = computed(() => route.query.status || '')
const currentPage = computed(() => Number(route.query.page) || 1)

const filters = [
  { label: '全部', value: '' },
  { label: '草稿', value: 'draft' },
  { label: '已發布', value: 'published' },
]

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await postsApi.listMine({
      status: currentStatus.value,
      page: currentPage.value,
    })
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

watch(() => [route.query.status, route.query.page], load, { immediate: true })

function setFilter(value) {
  router.push({ path: '/me/posts', query: value ? { status: value } : {} })
}

function goToPage(page) {
  router.push({ path: '/me/posts', query: { ...route.query, page } })
}
</script>

<template>
  <div class="container container--narrow">
    <header class="mine__header">
      <h1 class="mine__title">我的文章</h1>
      <RouterLink to="/new" class="btn btn--primary">寫新文章</RouterLink>
    </header>

    <nav class="mine__filters">
      <button
        v-for="f in filters"
        :key="f.value"
        class="mine__filter"
        :class="{ 'mine__filter--active': currentStatus === f.value }"
        @click="setFilter(f.value)"
      >
        {{ f.label }}
      </button>
    </nav>

    <LoadingState v-if="loading" />
    <p v-else-if="error" class="state state--error">{{ error }}</p>
    <p v-else-if="posts.length === 0" class="state">
      {{ currentStatus === 'draft' ? '沒有草稿。' : '還沒有任何文章。' }}
    </p>

    <template v-else>
      <div class="post-list">
        <PostCard v-for="post in posts" :key="post.id" :post="post" />
      </div>
      <PaginationNav v-if="pagination" :pagination="pagination" @change="goToPage" />
    </template>
  </div>
</template>

<style scoped>
.mine__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-6);
}

.mine__title {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
}

.mine__filters {
  display: flex;
  gap: var(--space-2);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.mine__filter {
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.mine__filter--active {
  background: var(--color-border);
  color: var(--color-text);
  font-weight: 600;
}

.post-list {
  display: flex;
  flex-direction: column;
}

@media (max-width: 640px) {
  .mine__header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-3);
  }
}
</style>