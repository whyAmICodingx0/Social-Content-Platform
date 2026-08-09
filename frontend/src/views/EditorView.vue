<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import { postsApi } from '../api'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { renderMarkdown } from '../utils/markdown'
import TagInput from '../components/TagInput.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const isEdit = computed(() => route.name === 'post-edit')

// 表單狀態
const postId = ref(null) // 只有編輯模式才有值；PATCH / DELETE 需要它
const title = ref('')
const content = ref('')
const tags = ref([])
const status = ref('draft')

// UI 狀態
const loading = ref(false)
const saving = ref(false)
const loadError = ref(null)
const formError = ref(null)
const fieldErrors = ref({})
const tab = ref('write') // 'write' | 'preview'

const previewHtml = computed(() => renderMarkdown(content.value))

const canSave = computed(
  () => title.value.trim() !== '' && content.value.trim() !== '' && !saving.value
)

/* ---------- 未儲存變更的偵測 ---------- */
// 用「快照比對」而不是 watch 設旗標：載入既有文章時不會被誤判成已修改。
const snapshot = ref('')
function snapshotOf() {
  return JSON.stringify([title.value, content.value, tags.value, status.value])
}
const dirty = computed(() => snapshotOf() !== snapshot.value)
snapshot.value = snapshotOf()

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('有尚未儲存的變更，確定要離開嗎？')
})

/* ---------- 編輯模式：載入既有內容 ---------- */
async function loadPost() {
  loading.value = true
  loadError.value = null
  try {
    // 用既有的「作者 + slug」端點讀取（見 V-5-0），回應裡的 id 供之後寫入使用
    const res = await postsApi.get(route.params.username, route.params.slug)
    const p = res.data

    // 提前擋下非作者（後端 PATCH 也會回 403，這裡只是讓體驗好一點）
    if (auth.user?.username !== p.author.username) {
      loadError.value = '你不是這篇文章的作者，無法編輯。'
      return
    }

    postId.value = p.id
    title.value = p.title
    content.value = p.content
    tags.value = [...p.tags]
    status.value = p.status
    snapshot.value = snapshotOf() // 重設基準，載入不算「已修改」
  } catch (err) {
    loadError.value =
      err instanceof ApiError && err.status === 404
        ? '找不到這篇文章。'
        : err.message
  } finally {
    loading.value = false
  }
}

if (isEdit.value) loadPost()

/* ---------- 儲存 ---------- */
async function save(nextStatus) {
  if (!canSave.value) return

  saving.value = true
  formError.value = null
  fieldErrors.value = {}

  const payload = {
    title: title.value.trim(),
    content: content.value,
    status: nextStatus,
    tags: tags.value,
  }

  try {
    const res = isEdit.value
      ? await postsApi.update(postId.value, payload)
      : await postsApi.create(payload)

    const p = res.data
    snapshot.value = snapshotOf() // 已存檔，解除離開警告
    router.replace(`/@${p.author.username}/${p.slug}`)
  } catch (err) {
    handleSaveError(err)
  } finally {
    saving.value = false
  }
}

function handleSaveError(err) {
  if (!(err instanceof ApiError)) {
    formError.value = '發生未預期的錯誤，請稍後再試。'
    return
  }
  switch (err.code) {
    case 'VALIDATION_ERROR':
      fieldErrors.value = err.details?.fields ?? {}
      formError.value = '有欄位不符合規則，請檢查後再送出。'
      break
    case 'SLUG_CONFLICT':
      formError.value = '無法為這個標題產生唯一網址，請稍微修改標題後再試。'
      break
    case 'FORBIDDEN':
      formError.value = '你不是這篇文章的作者。'
      break
    case 'NOT_FOUND':
      formError.value = '這篇文章已不存在。'
      break
    case 'UNAUTHENTICATED':
      router.push({ name: 'login', query: { redirect: route.fullPath } })
      break
    default:
      formError.value = err.message
  }
}

/* ---------- 標題文字：依模式與狀態變化 ---------- */
const pageTitle = computed(() => {
  if (!isEdit.value) return '寫新文章'
  return status.value === 'draft' ? '編輯草稿' : '編輯文章'
})

// content 有變動時自動切回編輯分頁（避免使用者在預覽分頁打字卻看不到）
watch(content, () => {
  if (tab.value === 'preview' && document.activeElement?.tagName === 'TEXTAREA') {
    tab.value = 'write'
  }
})
</script>

<template>
  <div class="container container--narrow">
    <p v-if="loading" class="state">載入中…</p>

    <div v-else-if="loadError" class="state state--error">
      <p>{{ loadError }}</p>
      <RouterLink to="/" class="btn btn--ghost">回首頁</RouterLink>
    </div>

    <div v-else class="editor">
      <h1 class="editor__heading">{{ pageTitle }}</h1>

      <!-- 標題 -->
      <div class="field">
        <label class="field__label" for="title">標題</label>
        <input
          id="title"
          v-model="title"
          class="field__input field__input--title"
          type="text"
          placeholder="一個好標題"
          maxlength="200"
        />
        <p v-if="fieldErrors.title" class="field__error">{{ fieldErrors.title }}</p>
      </div>

      <!-- 內文：編輯 / 預覽 -->
      <div class="field">
        <div class="editor__tabs">
          <button
            type="button"
            class="editor__tab"
            :class="{ 'editor__tab--active': tab === 'write' }"
            @click="tab = 'write'"
          >
            編輯
          </button>
          <button
            type="button"
            class="editor__tab"
            :class="{ 'editor__tab--active': tab === 'preview' }"
            @click="tab = 'preview'"
          >
            預覽
          </button>
          <span class="editor__hint">支援 Markdown 語法</span>
        </div>

        <textarea
          v-show="tab === 'write'"
          v-model="content"
          class="editor__textarea"
          placeholder="開始寫吧。可以使用 # 標題、**粗體**、- 清單…"
          spellcheck="false"
        ></textarea>

        <!-- 預覽用的是與文章頁完全相同的 .prose 樣式與消毒流程 -->
        <div v-show="tab === 'preview'" class="editor__preview">
          <div v-if="content.trim()" class="prose" v-html="previewHtml"></div>
          <p v-else class="state">還沒有內容可以預覽。</p>
        </div>

        <p v-if="fieldErrors.content" class="field__error">{{ fieldErrors.content }}</p>
      </div>

      <!-- 標籤 -->
      <div class="field">
        <label class="field__label">標籤</label>
        <TagInput v-model="tags" />
        <p v-if="fieldErrors.tags" class="field__error">{{ fieldErrors.tags }}</p>
      </div>

      <p v-if="formError" class="form__error">{{ formError }}</p>

      <!-- 動作列 -->
      <div class="editor__actions">
        <template v-if="status === 'published'">
          <button class="btn btn--primary" :disabled="!canSave" @click="save('published')">
            {{ saving ? '儲存中…' : '儲存變更' }}
          </button>
          <button class="btn btn--ghost" :disabled="!canSave" @click="save('draft')">
            改回草稿
          </button>
        </template>
        <template v-else>
          <button class="btn btn--primary" :disabled="!canSave" @click="save('published')">
            {{ saving ? '發布中…' : '發布' }}
          </button>
          <button class="btn btn--ghost" :disabled="!canSave" @click="save('draft')">
            {{ isEdit ? '儲存草稿' : '存成草稿' }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.editor__heading {
  font-family: var(--font-serif);
  font-size: var(--text-2xl);
  margin-bottom: var(--space-8);
}

.field {
  margin-bottom: var(--space-8);
}

.field__label {
  display: block;
  margin-bottom: var(--space-2);
  font-size: var(--text-sm);
  font-weight: 600;
}

.field__input {
  width: 100%;
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
}

.field__input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.field__input--title {
  font-family: var(--font-serif);
  font-size: var(--text-xl);
}

.field__error {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-danger);
}

.editor__tabs {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}

.editor__tab {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.editor__tab--active {
  background: var(--color-border);
  color: var(--color-text);
  font-weight: 600;
}

.editor__hint {
  margin-left: auto;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.editor__textarea {
  width: 100%;
  min-height: 420px;
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font-family: ui-monospace, "Cascadia Code", Consolas, monospace;
  font-size: var(--text-base);
  line-height: var(--leading-relaxed);
  resize: vertical;
}

.editor__textarea:focus {
  outline: none;
  border-color: var(--color-accent);
}

.editor__preview {
  min-height: 420px;
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
}

.form__error {
  margin-bottom: var(--space-4);
  padding: var(--space-3);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius);
  color: var(--color-danger);
  font-size: var(--text-sm);
}

.editor__actions {
  display: flex;
  gap: var(--space-3);
  padding-top: var(--space-4);
  border-top: 1px solid var(--color-border);
}

.editor__actions .btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>