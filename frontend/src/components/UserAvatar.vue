<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  src: { type: String, default: null },
  name: { type: String, default: '' },
  size: { type: Number, default: 40 },
})

const failed = ref(false)
watch(() => props.src, () => { failed.value = false })

const showImage = computed(() => props.src && !failed.value)

const initial = computed(() => {
  const n = (props.name || '?').trim()
  return n ? n.charAt(0).toUpperCase() : '?'
})

const style = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  fontSize: `${Math.round(props.size * 0.42)}px`,
}))
</script>

<template>
  <img
    v-if="showImage"
    :src="src"
    :alt="name"
    class="avatar"
    :style="style"
    referrerpolicy="no-referrer"
    @error="failed = true"
  />
  <span v-else class="avatar avatar--fallback" :style="style">{{ initial }}</span>
</template>

<style scoped>
.avatar {
  flex-shrink: 0;
  border-radius: 50%;
  object-fit: cover;
  background: var(--color-border);
}

.avatar--fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-muted);
  font-weight: 600;
  font-family: var(--font-ui);
  line-height: 1;
}
</style>