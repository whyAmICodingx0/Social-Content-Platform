<template>
  <div class="container container--narrow notfound">
    <h1 class="notfound__code">404</h1>
    <p class="notfound__desc">找不到這個頁面。</p>
    <RouterLink to="/" class="btn btn--ghost">回首頁</RouterLink>
  </div>
</template>

<style scoped>
.notfound {
  text-align: center;
  padding-top: var(--space-16);
}
.notfound__code {
  font-family: var(--font-serif);
  font-size: var(--text-3xl);
  margin-bottom: var(--space-2);
}
.notfound__desc {
  color: var(--color-text-muted);
  margin-bottom: var(--space-8);
}
</style>