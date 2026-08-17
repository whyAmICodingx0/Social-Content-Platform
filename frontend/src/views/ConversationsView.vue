<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { conversationsApi } from '../api'
import UserAvatar from '../components/UserAvatar.vue'
import PaginationNav from '../components/PaginationNav.vue'
import LoadingState from '../components/LoadingState.vue'
import { onMounted, onUnmounted } from 'vue'
import { useWsStore } from '../stores/ws'
import { useAuthStore } from '../stores/auth'

const ws = useWsStore()
const auth = useAuthStore()

let unsubscribe = null

const route = useRoute()
const router = useRouter()

const items = ref([])
const pagination = ref(null)
const loading = ref(true)
const error = ref(null)

const currentPage = computed(() => Number(route.query.page) || 1)

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await conversationsApi.list({ page: currentPage.value })
    items.value = res.data
    pagination.value = res.pagination
  } catch (err) {
    error.value = err.message
    items.value = []
    pagination.value = null
  } finally {
    loading.value = false
  }
}

watch(() => route.query.page, load, { immediate: true })

function goToPage(page) {
  router.push({ path: '/messages', query: { page } })
}

function nameOf(item) {
  return item.other_user.display_name || item.other_user.username
}

// 列表用相對時間：今天顯示時間、今年顯示月日、更早顯示年月日
function formatWhen(iso) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''

  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) {
    return new Intl.DateTimeFormat('zh-TW', {
      hour: '2-digit', minute: '2-digit', hour12: false,
    }).format(d)
  }
  if (d.getFullYear() === now.getFullYear()) {
    return new Intl.DateTimeFormat('zh-TW', { month: 'numeric', day: 'numeric' }).format(d)
  }
  return new Intl.DateTimeFormat('zh-TW', {
    year: 'numeric', month: 'numeric', day: 'numeric',
  }).format(d)
}

/**
 * 收到新訊息時更新列表：把該對話的最後一則訊息換掉、
 * 未讀數 +1、並把它移到最上面。
 *
 * 若該對話不在目前這一頁（例如在第 2 頁，或是全新的對話），
 * 就重新載入 —— 這種情況少見，直接重抓最單純。
 */
function onMessageCreated(data) {
  const i = items.value.findIndex((it) => it.id === data.conversation_id)

  if (i === -1) {
    // 不在目前頁面上的對話（新對話或在別頁）→ 只在第一頁時重載
    if (currentPage.value === 1) load()
    return
  }

  const item = items.value[i]
  const isMine = data.sender?.username === auth.user?.username

  item.last_message = {
    id: data.id,
    content: data.content,
    is_mine: isMine,
    created_at: data.created_at,
  }
  if (!isMine) item.unread_count += 1

  // 移到最上面（列表依最後訊息時間排序）
  items.value.splice(i, 1)
  items.value.unshift(item)
}

onMounted(() => {
  unsubscribe = ws.on('message.created', onMessageCreated)
})

onUnmounted(() => {
  if (unsubscribe) unsubscribe()
})
</script>

<template>
  <div class="container container--narrow">
    <header class="convs__header">
      <h1 class="convs__title">訊息</h1>
    </header>

    <LoadingState v-if="loading" />

    <p v-else-if="error" class="state state--error">{{ error }}</p>

    <div v-else-if="items.length === 0" class="empty">
      <h2 class="empty__title">還沒有任何對話</h2>
      <p class="empty__desc">
        到別人的個人頁點「傳訊息」就能開始聊天。
      </p>
      <RouterLink to="/" class="btn btn--primary">探索文章</RouterLink>
    </div>

    <template v-else>
      <ul class="convs">
        <li v-for="item in items" :key="item.id">
          <RouterLink
            :to="`/messages/@${item.other_user.username}`"
            class="conv"
            :class="{ 'conv--unread': item.unread_count > 0 }"
          >
            <UserAvatar
              :src="item.other_user.avatar_url"
              :name="nameOf(item)"
              :size="44"
            />
            <div class="conv__body">
              <div class="conv__top">
                <span class="conv__name">{{ nameOf(item) }}</span>
                <time class="conv__when">{{ formatWhen(item.last_message.created_at) }}</time>
              </div>
              <p class="conv__preview">
                <span v-if="item.last_message.is_mine" class="conv__prefix">你：</span>
                {{ item.last_message.content }}
              </p>
            </div>
            <span v-if="item.unread_count > 0" class="conv__badge">
              {{ item.unread_count > 99 ? '99+' : item.unread_count }}
            </span>
          </RouterLink>
        </li>
      </ul>

      <PaginationNav v-if="pagination" :pagination="pagination" @change="goToPage" />
    </template>
  </div>
</template>

<style scoped>
.convs__header {
  padding-bottom: var(--space-4);
  margin-bottom: var(--space-2);
  border-bottom: 1px solid var(--color-border);
}

.convs__title {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
}

.conv {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-2);
  border-bottom: 1px solid var(--color-border);
  transition: background-color 0.15s ease;
}

.conv:hover {
  background: var(--color-surface);
}

.conv__body {
  flex: 1;
  min-width: 0;
}

.conv__top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
}

.conv__name {
  font-weight: 600;
}

.conv__when {
  flex-shrink: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.conv__preview {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  /* 單行截斷，避免長訊息撐破版面 */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.conv--unread .conv__preview {
  color: var(--color-text);
  font-weight: 550;
}

.conv__prefix {
  opacity: 0.7;
}

.conv__badge {
  flex-shrink: 0;
  min-width: 22px;
  padding: 0 var(--space-2);
  border-radius: 11px;
  background: var(--color-accent);
  color: #fff;
  font-size: 0.75rem;
  line-height: 22px;
  text-align: center;
  font-variant-numeric: tabular-nums;
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
</style>