<script setup>
defineProps({
  text: { type: String, default: '載入中' },
})
</script>

<template>
  <div class="loading" role="status" aria-live="polite">
    <span class="loading__spinner" aria-hidden="true"></span>
    <span class="loading__text">{{ text }}</span>
  </div>
</template>

<style scoped>
.loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-3);
  padding: var(--space-16) 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.loading__spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--color-border);
  border-top-color: var(--color-accent);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 尊重使用者的「減少動態效果」系統設定（無障礙考量） */
@media (prefers-reduced-motion: reduce) {
  .loading__spinner {
    animation-duration: 2.5s;
  }
}
</style>