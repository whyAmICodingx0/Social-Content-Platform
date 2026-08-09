<script setup>
defineProps({
  // 後端回傳的 pagination 物件：{ page, limit, total, total_pages, has_next }
  pagination: { type: Object, required: true },
})

// 換頁時保留現有的其他 query（例如 tag），只改 page
defineEmits(['change'])
</script>

<template>
  <nav v-if="pagination.total_pages > 1" class="pager">
    <button
      class="btn btn--ghost"
      :disabled="pagination.page <= 1"
      @click="$emit('change', pagination.page - 1)"
    >
      上一頁
    </button>

    <span class="pager__info">
      第 {{ pagination.page }} / {{ pagination.total_pages }} 頁
      <span class="pager__total">（共 {{ pagination.total }} 篇）</span>
    </span>

    <button
      class="btn btn--ghost"
      :disabled="!pagination.has_next"
      @click="$emit('change', pagination.page + 1)"
    >
      下一頁
    </button>
  </nav>
</template>

<style scoped>
.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-4);
  padding-top: var(--space-8);
}

.pager__info {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.pager__total {
  opacity: 0.7;
}

.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>