const SITE_NAME = 'Inkwell'

/**
 * 設定分頁標題。
 * 傳入空值則只顯示站名（首頁用）。
 */
export function setTitle(pageTitle) {
  document.title = pageTitle ? `${pageTitle} · ${SITE_NAME}` : SITE_NAME
}