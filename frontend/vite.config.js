import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      // 所有 /api 開頭的請求轉發到後端。
      // 瀏覽器只看到 localhost:5173 → 同源 → cookie 正常運作。
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false, // 保留原始 Origin header，讓後端的 CSRF 白名單檢查通過
      },
    },
  },
})