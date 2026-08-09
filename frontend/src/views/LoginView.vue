<script setup>
import { authApi } from '../api'

function login() {
  window.location.href = authApi.loginUrl
}
</script>

<template>
  <div class="container container--narrow login">
    <h1 class="login__title">登入</h1>
    <p class="login__desc">使用 Google 帳號登入即可開始寫作。</p>
    <button class="btn btn--primary" @click="login">使用 Google 登入</button>
  </div>
</template>

<style scoped>
.login {
  text-align: center;
  padding-top: var(--space-16);
}
.login__title {
  font-family: var(--font-serif);
  font-size: var(--text-3xl);
  margin-bottom: var(--space-3);
}
.login__desc {
  color: var(--color-text-muted);
  margin-bottom: var(--space-8);
}
</style>