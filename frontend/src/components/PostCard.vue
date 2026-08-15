<script setup>
import { computed } from 'vue'
import { formatDate } from '../utils/format'
import LikeButton from './LikeButton.vue'

const props = defineProps({
  post: { type: Object, required: true },
})

const emit = defineEmits(['like-update'])

const postUrl = computed(
  () => `/@${props.post.author.username}/${props.post.slug}`
)

const authorUrl = computed(() => `/@${props.post.author.username}`)

const authorName = computed(
  () => props.post.author.display_name || props.post.author.username
)

const displayDate = computed(
  () => props.post.published_at || props.post.created_at
)

// 把讚的變化往上傳給列表頁，由它更新該筆資料
function onLikeUpdate(state) {
  emit('like-update', { id: props.post.id, ...state })
}
</script>

<template>
  <article class="card">
    <div class="card__meta">
      <RouterLink :to="authorUrl" class="card__author">{{ authorName }}</RouterLink>
      <span class="card__dot">·</span>
      <time class="card__date">{{ formatDate(displayDate) }}</time>
      <span v-if="post.status === 'draft'" class="card__badge">草稿</span>
    </div>

    <RouterLink :to="postUrl" class="card__link">
      <h2 class="card__title">{{ post.title }}</h2>
      <p v-if="post.excerpt" class="card__excerpt">{{ post.excerpt }}</p>
    </RouterLink>

    <div class="card__footer">
      <ul v-if="post.tags.length" class="card__tags">
        <li v-for="tag in post.tags" :key="tag">
          <RouterLink :to="{ path: '/', query: { tag } }" class="card__tag">
            {{ tag }}
          </RouterLink>
        </li>
      </ul>

      <div class="card__stats">
        <!-- 草稿不顯示按讚（後端一律 404，決策 #43） -->
        <LikeButton
          v-if="post.status === 'published'"
          :post-id="post.id"
          :count="post.like_count"
          :liked="post.liked_by_me"
          @update="onLikeUpdate"
        />
        <RouterLink v-if="post.comment_count > 0" :to="postUrl" class="card__comments">
          {{ post.comment_count }} 則留言
        </RouterLink>
      </div>
    </div>
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

.card__author:hover {
  color: var(--color-accent);
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
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-top: var(--space-3);
}

.card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
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

.card__stats {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-left: auto;
  flex-shrink: 0;
}

.card__comments {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.card__comments:hover {
  color: var(--color-accent);
}

@media (max-width: 640px) {
  .card__footer {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
  }

  .card__stats {
    margin-left: 0;
  }
}
</style>