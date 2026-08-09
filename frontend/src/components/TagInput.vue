<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue'])

const MAX_TAGS = 5
const draft = ref('')
const hint = ref('')

const isFull = computed(() => props.modelValue.length >= MAX_TAGS)

/**
 * 前端先做一次正規化，讓使用者馬上看到實際會被存成什麼。
 * 規則刻意與後端 8.4 一致：小寫 → 非英數轉 '-' → 收斂 → 去頭尾 → 截斷 50。
 * 【注意】這只是即時回饋，後端仍會再正規化一次，權威在後端。
 */
function normalize(raw) {
  return raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 50)
    .replace(/^-+|-+$/g, '')
}

function addTag() {
  const value = normalize(draft.value)
  hint.value = ''

  if (draft.value.trim() !== '' && value === '') {
    // 中文或純符號的標籤正規化後會變空字串（spec 8.4 的已知限制）
    hint.value = '標籤目前僅支援英文字母、數字與底線'
    return
  }
  if (value === '') return

  if (props.modelValue.includes(value)) {
    hint.value = '這個標籤已經加過了'
    draft.value = ''
    return
  }
  if (isFull.value) {
    hint.value = `最多只能有 ${MAX_TAGS} 個標籤`
    return
  }

  emit('update:modelValue', [...props.modelValue, value])
  draft.value = ''
}

function removeTag(tag) {
  hint.value = ''
  emit('update:modelValue', props.modelValue.filter((t) => t !== tag))
}

// 按 Backspace 且輸入框為空時，刪掉最後一個標籤（常見的輸入習慣）
function onBackspace() {
  if (draft.value === '' && props.modelValue.length > 0) {
    removeTag(props.modelValue[props.modelValue.length - 1])
  }
}
</script>

<template>
  <div class="tag-input">
    <div class="tag-input__box">
      <span v-for="tag in modelValue" :key="tag" class="chip">
        {{ tag }}
        <button type="button" class="chip__remove" @click="removeTag(tag)">×</button>
      </span>

      <input
        v-model="draft"
        class="tag-input__field"
        type="text"
        :placeholder="isFull ? '已達上限' : '輸入後按 Enter 新增'"
        :disabled="isFull"
        autocapitalize="none"
        @keydown.enter.prevent="addTag"
        @keydown.,.prevent="addTag"
        @keydown.delete="onBackspace"
        @blur="addTag"
      />
    </div>
    <p class="tag-input__hint" :class="{ 'tag-input__hint--error': hint }">
      {{ hint || `最多 ${MAX_TAGS} 個，按 Enter 或逗號新增` }}
    </p>
  </div>
</template>

<style scoped>
.tag-input__box {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
}

.tag-input__box:focus-within {
  border-color: var(--color-accent);
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  border-radius: 3px;
  background: var(--color-border);
  font-size: var(--text-sm);
}

.chip__remove {
  padding: 0 2px;
  color: var(--color-text-muted);
  line-height: 1;
}

.chip__remove:hover {
  color: var(--color-danger);
}

.tag-input__field {
  flex: 1;
  min-width: 140px;
  padding: var(--space-1);
  border: none;
  outline: none;
  background: none;
}

.tag-input__hint {
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.tag-input__hint--error {
  color: var(--color-danger);
}
</style>