<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useNotificationsStore } from '../stores/notifications'
import { useWsStore } from '../stores/ws'
import NotificationItem from './NotificationItem.vue'
import LoadingState from './LoadingState.vue'

const router = useRouter()
const notifications = useNotificationsStore()
const ws = useWsStore()

const open = ref(false)
const rootEl = ref(null)
let unsubscribeOpen = null

async function openPanel() {
  open.value = true

  // 每次開啟都重抓，確保看到的是最新狀態
  await notifications.loadList()

  // 決策 P4-0 §3.8：開啟面板 → read-all（最常見的操作）。
  // 已經是 0 就不必多打一次 API。
  if (notifications.unreadCount > 0) {
    await notifications.markAllRead()
  }

  await nextTick()
  document.addEventListener('keydown', onKeydown)
  document.addEventListener('mousedown', onClickOutside)
}

function closePanel() {
  open.value = false
  // 關閉時清掉高亮快照 —— 下次開啟時那些項目已經是已讀，不該再高亮
  notifications.clearHighlight()
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('mousedown', onClickOutside)
}

function toggle() {
  open.value ? closePanel() : openPanel()
}

function onKeydown(e) {
  if (e.key === 'Escape') closePanel()
}

function onClickOutside(e) {
  if (rootEl.value && !rootEl.value.contains(e.target)) closePanel()
}

/**
 * 點擊通知：先標記已讀（若未讀），關閉面板，再跳轉。
 *
 * 「面板開著時才進來的那一則」不會被開啟時的 read-all 涵蓋，
 * 所以這裡的單筆 markRead 是必要的，不是冗餘。
 */
function onSelect(n) {
  const to = n.type === 'follow'
    ? (n.actor ? `/@${n.actor.username}` : null)
    : (n.target?.url ?? null)

  if (!n.is_read) notifications.markRead([n.id])
  closePanel()
  if (to) router.push(to)
}

onMounted(() => {
  // 決策 #90：WS 每次（重）連線成功後補齊。
  // 面板開著時連列表一起重抓 —— 斷線期間的通知才會出現。
  unsubscribeOpen = ws.on('open', () => {
    if (open.value) notifications.loadList()
  })
})

onUnmounted(() => {
  if (unsubscribeOpen) unsubscribeOpen()
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('mousedown', onClickOutside)
})
</script>

<template>
  <div ref="rootEl" class="bell">
    <button
      class="bell__btn"
      :aria-expanded="open"
      aria-haspopup="true"
      :aria-label="`通知${notifications.unreadCount > 0 ? `（${notifications.unreadCount} 則未讀）` : ''}`"
      @click="toggle"
    >
      <svg class="bell__icon" viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M12 3a6 6 0 0 0-6 6v3.6L4.5 15.5A1 1 0 0 0 5.4 17h13.2a1 1 0 0 0 .9-1.5L18 12.6V9a6 6 0 0 0-6-6z"
          fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"
        />
        <path
          d="M9.5 19a2.5 2.5 0 0 0 5 0"
          fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"
        />
      </svg>
      <span v-if="notifications.unreadCount > 0" class="bell__badge">
        {{ notifications.unreadCount > 99 ? '99+' : notifications.unreadCount }}
      </span>
    </button>

    <div v-if="open" class="panel" role="dialog" aria-label="通知">
      <header class="panel__header">
        <h2 class="panel__title">通知</h2>
        <button class="panel__close" aria-label="關閉" @click="closePanel">×</button>
      </header>

      <LoadingState v-if="notifications.loading" text="載入通知中" />

      <p v-else-if="notifications.error" class="panel__state panel__state--error">
        {{ notifications.error }}
      </p>

      <p v-else-if="notifications.items.length === 0" class="panel__state">
        還沒有任何通知。
      </p>

      <template v-else>
        <ul class="panel__list">
          <NotificationItem
            v-for="n in notifications.items"
            :key="n.id"
            :notification="n"
            :highlight="notifications.highlighted.has(n.id)"
            @select="onSelect"
          />
        </ul>

        <button
          v-if="notifications.hasMore"
          class="panel__more"
          :disabled="notifications.loadingMore"
          @click="notifications.loadMore()"
        >
          {{ notifications.loadingMore ? '載入中…' : '載入更多' }}
        </button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.bell {
  position: relative;
  display: inline-flex;
}

.bell__btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: var(--space-1);
  border-radius: var(--radius);
  color: var(--color-text-muted);
  transition: color 0.15s ease;
}

.bell__btn:hover {
  color: var(--color-accent);
}

.bell__icon {
  width: 20px;
  height: 20px;
}

.bell__badge {
  position: absolute;
  top: -2px;
  right: -6px;
  min-width: 17px;
  padding: 0 4px;
  border-radius: 9px;
  background: var(--color-danger);
  color: #fff;
  font-size: 0.65rem;
  line-height: 17px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.panel {
  position: absolute;
  top: calc(100% + var(--space-2));
  right: 0;
  z-index: 50;
  width: 360px;
  max-height: 70vh;
  overflow-y: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.panel__header {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}

.panel__title {
  font-size: var(--text-base);
  font-weight: 650;
}

.panel__close {
  padding: 0 var(--space-2);
  font-size: var(--text-lg);
  line-height: 1;
  color: var(--color-text-muted);
}

.panel__close:hover {
  color: var(--color-text);
}

.panel__state {
  padding: var(--space-8) var(--space-4);
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.panel__state--error {
  color: var(--color-danger);
}

.panel__more {
  display: block;
  width: 100%;
  padding: var(--space-3);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.panel__more:hover {
  color: var(--color-accent);
}

.panel__more:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 手機：面板改為近乎全螢幕，避免超出視窗 */
@media (max-width: 640px) {
  .bell {
    position: static;
  }

  .panel {
    position: fixed;
    top: auto;
    left: var(--space-3);
    right: var(--space-3);
    width: auto;
    max-height: 60vh;
  }
}
</style>