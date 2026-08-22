<script setup>
import { computed } from 'vue'
import { formatRelativeTime } from '../utils/format'
import UserAvatar from './UserAvatar.vue'

const props = defineProps({
  notification: { type: Object, required: true },
  // 本次開啟面板時是否為未讀（高亮用，見 store 的 highlighted）
  highlight: { type: Boolean, default: false },
})

defineEmits(['select'])

const n = computed(() => props.notification)

// actor 為 null 代表觸發者已軟刪（決策 #91：列表不過濾）
const actorName = computed(() => {
  const a = n.value.actor
  if (!a) return '已刪除的使用者'
  return a.display_name || a.username
})

const actionText = computed(() => {
  switch (n.value.type) {
    case 'like': return '喜歡了你的文章'
    case 'comment': return '留言了你的文章'
    case 'follow': return '追蹤了你'
    default: return '有新動態'
  }
})

/**
 * 點下去要跳到哪。null 代表不可點。
 *
 * ⚠️ target.type 恆為 "post"（P4-1 的合約申報）——
 * comment 類型的 target 也是那篇文章，不需要 comment 分支。
 * target 為 null 代表 type=follow，或相關內容已被刪除。
 */
const to = computed(() => {
  if (n.value.type === 'follow') {
    return n.value.actor ? `/@${n.value.actor.username}` : null
  }
  return n.value.target?.url ?? null
})

const targetLabel = computed(() => {
  if (n.value.type === 'follow') return null
  return n.value.target?.title ?? '內容已不存在'
})

const isMissing = computed(
  () => n.value.type !== 'follow' && !n.value.target
)
</script>

<template>
  <li
    class="noti"
    :class="{ 'noti--unread': highlight, 'noti--disabled': !to }"
    @click="to && $emit('select', notification)"
  >
    <UserAvatar
      :src="n.actor?.avatar_url ?? null"
      :name="actorName"
      :size="36"
    />

    <div class="noti__body">
      <p class="noti__text">
        <strong>{{ actorName }}</strong>
        {{ actionText }}
      </p>

      <p v-if="targetLabel" class="noti__target" :class="{ 'noti__target--missing': isMissing }">
        {{ targetLabel }}
      </p>

      <time class="noti__time">{{ formatRelativeTime(n.created_at) }}</time>
    </div>

    <span v-if="highlight" class="noti__dot" aria-label="未讀"></span>
  </li>
</template>

<style scoped>
.noti {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-3);
  border-bottom: 1px solid var(--color-border);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.noti:hover {
  background: var(--color-bg);
}

.noti--unread {
  background: color-mix(in srgb, var(--color-accent) 6%, transparent);
}

.noti--unread:hover {
  background: color-mix(in srgb, var(--color-accent) 10%, transparent);
}

/* 內容已不存在 → 不可點 */
.noti--disabled {
  cursor: default;
}

.noti--disabled:hover {
  background: none;
}

.noti__body {
  flex: 1;
  min-width: 0;
}

.noti__text {
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.noti__target {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  /* 單行截斷，長標題不撐破面板 */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.noti__target--missing {
  font-style: italic;
  opacity: 0.7;
}

.noti__time {
  display: block;
  margin-top: var(--space-1);
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.noti__dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  margin-top: var(--space-2);
  border-radius: 50%;
  background: var(--color-accent);
}
</style>