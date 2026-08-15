<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { postsApi } from '../api'
import { ApiError } from '../api/client'
import { renderMarkdown } from '../utils/markdown'
import { formatDate } from '../utils/format'
import { setTitle } from '../utils/title'
import LoadingState from '../components/LoadingState.vue'
import LikeButton from '../components/LikeButton.vue'

const route = useRoute()

const post = ref(null)
const loading = ref(true)
const notFound = ref(false)
const error = ref(null)

const authorName = computed(
  () => post.value?.author.display_name || post.value?.author.username
)

const authorUrl = computed(() => `/@${post.value?.author.username}`)

const displayDate = computed(
  () => post.value?.published_at || post.value?.created_at
)

// 渲染後的 HTML。computed 讓它只在 post 變動時重算。
const contentHtml = computed(() =>
  post.value ? renderMarkdown(post.value.content) : ''
)

const router = useRouter()
const auth = useAuthStore()

const deleting = ref(false)

// Post Detail 的 author 只有 username，用它比對即可
const isAuthor = computed(
  () => auth.isAuthenticated && post.value && auth.user.username === post.value.author.username
)

const editUrl = computed(
  () => `/@${post.value.author.username}/${post.value.slug}/edit`
)

async function handleDelete() {
  // MVP 用瀏覽器內建 confirm；之後想做自訂 Modal 再換
  if (!window.confirm(`確定要刪除「${post.value.title}」嗎？此操作無法復原。`)) return

  deleting.value = true
  try {
    await postsApi.remove(post.value.id)
    router.replace('/me/posts')
  } catch (err) {
    error.value = err.message
  } finally {
    deleting.value = false
  }
}

async function load() {
  loading.value = true
  notFound.value = false
  error.value = null
  post.value = null
  

  try {
    const res = await postsApi.get(route.params.username, route.params.slug)
    post.value = res.data
    setTitle(post.value.title)
  } catch (err) {
    // 404 涵蓋三種情況：文章不存在、已刪除、他人的草稿。
    // 後端刻意不區分（spec 7.4），前端也不該猜。
    if (err instanceof ApiError && err.status === 404) {
      notFound.value = true
      setTitle('找不到文章')
    } else {
      error.value = err.message
    }
  } finally {
    loading.value = false
  }
}

function onLikeUpdate({ count, liked }) {
  if (post.value) {
    post.value.like_count = count
    post.value.liked_by_me = liked
  }
}

watch(() => [route.params.username, route.params.slug], load, { immediate: true })
</script>

<template>
  <div class="container container--narrow">
    <LoadingState v-if="loading" />

    <div v-else-if="notFound" class="state">
      <h1 class="notfound__title">找不到這篇文章</h1>
      <p>它可能已被刪除，或是尚未公開的草稿。</p>
      <RouterLink to="/" class="btn btn--ghost">回首頁</RouterLink>
    </div>

    <p v-else-if="error" class="state state--error">{{ error }}</p>

    <article v-else-if="post" class="post">
      <header class="post__header">
        <h1 class="post__title">{{ post.title }}</h1>

        <div class="post__meta">
          <RouterLink :to="authorUrl" class="post__author">{{ authorName }}</RouterLink>
          <span class="post__dot">·</span>
          <time>{{ formatDate(displayDate) }}</time>
          <span v-if="post.status === 'draft'" class="post__badge">草稿</span>
        </div>

        <ul v-if="post.tags.length" class="post__tags">
          <li v-for="tag in post.tags" :key="tag">
            <RouterLink :to="{ path: '/', query: { tag } }" class="post__tag">
              {{ tag }}
            </RouterLink>
          </li>
        </ul>

        <div v-if="post.status === 'published'" class="post__engage">
          <LikeButton
            :post-id="post.id"
            :count="post.like_count"
            :liked="post.liked_by_me"
            size="md"
            @update="onLikeUpdate"
          />
          <span v-if="post.comment_count > 0" class="post__comment-count">
            {{ post.comment_count }} 則留言
          </span>
        </div>

        <div v-if="isAuthor" class="post__actions">
          <RouterLink :to="editUrl" class="btn btn--ghost">編輯</RouterLink>
          <button class="btn btn--ghost post__delete" :disabled="deleting" @click="handleDelete">
            {{ deleting ? '刪除中…' : '刪除' }}
          </button>
        </div>
      </header>

      <!--
        v-html 的內容已經過 renderMarkdown() 的 DOMPurify 消毒。
        絕不可改成直接綁未消毒的字串。
      -->
      <div class="prose" v-html="contentHtml"></div>
    </article>
  </div>
</template>

<style scoped>
.notfound__title {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  margin-bottom: var(--space-3);
  color: var(--color-text);
}

.post__header {
  padding-bottom: var(--space-6);
  margin-bottom: var(--space-8);
  border-bottom: 1px solid var(--color-border);
}

.post__title {
  font-family: var(--font-serif);
  font-size: var(--text-3xl);
  line-height: var(--leading-tight);
  margin-bottom: var(--space-4);
}

.post__meta {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.post__author {
  font-weight: 550;
  color: var(--color-text);
}

.post__author:hover {
  color: var(--color-accent);
}

.post__badge {
  padding: 0 var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: 3px;
  font-size: 0.75rem;
}

.post__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-top: var(--space-4);
}

.post__tag {
  display: inline-block;
  padding: var(--space-1) var(--space-2);
  border-radius: 3px;
  background: var(--color-border);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.post__tag:hover {
  color: var(--color-accent);
}

/* ============================================
   文章內文排版
   ⚠️ 必須用 :deep()——scoped CSS 依賴編譯時加上的
   data-v 屬性，而 v-html 是執行期插入的節點，
   拿不到那個屬性，一般 scoped 選擇器對它完全無效。
   ============================================ */
.prose {
  font-family: var(--font-serif);
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
}

.prose :deep(p) {
  margin-bottom: var(--space-6);
}

.prose :deep(h1),
.prose :deep(h2),
.prose :deep(h3),
.prose :deep(h4) {
  font-family: var(--font-ui);
  margin-top: var(--space-12);
  margin-bottom: var(--space-4);
  line-height: var(--leading-tight);
}

.prose :deep(h1) { font-size: var(--text-2xl); }
.prose :deep(h2) { font-size: var(--text-xl); }
.prose :deep(h3) { font-size: var(--text-lg); }
.prose :deep(h4) { font-size: var(--text-base); }

.prose :deep(a) {
  color: var(--color-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

/* base.css 把清單樣式清空了，這裡要還原 */
.prose :deep(ul),
.prose :deep(ol) {
  margin-bottom: var(--space-6);
  padding-left: var(--space-6);
}

.prose :deep(ul) { list-style: disc; }
.prose :deep(ol) { list-style: decimal; }

.prose :deep(li) {
  margin-bottom: var(--space-2);
}

.prose :deep(blockquote) {
  margin: var(--space-6) 0;
  padding-left: var(--space-4);
  border-left: 3px solid var(--color-accent);
  color: var(--color-text-muted);
  font-style: italic;
}

.prose :deep(code) {
  font-family: ui-monospace, "Cascadia Code", Consolas, monospace;
  font-size: 0.875em;
  padding: 0.15em 0.4em;
  background: var(--color-border);
  border-radius: 3px;
}

.prose :deep(pre) {
  margin: var(--space-6) 0;
  padding: var(--space-4);
  background: #2b2926;
  border-radius: var(--radius);
  overflow-x: auto;
}

.prose :deep(pre code) {
  padding: 0;
  background: none;
  color: #f0ece6;
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.prose :deep(img) {
  margin: var(--space-6) 0;
  border-radius: var(--radius);
}

.prose :deep(hr) {
  margin: var(--space-12) 0;
  border: none;
  border-top: 1px solid var(--color-border);
}

.prose :deep(table) {
  width: 100%;
  margin: var(--space-6) 0;
  border-collapse: collapse;
  font-family: var(--font-ui);
  font-size: var(--text-base);
}

.prose :deep(th),
.prose :deep(td) {
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  text-align: left;
}

.prose :deep(th) {
  background: var(--color-border);
  font-weight: 600;
}

.prose :deep(strong) { font-weight: 700; }
.prose :deep(em) { font-style: italic; }

.post__actions {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-4);
}

.post__delete:hover {
  border-color: var(--color-danger);
  color: var(--color-danger);
}

.post__engage {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  margin-top: var(--space-4);
}

.post__comment-count {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}
</style>