<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { authApi } from '../api'
import ConnectionStatus from './ConnectionStatus.vue'
import NotificationBell from './NotificationBell.vue'
import { useUnreadStore } from '../stores/unread'
import { ref } from 'vue'

const searchDraft = ref('')

function submitSearch() {
  const term = searchDraft.value.trim()
  if (!term) return
  router.push({ path: '/search', query: { q: term } })
  searchDraft.value = ''
}

const unread = useUnreadStore()

const auth = useAuthStore()
const router = useRouter()

function login() {
  // OAuth 是整頁導向，不是 fetch
  window.location.href = authApi.loginUrl
}

async function handleLogout() {
  await auth.logout()
  // 若當前在需要登入的頁面，留在原地會看到錯誤畫面
  router.push('/')
}
</script>

<template>
  <header class="header">
    <div class="container header__inner">
      <RouterLink to="/" class="header__logo">Inkwell</RouterLink>

      <form class="header__search" @submit.prevent="submitSearch">
        <input
          v-model="searchDraft"
          class="header__search-input"
          type="search"
          placeholder="搜尋"
          aria-label="搜尋"
        />
      </form>

      <nav class="header__nav">
        <!-- ready 之前不顯示，避免閃過錯誤狀態 -->
        <template v-if="auth.ready">
          <template v-if="auth.isAuthenticated">
            <ConnectionStatus />
            <NotificationBell />
            <RouterLink to="/feed" class="header__link">追蹤動態</RouterLink>
            <RouterLink to="/messages" class="header__link header__link--badge">
              訊息
              <span v-if="unread.count > 0" class="header__badge">
                {{ unread.count > 99 ? '99+' : unread.count }}
              </span>
            </RouterLink>
            <RouterLink to="/new" class="header__link">寫文章</RouterLink>
            <RouterLink to="/me/posts" class="header__link">我的文章</RouterLink>
            <RouterLink :to="`/@${auth.user.username}`" class="header__user">
              {{ auth.user.display_name || auth.user.username }}
            </RouterLink>
            <button class="btn btn--ghost" @click="handleLogout">登出</button>
          </template>
          <button v-else class="btn btn--primary" @click="login">
            使用 Google 登入
          </button>
        </template>
      </nav>
    </div>
  </header>
</template>

<style scoped>
.header {
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}

.header__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
}

.header__logo {
  font-family: var(--font-serif);
  font-size: var(--text-xl);
  font-weight: 700;
  letter-spacing: -0.01em;
}

.header__nav {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.header__link {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.header__link:hover {
  color: var(--color-accent);
}

/* vue-router 會自動為當前頁面的連結加上這個 class */
.header__link.router-link-active {
  color: var(--color-text);
  font-weight: 600;
}

.header__user {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.header__user:hover {
  color: var(--color-accent);
}

@media (max-width: 640px) {
  .header__inner {
    height: auto;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-3) 0;
  }

  .header__nav {
    width: 100%;
    flex-wrap: wrap;
    gap: var(--space-3);
    font-size: var(--text-sm);
  }

  .header__user {
    margin-left: auto;
  }
}

.header__link--badge {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
}

.header__badge {
  min-width: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--color-danger);
  color: #fff;
  font-size: 0.7rem;
  line-height: 18px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.header__search {
  flex: 1;
  max-width: 260px;
  margin: 0 var(--space-4);
}

.header__search-input {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-bg);
  font: inherit;
  font-size: var(--text-sm);
}

.header__search-input:focus {
  outline: none;
  border-color: var(--color-accent);
  background: var(--color-surface);
}

@media (max-width: 640px) {
  .header__search {
    max-width: none;
    width: 100%;
    margin: 0;
    order: 3;   /* 手機版排到導覽列下方 */
  }
}
</style>