<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import { postsApi } from '../api'
import { ApiError } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { renderMarkdown } from '../utils/markdown'
import TagInput from '../components/TagInput.vue'
import { setTitle } from '../utils/title'
import LoadingState from '../components/LoadingState.vue'
import ImageUploadButton from '../components/ImageUploadButton.vue'

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
const textareaEl = ref(null) // 插入圖片時要讀游標位置

/* ---------- P5-3：文章媒體 ---------- */
// 這篇文章「可能」擁有的媒體：載入時從 Post Detail 的 media 來，上傳成功再加進來。
// 只存畫面需要的三個欄位。
const mediaItems = ref([]) // [{ id, purpose, url }]

// 編輯模式下，回應裡真的有 media 陣列才算「知道目前的媒體」。
// 不知道的話存檔時就不送 media_ids——否則等於送出空陣列，會把既有關聯全部清掉（決策 #162）。
const mediaKnown = ref(true)

const imageBusy = ref(false)
const MAX_MEDIA = 10 // 與後端相同（決策 #122）；前端只是提示，後端才是權威

// 存檔時實際送出的媒體（決策 #161）：
//   - 圖片：網址還在內文裡才算。使用者刪掉 ![](...) 之後，關聯也跟著拿掉
//   - 影片（P5-4）：是附件、不在內文裡，一律保留
const activeMedia = computed(() =>
  mediaItems.value.filter((m) => m.purpose !== 'post_image' || content.value.includes(m.url))
)
const mediaFull = computed(() => activeMedia.value.length >= MAX_MEDIA)

const previewHtml = computed(() => renderMarkdown(content.value))

// 圖片上傳中不能存檔：圖片會在上傳完成後才插進內文，先存的話那張圖就漏掉了
const canSave = computed(
  () =>
    title.value.trim() !== '' &&
    content.value.trim() !== '' &&
    !saving.value &&
    !imageBusy.value
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

    // P5-3：Post Detail 的 media 含圖片與影片（決策 #133）
    mediaKnown.value = Array.isArray(p.media)
    mediaItems.value = mediaKnown.value
      ? p.media.map((m) => ({ id: m.id, purpose: m.purpose, url: m.url }))
      : []
    setTitle(`編輯：${p.title}`)
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
  // P5-3：每次都送完整的一組（PATCH 是整組替換，P5-0 §5.4）
  if (mediaKnown.value) {
    payload.media_ids = activeMedia.value.map((m) => m.id)
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

/* ---------- P5-3：插入圖片 ---------- */
function onImageUploaded(media, fileName) {
  if (!mediaItems.value.some((m) => m.id === media.id)) {
    mediaItems.value.push({ id: media.id, purpose: media.purpose, url: media.url })
  }
  insertAtCaret(`![${altFromFileName(fileName)}](${media.url})`)
}

// 檔名當作預設的替代文字（alt）。去掉副檔名，以及會弄壞 Markdown 語法的字元。
// 使用者之後可以自己改成更好的描述。
function altFromFileName(name) {
  const base = (name || '').replace(/\.[^.]+$/, '')
  const cleaned = base
    .replace(/[[\]\\\r\n]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 100)
  return cleaned || '圖片'
}

// 插在「上傳完成那一刻」的游標位置（決策 #163）。
// textarea 失去焦點後仍會記得 selectionStart，所以按了按鈕也拿得到。
// 前後補空行，讓圖片自成一段——和文字黏在同一段的話，圖片會跟文字擠在同一個 <p> 裡。
function insertAtCaret(snippet) {
  const el = textareaEl.value
  const text = content.value
  const start = el ? el.selectionStart : text.length
  const end = el ? el.selectionEnd : text.length
  const before = text.slice(0, start)
  const after = text.slice(end)

  const lead = newlinesBefore(before)
  content.value = before + lead + snippet + newlinesAfter(after) + after
  tab.value = 'write'

  // 等 Vue 把新內容寫進 textarea 之後，把游標放在圖片語法的正後方（同 GitHub 的行為）
  nextTick(() => {
    if (!el) return
    const pos = before.length + lead.length + snippet.length
    el.focus()
    el.setSelectionRange(pos, pos)
  })
}

// 圖片前面需要補幾個換行，才能和前一段文字之間隔出一個空行
function newlinesBefore(before) {
  if (before === '' || before.endsWith('\n\n')) return ''
  return before.endsWith('\n') ? '\n' : '\n\n'
}

// 同上，圖片後面
function newlinesAfter(after) {
  if (after === '' || after.startsWith('\n\n')) return ''
  return after.startsWith('\n') ? '\n' : '\n\n'
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
    <LoadingState v-if="loading" />

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
          <ImageUploadButton
            :disabled="saving || mediaFull"
            @uploaded="onImageUploaded"
            @busy="imageBusy = $event"
          />
          <span v-if="activeMedia.length > 0" class="editor__media-count">
            {{ activeMedia.length }} / {{ MAX_MEDIA }}
          </span>
          <span class="editor__hint">支援 Markdown 語法</span>
        </div>

        <textarea
          v-show="tab === 'write'"
          ref="textareaEl"
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
  flex-wrap: wrap; /* P5-3：多了圖片按鈕與狀態訊息，窄螢幕時允許換行 */
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}

/* P5-3：已使用的媒體數，例如「3 / 10」 */
.editor__media-count {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
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

@media (max-width: 640px) {
  .editor__actions {
    flex-wrap: wrap;
  }

  .editor__textarea,
  .editor__preview {
    min-height: 320px;
  }
}
</style>