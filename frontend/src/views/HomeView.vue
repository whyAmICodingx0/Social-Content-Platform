<script setup>
import { ref, onMounted } from 'vue'
import { tagsApi } from '../api'

const tags = ref([])
const loading = ref(true)
const error = ref(null)

onMounted(async () => {
  try {
    const res = await tagsApi.list()
    tags.value = res.data
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="container">
    <h1 class="page-title">連線測試</h1>
    <p class="page-desc">以下標籤由後端 API 提供，能看到內容代表前後端串接成功。</p>

    <p v-if="loading" class="state">載入中…</p>
    <p v-else-if="error" class="state state--error">{{ error }}</p>
    <p v-else-if="tags.length === 0" class="state">
      目前沒有任何標籤（發一篇帶標籤的文章就會出現）。
    </p>
    <ul v-else class="tag-list">
      <li v-for="tag in tags" :key="tag.id" class="tag">
        {{ tag.slug }}
        <span class="tag__count">{{ tag.post_count }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.page-title {
  font-family: var(--font-serif);
  font-size: var(--text-3xl);
  margin-bottom: var(--space-2);
}

.page-desc {
  color: var(--color-text-muted);
  margin-bottom: var(--space-8);
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
}

.tag {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: var(--color-surface);
  font-size: var(--text-sm);
}

.tag__count {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
</style>