import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: false },
    },
  },
  // npm run preview 用來檢視打包後的成品，同樣需要 proxy 才能連後端
  preview: {
    port: 4173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: false },
    },
  },
})