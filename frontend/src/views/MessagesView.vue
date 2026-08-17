<script setup>
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { conversationsApi } from '../api'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { useWsStore } from '../stores/ws'
import { newClientMessageId } from '../utils/uuid'
import { setTitle } from '../utils/title'
import UserAvatar from '../components/UserAvatar.vue'
import LoadingState from '../components/LoadingState.vue'
import ConnectionStatus from '../components/ConnectionStatus.vue'
import { fetchMessagesAfter } from '../utils/messages'

const route = useRoute()
const auth = useAuthStore()
const ws = useWsStore()

const conversation = ref(null)
const messages = ref([])
const loading = ref(true)
const loadError = ref(null)

const draft = ref('')
const sending = ref(false)
const sendError = ref(null)

const listEl = ref(null)
const loadingOlder = ref(false)
const hasOlder = ref(false)
let catchingUp = false
let unsubscribeOpen = null
let unsubscribe = null

const MAX = 2000
// 用展開運算子計算長度，與後端的 rune 計數一致
const draftLength = computed(() => [...draft.value].length)
const overflow = computed(() => draftLength.value > MAX)
const canSend = computed(
  () => draft.value.trim() !== '' && !overflow.value && !sending.value && conversation.value
)

const otherName = computed(
  () => conversation.value?.other_user.display_name || conversation.value?.other_user.username
)

function scrollToBottom() {
  nextTick(() => {
    if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
  })
}

/**
 * 收到 WS 事件或 HTTP 回應時，把訊息併入本地列表。
 *
 * 去重規則（決策，寫死）：
 *  1. id 已存在 → 忽略（自己送出的那個分頁已經有了）
 *  2. client_message_id 對應到某則 pending → 替換
 *  3. 否則 → 新增
 */
function upsertMessage(msg) {
  const byId = messages.value.findIndex((m) => m.id === msg.id)
  if (byId !== -1) return

  const byClientId = messages.value.findIndex(
    (m) => m.pending && m.client_message_id === msg.client_message_id
  )
  if (byClientId !== -1) {
    messages.value[byClientId] = { ...msg, pending: false, failed: false }
    return
  }

  messages.value.push({ ...msg, pending: false, failed: false })
  scrollToBottom()
}

/**
 * 首次載入：取最新的一批訊息。
 * 回應已是遞增排序（最舊在前），直接使用。
 */
async function loadInitialMessages() {
  const res = await conversationsApi.messages(conversation.value.id, { limit: 30 })
  messages.value = res.data.map((m) => ({ ...m, pending: false, failed: false }))
  hasOlder.value = res.has_more
  scrollToBottom()
}

/**
 * 往上捲載入更舊的訊息。
 *
 * 捲動位置的處理：插入內容到列表最前面會讓可視區域「往下跳」，
 * 所以先記錄 scrollHeight，插入後補回差值，讓使用者停在原本看的位置。
 */
async function loadOlder() {
  if (loadingOlder.value || !hasOlder.value || messages.value.length === 0) return

  loadingOlder.value = true
  const el = listEl.value
  const prevHeight = el ? el.scrollHeight : 0

  try {
    const oldest = messages.value.find((m) => !m.pending)
    if (!oldest) return

    const res = await conversationsApi.messages(conversation.value.id, {
      before: oldest.id,
      limit: 30,
    })

    const older = res.data.map((m) => ({ ...m, pending: false, failed: false }))
    messages.value = [...older, ...messages.value]
    hasOlder.value = res.has_more

    await nextTick()
    if (el) el.scrollTop = el.scrollHeight - prevHeight
  } catch (err) {
    sendError.value = '載入歷史訊息失敗。'
  } finally {
    loadingOlder.value = false
  }
}

/**
 * 重連後補齊斷線期間漏掉的訊息（決策 #68）。
 *
 * WS 不保證送達（決策 #62），PostgreSQL 才是權威 ——
 * 所以每次連線建立（含首次與重連）都要用 ?after= 補一次。
 */
async function catchUp() {
  if (catchingUp || !conversation.value) return

  const known = [...messages.value].reverse().find((m) => !m.pending)
  if (!known) {
    // 本地一則都沒有（例如首次載入失敗）→ 直接重載
    try {
      await loadInitialMessages()
    } catch {
      /* 靜默：連線指示燈已經反映狀態 */
    }
    return
  }

  catchingUp = true
  try {
    const missed = await fetchMessagesAfter(conversation.value.id, known.id)
    missed.forEach(upsertMessage)
  } catch {
    /* 補抓失敗不打斷使用；下次重連會再試一次 */
  } finally {
    catchingUp = false
  }
}

// 捲到頂端附近就載入更舊的
function onScroll() {
  if (listEl.value && listEl.value.scrollTop < 80) loadOlder()
}

async function load() {
  loading.value = true
  loadError.value = null
  try {
    // 冪等的 find-or-create：既有對話回 200，第一次開啟則建立並回 201
    const res = await conversationsApi.findOrCreate(route.params.username)
    conversation.value = res.data
    setTitle(`與 ${otherName.value} 的對話`)
  } catch (err) {
    if (err instanceof ApiError) {
      switch (err.code) {
        case 'NOT_FOUND':
          loadError.value = '找不到這位使用者。'
          break
        case 'CANNOT_MESSAGE_SELF':
          loadError.value = '你不能和自己對話。'
          break
        default:
          loadError.value = err.message
      }
    } else {
      loadError.value = '發生未預期的錯誤。'
    }
  } finally {
    loading.value = false
  }
}

async function send() {
  if (!canSend.value) return

  const content = draft.value
  const clientMessageId = newClientMessageId()

  // 樂觀顯示：先放一則 pending 訊息，讓送出有立即回饋
  const optimistic = {
    id: `pending:${clientMessageId}`,
    client_message_id: clientMessageId,
    content,
    sender: {
      username: auth.user.username,
      display_name: auth.user.display_name,
      avatar_url: auth.user.avatar_url,
    },
    created_at: new Date().toISOString(),
    pending: true,
    failed: false,
  }
  messages.value.push(optimistic)
  draft.value = ''
  scrollToBottom()

  sending.value = true
  sendError.value = null

  try {
    const res = await conversationsApi.sendMessage(
      conversation.value.id, clientMessageId, content)
    upsertMessage(res.data)
  } catch (err) {
    // 標記失敗而非移除 —— 使用者打的字不該憑空消失
    const i = messages.value.findIndex((m) => m.client_message_id === clientMessageId)
    if (i !== -1) messages.value[i].failed = true

    if (err instanceof ApiError && err.code === 'VALIDATION_ERROR') {
      sendError.value = err.details?.fields?.content
        ?? err.details?.fields?.client_message_id
        ?? '內容不符合規則。'
    } else {
      sendError.value = '訊息傳送失敗，請再試一次。'
    }
  } finally {
    sending.value = false
  }
}

// 重送失敗的訊息：沿用同一個 client_message_id，
// 所以就算前一次其實已寫入成功，後端也會走冪等路徑回 200，不會重複。
async function retry(msg) {
  const i = messages.value.findIndex((m) => m.client_message_id === msg.client_message_id)
  if (i === -1) return

  messages.value[i].failed = false
  messages.value[i].pending = true
  sendError.value = null

  try {
    const res = await conversationsApi.sendMessage(
      conversation.value.id, msg.client_message_id, msg.content)
    upsertMessage(res.data)
  } catch {
    messages.value[i].failed = true
    messages.value[i].pending = false
    sendError.value = '訊息傳送失敗，請再試一次。'
  }
}

function isMine(msg) {
  return msg.sender.username === auth.user?.username
}

function formatTime(iso) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return new Intl.DateTimeFormat('zh-TW', {
    hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(d)
}

onMounted(async () => {
  await load()
  if (!conversation.value) return

  try {
    await loadInitialMessages()
  } catch (err) {
    loadError.value = '載入歷史訊息失敗。'
    return
  }

  // 訂閱即時訊息。只處理屬於本對話的事件 ——
  // 同一條 WS 連線會收到所有對話的訊息。
  unsubscribe = ws.on('message.created', (data) => {
    if (!conversation.value || data.conversation_id !== conversation.value.id) return
    upsertMessage(data)
  })

  // ⚠️ 每次連線建立（含重連）都補抓一次。
  // 只在 onMounted 補一次是不夠的 —— 斷線期間的訊息就是在
  // 「重連的那一刻」才需要補。
  unsubscribeOpen = ws.on('open', catchUp)

  // 若進頁時 WS 已經是連線狀態（不會再觸發 open），主動補一次
  if (ws.status === 'open') catchUp()
})

onUnmounted(() => {
  if (unsubscribe) unsubscribe()
  if (unsubscribeOpen) unsubscribeOpen()
})
</script>

<template>
  <div class="container container--narrow">
    <LoadingState v-if="loading" />

    <div v-else-if="loadError" class="state state--error">
      <p>{{ loadError }}</p>
      <RouterLink to="/" class="btn btn--ghost">回首頁</RouterLink>
    </div>

    <div v-else-if="conversation" class="chat">
      <header class="chat__header">
        <RouterLink :to="`/@${conversation.other_user.username}`" class="chat__peer">
          <UserAvatar
            :src="conversation.other_user.avatar_url"
            :name="otherName"
            :size="40"
          />
          <span class="chat__peer-name">{{ otherName }}</span>
        </RouterLink>
        <ConnectionStatus />
      </header>

      <div ref="listEl" class="chat__list" @scroll="onScroll">
        <p v-if="loadingOlder" class="chat__older">載入更舊的訊息…</p>
        <p v-else-if="hasOlder" class="chat__older">往上捲以載入更舊的訊息</p>
        <p v-else-if="messages.length > 0" class="chat__older">已是最開始</p>

        <p v-if="messages.length === 0" class="state">
          還沒有訊息，說點什麼吧。
        </p>

        <div
          v-for="m in messages"
          :key="m.id"
          class="msg"
          :class="{
            'msg--mine': isMine(m),
            'msg--pending': m.pending,
            'msg--failed': m.failed,
          }"
        >
          <div class="msg__bubble">
            <p class="msg__content">{{ m.content }}</p>
            <div class="msg__meta">
              <time>{{ formatTime(m.created_at) }}</time>
              <span v-if="m.pending">傳送中…</span>
              <button v-if="m.failed" class="msg__retry" @click="retry(m)">重試</button>
            </div>
          </div>
        </div>
      </div>

      <div class="composer">
        <textarea
          v-model="draft"
          class="composer__input"
          rows="2"
          placeholder="輸入訊息…（Enter 送出，Shift+Enter 換行）"
          @keydown.enter.exact.prevent="send"
        ></textarea>
        <div class="composer__side">
          <span class="composer__counter" :class="{ 'composer__counter--over': overflow }">
            {{ draftLength }} / {{ MAX }}
          </span>
          <button class="btn btn--primary btn--sm" :disabled="!canSend" @click="send">
            送出
          </button>
        </div>
      </div>

      <p v-if="sendError" class="chat__error">{{ sendError }}</p>
    </div>
  </div>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 200px);
  min-height: 420px;
}

.chat__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.chat__peer {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.chat__peer-name {
  font-family: var(--font-serif);
  font-size: var(--text-lg);
  font-weight: 600;
}

.chat__peer:hover .chat__peer-name {
  color: var(--color-accent);
}

.chat__list {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-4) 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.msg {
  display: flex;
  justify-content: flex-start;
}

.msg--mine {
  justify-content: flex-end;
}

.msg__bubble {
  max-width: 75%;
  padding: var(--space-2) var(--space-3);
  border-radius: 12px;
  background: var(--color-border);
}

.msg--mine .msg__bubble {
  background: var(--color-accent);
  color: #fff;
}

.msg--pending .msg__bubble {
  opacity: 0.6;
}

.msg--failed .msg__bubble {
  background: var(--color-danger);
  color: #fff;
}

/* 純文字渲染，保留換行但不解析 Markdown（決策 #75）—— 少一個 XSS 面 */
.msg__content {
  white-space: pre-wrap;
  overflow-wrap: break-word;
  line-height: var(--leading-normal);
}

.msg__meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-1);
  font-size: 0.75rem;
  opacity: 0.75;
}

.msg__retry {
  text-decoration: underline;
  font-size: 0.75rem;
  color: inherit;
}

.composer {
  display: flex;
  gap: var(--space-3);
  padding-top: var(--space-4);
  border-top: 1px solid var(--color-border);
}

.composer__input {
  flex: 1;
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font: inherit;
  line-height: var(--leading-normal);
  resize: none;
}

.composer__input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.composer__side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-2);
}

.composer__counter {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}

.composer__counter--over {
  color: var(--color-danger);
}

.chat__error {
  padding-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-danger);
}

.chat__older {
  padding: var(--space-2) 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  text-align: center;
}

.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

@media (max-width: 640px) {
  .chat {
    height: calc(100vh - 260px);
  }

  .msg__bubble {
    max-width: 85%;
  }
}
</style>