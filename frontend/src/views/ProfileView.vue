<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usersApi, postsApi } from '../api'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { formatDate } from '../utils/format'
import UserAvatar from '../components/UserAvatar.vue'
import PostCard from '../components/PostCard.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { setTitle } from '../utils/title'
import LoadingState from '../components/LoadingState.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const profile = ref(null)
const posts = ref([])
const pagination = ref(null)
const loading = ref(true)
const notFound = ref(false)
const error = ref(null)

const username = computed(() => route.params.username)
const currentPage = computed(() => Number(route.query.page) || 1)

const isMe = computed(
  () => auth.isAuthenticated && profile.value && auth.user.username === profile.value.username
)

const displayName = computed(
  () => profile.value?.display_name || profile.value?.username
)

// 「查無此人」只由 GET /users/{username} 決定：
// GET /users/{u}/posts 在使用者不存在時回 200 + 空陣列，不能拿來判斷。
async function loadProfile() {
  loading.value = true
  notFound.value = false
  error.value = null
  try {
    const res = await usersApi.get(username.value)
    profile.value = res.data
    setTitle(profile.value.display_name || profile.value.username)
  } catch (err) {
    profile.value = null
    if (err instanceof ApiError && err.status === 404) {
      notFound.value = true
    } else {
      error.value = err.message
    }
  } finally {
    loading.value = false
  }
}

// 文章列表失敗不該讓整頁壞掉，個人資料仍要能顯示
async function loadPosts() {
  try {
    const res = await postsApi.listByUser(username.value, { page: currentPage.value })
    posts.value = res.data
    pagination.value = res.pagination
  } catch {
    posts.value = []
    pagination.value = null
  }
}

watch(
  username,
  () => {
    loadProfile()
    loadPosts()
  },
  { immediate: true }
)

// 只換頁時不必重抓個人資料
watch(currentPage, loadPosts)

function goToPage(page) {
  router.push({ path: `/@${username.value}`, query: { page } })
}
</script>

<template>
  <div class="container container--narrow">
    <LoadingState v-if="loading" />

    <div v-else-if="notFound" class="state">
      <h1 class="notfound__title">找不到這位使用者</h1>
      <p>@{{ username }} 不存在，或帳號已停用。</p>
      <RouterLink to="/" class="btn btn--ghost">回首頁</RouterLink>
    </div>

    <p v-else-if="error" class="state state--error">{{ error }}</p>

    <template v-else-if="profile">
      <header class="profile">
        <UserAvatar :src="profile.avatar_url" :name="displayName" :size="72" />

        <div class="profile__info">
          <h1 class="profile__name">{{ displayName }}</h1>
          <p class="profile__handle">@{{ profile.username }}</p>
          <p v-if="profile.bio" class="profile__bio">{{ profile.bio }}</p>
          <p class="profile__meta">
            {{ formatDate(profile.created_at) }} 加入
            <template v-if="pagination">
              · 共 {{ pagination.total }} 篇文章
            </template>
          </p>
        </div>

        <RouterLink v-if="isMe" to="/settings" class="btn btn--ghost profile__edit">
          編輯個人檔案
        </RouterLink>
      </header>

      <p v-if="isMe" class="profile__note">
        這裡只顯示已發布的文章。草稿請到
        <RouterLink to="/me/posts" class="profile__link">我的文章</RouterLink>
        查看。
      </p>

      <p v-if="posts.length === 0" class="state">
        {{ isMe ? '你還沒有已發布的文章。' : '這位使用者還沒有發布文章。' }}
      </p>

      <template v-else>
        <div class="post-list">
          <PostCard v-for="post in posts" :key="post.id" :post="post" />
        </div>
        <PaginationNav v-if="pagination" :pagination="pagination" @change="goToPage" />
      </template>
    </template>
  </div>
</template>

<style scoped>
.notfound__title {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  margin-bottom: var(--space-3);
  color: var(--color-text);
}

.profile {
  display: flex;
  align-items: flex-start;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  border-bottom: 1px solid var(--color-border);
}

.profile__info {
  flex: 1;
  min-width: 0;
}

.profile__name {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  line-height: var(--leading-tight);
}

.profile__handle {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  margin-top: var(--space-1);
}

.profile__bio {
  margin-top: var(--space-3);
  line-height: var(--leading-normal);
  white-space: pre-wrap;
}

.profile__meta {
  margin-top: var(--space-3);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.profile__edit {
  flex-shrink: 0;
}

.profile__note {
  padding-top: var(--space-4);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.profile__link {
  color: var(--color-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.post-list {
  display: flex;
  flex-direction: column;
}

@media (max-width: 640px) {
  .profile {
    flex-direction: column;
    gap: var(--space-3);
  }
}
</style>