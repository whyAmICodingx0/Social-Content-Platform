<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { postsApi } from '../api'
import PostCard from '../components/PostCard.vue'
import TagFilter from '../components/TagFilter.vue'
import PaginationNav from '../components/PaginationNav.vue'

const route = useRoute()
const router = useRouter()

const posts = ref([])
const pagination = ref(null)
const loading = ref(true)
const error = ref(null)

// 篩選狀態一律讀自網址（見 V-3-0）
const currentTag = computed(() => route.query.tag || '')
const currentPage = computed(() => Number(route.query.page) || 1)

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await postsApi.list({
      tag: currentTag.value,
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

// 監聽網址上的篩選參數；immediate 讓它在初次載入時也跑一次。
// 注意監聽的是「陣列裡的兩個值」而非整個 query 物件——
// 物件每次都是新的參考，會造成不必要的重複請求。
watch(() => [route.query.tag, route.query.page], load, { immediate: true })

function goToPage(page) {
  router.push({ path: '/', query: { ...route.query, page } })
}
</script>

<template>
  <div class="container container--narrow">
    <TagFilter :active="currentTag" />

    <p v-if="loading" class="state">載入中…</p>

    <p v-else-if="error" class="state state--error">
      {{ error }}
    </p>

    <div v-else-if="posts.length === 0" class="state">
      <template v-if="currentTag">
        「{{ currentTag }}」這個標籤底下還沒有文章。
      </template>
      <template v-else>
        還沒有任何文章。成為第一個發文的人吧！
      </template>
    </div>

    <template v-else>
      <div class="post-list">
        <PostCard v-for="post in posts" :key="post.id" :post="post" />
      </div>
      <PaginationNav
        v-if="pagination"
        :pagination="pagination"
        @change="goToPage"
      />
    </template>
  </div>
</template>

<style scoped>
.post-list {
  display: flex;
  flex-direction: column;
}

/* 第一張卡片不需要上方留白 */
.post-list :deep(.card:first-child) {
  padding-top: var(--space-8);
}
</style>