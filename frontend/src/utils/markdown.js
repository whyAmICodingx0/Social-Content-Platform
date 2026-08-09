import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({
  gfm: true,     // GitHub Flavored Markdown：支援表格、刪除線等
  breaks: true,  // 單一換行也視為 <br>（多數人寫作時的直覺預期）
})

/**
 * 【安全】對所有 <a> 加上 target="_blank" 與 rel="noopener noreferrer"。
 *
 * 為什麼需要 rel：只加 target="_blank" 時，被開啟的新分頁可以透過
 * window.opener 操控原分頁（例如把它導向釣魚頁面），這叫 reverse tabnabbing。
 * 現代瀏覽器多已預設防範，但明確加上才不依賴瀏覽器版本。
 */
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.hasAttribute('href')) {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

/**
 * 把 Markdown 轉成「可安全放進 v-html」的 HTML。
 *
 * ⚠️ 這是專案裡唯一允許產生 v-html 內容的地方。
 *    任何要塞進 v-html 的字串都必須經過這個函式，不可跳過。
 */
export function renderMarkdown(md) {
  if (!md) return ''
  const rawHtml = marked.parse(md)
  return DOMPurify.sanitize(rawHtml)
}