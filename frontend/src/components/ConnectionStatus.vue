<script setup>
import { computed } from 'vue'
import { useWsStore } from '../stores/ws'

const ws = useWsStore()

const text = computed(() => {
  switch (ws.status) {
    case 'open': return '即時連線中'
    case 'connecting': return '連線中…'
    default: return '連線中斷，重試中'
  }
})
</script>

<template>
  <span class="conn" :class="`conn--${ws.status}`" :title="text">
    <span class="conn__dot" aria-hidden="true"></span>
    <span class="conn__text">{{ text }}</span>
  </span>
</template>

<style scoped>
.conn {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.conn__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-text-muted);
  flex-shrink: 0;
}

.conn--open .conn__dot { background: #3f8f5f; }
.conn--connecting .conn__dot { background: #c8a02f; }
.conn--closed .conn__dot { background: var(--color-danger); }

@media (prefers-reduced-motion: no-preference) {
  .conn--connecting .conn__dot {
    animation: pulse 1.2s ease-in-out infinite;
  }
}

@keyframes pulse {
  50% { opacity: 0.3; }
}

/* 手機版只留燈號，省空間 */
@media (max-width: 640px) {
  .conn__text { display: none; }
}
</style>