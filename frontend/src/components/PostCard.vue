<script setup>
import { computed } from 'vue'
import { formatDate } from '../utils/format'

const props = defineProps({
  post: { type: Object, required: true },
})

// 文章網址：/@username/slug（決策 #5）
const postUrl = computed(
  () => `/@${props.post.author.username}/${props.post.slug}`
)

const authorName = computed(
  () => props.post.author.display_name || props.post.author.username
)

// 草稿沒有 published_at，退回 created_at
const displayDate = computed(
  () => props.post.published_at || props.post.created_at
)
</script>

<template>
  <article class="card">
    <div class="card__meta">
      <span class="card__author">{{ authorName }}</span>
      <span class="card__dot">·</span>
      <time class="card__date">{{ formatDate(displayDate) }}</time>
      <span v-if="post.status === 'draft'" class="card__badge">草稿</span>
    </div>

    <RouterLink :to="postUrl" class="card__link">
      <h2 class="card__title">{{ post.title }}</h2>
      <p v-if="post.excerpt" class="card__excerpt">{{ post.excerpt }}</p>
    </RouterLink>

    <ul v-if="post.tags.length" class="card__tags">
      <li v-for="tag in post.tags" :key="tag">
        <RouterLink :to="{ path: '/', query: { tag } }" class="card__tag">
          {{ tag }}
        </RouterLink>
      </li>
    </ul>
  </article>
</template>

<style scoped>
.card {
  padding: var(--space-6) 0;
  border-bottom: 1px solid var(--color-border);
}

.card__meta {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.card__author {
  font-weight: 550;
  color: var(--color-text);
}

.card__badge {
  padding: 0 var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: 3px;
  font-size: 0.75rem;
}

.card__link {
  display: block;
}

.card__title {
  font-family: var(--font-serif);
  font-size: var(--text-xl);
  margin-bottom: var(--space-2);
  transition: color 0.15s ease;
}

.card__link:hover .card__title {
  color: var(--color-accent);
}

.card__excerpt {
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
  /* 最多顯示兩行，超過以 … 收尾 */
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-top: var(--space-3);
}

.card__tag {
  display: inline-block;
  padding: var(--space-1) var(--space-2);
  border-radius: 3px;
  background: var(--color-border);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.card__tag:hover {
  color: var(--color-accent);
}
</style>