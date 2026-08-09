<script setup>
import { ref, onMounted } from 'vue'
import { tagsApi } from '../api'

defineProps({
  // 目前選中的標籤 slug，空字串代表「全部」
  active: { type: String, default: '' },
})

const tags = ref([])
const loading = ref(true)

onMounted(async () => {
  try {
    // 只取前 20 個熱門標籤，避免標籤太多塞爆版面
    const res = await tagsApi.list({ limit: 20 })
    // post_count 為 0 的標籤不顯示（點了也沒東西看）
    tags.value = res.data.filter((t) => t.post_count > 0)
  } catch {
    // 標籤列表失敗不該讓整頁壞掉，靜默略過即可
    tags.value = []
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <nav v-if="!loading && tags.length" class="tag-filter">
    <RouterLink
      :to="{ path: '/' }"
      class="tag-filter__item"
      :class="{ 'tag-filter__item--active': active === '' }"
    >
      全部
    </RouterLink>
    <RouterLink
      v-for="tag in tags"
      :key="tag.id"
      :to="{ path: '/', query: { tag: tag.slug } }"
      class="tag-filter__item"
      :class="{ 'tag-filter__item--active': active === tag.slug }"
    >
      {{ tag.slug }}
      <span class="tag-filter__count">{{ tag.post_count }}</span>
    </RouterLink>
  </nav>
</template>

<style scoped>
.tag-filter {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding-bottom: var(--space-6);
  border-bottom: 1px solid var(--color-border);
}

.tag-filter__item {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  font-size: var(--text-sm);
  background: var(--color-surface);
  transition: all 0.15s ease;
}

.tag-filter__item:hover {
  border-color: var(--color-accent);
  color: var(--color-accent);
}

.tag-filter__item--active {
  background: var(--color-accent);
  border-color: var(--color-accent);
  color: #fff;
}

.tag-filter__count {
  font-size: 0.75rem;
  opacity: 0.7;
}
</style>