<script setup>
import { ref, computed } from 'vue'
import { commentsApi } from '../api'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { formatDate } from '../utils/format'
import UserAvatar from './UserAvatar.vue'

const props = defineProps({
  comment: { type: Object, required: true },
  // 文章作者的 username，用來判斷「版主刪除」權限
  postAuthorUsername: { type: String, required: true },
})

const emit = defineEmits(['updated', 'deleted'])

const auth = useAuthStore()

const editing = ref(false)
const draft = ref('')
const saving = ref(false)
const deleting = ref(false)
const error = ref(null)

const authorName = computed(
  () => props.comment.author.display_name || props.comment.author.username
)

const authorUrl = computed(() => `/@${props.comment.author.username}`)

// 權限判斷：用 username 比對（決策 #3：username 唯一且不可變更）
const isCommentAuthor = computed(
  () => auth.isAuthenticated && auth.user.username === props.comment.author.username
)

const isPostAuthor = computed(
  () => auth.isAuthenticated && auth.user.username === props.postAuthorUsername
)

// 只有留言作者能編輯（決策 #54）
const canEdit = computed(() => isCommentAuthor.value)

// 留言作者或文章作者都能刪除
const canDelete = computed(() => isCommentAuthor.value || isPostAuthor.value)

const canSave = computed(() => draft.value.trim() !== '' && !saving.value)

function startEdit() {
  draft.value = props.comment.content
  editing.value = true
  error.value = null
}

function cancelEdit() {
  editing.value = false
  error.value = null
}

async function save() {
  if (!canSave.value) return

  saving.value = true
  error.value = null

  try {
    const res = await commentsApi.update(props.comment.id, draft.value)
    emit('updated', res.data)
    editing.value = false
  } catch (err) {
    error.value = messageOf(err)
  } finally {
    saving.value = false
  }
}

async function remove() {
  const msg = isCommentAuthor.value
    ? '確定要刪除這則留言嗎？'
    : '確定要刪除這則留言嗎？（你是文章作者）'
  if (!window.confirm(msg)) return

  deleting.value = true
  error.value = null

  try {
    await commentsApi.remove(props.comment.id)
    emit('deleted', props.comment.id)
  } catch (err) {
    error.value = messageOf(err)
    deleting.value = false
  }
}

function messageOf(err) {
  if (!(err instanceof ApiError)) return '發生未預期的錯誤。'
  switch (err.code) {
    case 'VALIDATION_ERROR':
      return err.details?.fields?.content ?? '內容不符合規則。'
    case 'FORBIDDEN':
      return '你沒有權限執行這個操作。'
    case 'NOT_FOUND':
      return '這則留言已不存在。'
    case 'UNAUTHENTICATED':
      return '請先登入。'
    default:
      return err.message
  }
}
</script>

<template>
  <article class="comment">
    <UserAvatar
      :src="comment.author.avatar_url"
      :name="authorName"
      :size="36"
    />

    <div class="comment__body">
      <div class="comment__meta">
        <RouterLink :to="authorUrl" class="comment__author">{{ authorName }}</RouterLink>
        <span v-if="comment.author.username === postAuthorUsername" class="comment__badge">
          作者
        </span>
        <time class="comment__date">{{ formatDate(comment.created_at) }}</time>
        <span v-if="comment.edited" class="comment__edited">（已編輯）</span>
      </div>

      <template v-if="editing">
        <textarea
          v-model="draft"
          class="comment__textarea"
          rows="3"
          maxlength="1000"
        ></textarea>
        <div class="comment__actions">
          <button class="btn btn--primary btn--sm" :disabled="!canSave" @click="save">
            {{ saving ? '儲存中…' : '儲存' }}
          </button>
          <button class="btn btn--ghost btn--sm" :disabled="saving" @click="cancelEdit">
            取消
          </button>
        </div>
      </template>

      <template v-else>
        <p class="comment__content">{{ comment.content }}</p>
        <div v-if="canEdit || canDelete" class="comment__actions">
          <button v-if="canEdit" class="comment__link" @click="startEdit">編輯</button>
          <button
            v-if="canDelete"
            class="comment__link comment__link--danger"
            :disabled="deleting"
            @click="remove"
          >
            {{ deleting ? '刪除中…' : '刪除' }}
          </button>
        </div>
      </template>

      <p v-if="error" class="comment__error">{{ error }}</p>
    </div>
  </article>
</template>

<style scoped>
.comment {
  display: flex;
  gap: var(--space-3);
  padding: var(--space-4) 0;
  border-bottom: 1px solid var(--color-border);
}

.comment__body {
  flex: 1;
  min-width: 0;
}

.comment__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.comment__author {
  font-weight: 550;
  color: var(--color-text);
}

.comment__author:hover {
  color: var(--color-accent);
}

.comment__badge {
  padding: 0 var(--space-2);
  border-radius: 3px;
  background: var(--color-accent);
  color: #fff;
  font-size: 0.7rem;
}

.comment__edited {
  font-size: 0.75rem;
  opacity: 0.8;
}

/* white-space: pre-wrap 保留使用者輸入的換行；
   ⚠️ 這裡刻意用純文字渲染，不做 Markdown —— 留言不需要，
   也避免多一個 XSS 風險面。 */
.comment__content {
  line-height: var(--leading-normal);
  white-space: pre-wrap;
  overflow-wrap: break-word;
}

.comment__textarea {
  width: 100%;
  padding: var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font: inherit;
  line-height: var(--leading-normal);
  resize: vertical;
}

.comment__textarea:focus {
  outline: none;
  border-color: var(--color-accent);
}

.comment__actions {
  display: flex;
  gap: var(--space-3);
  margin-top: var(--space-2);
}

.comment__link {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.comment__link:hover {
  color: var(--color-accent);
}

.comment__link--danger:hover {
  color: var(--color-danger);
}

.comment__link:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.comment__error {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-danger);
}
</style>