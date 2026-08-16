/**
 * 產生 client_message_id。
 *
 * crypto.randomUUID 需要 secure context（https 或 localhost）。
 * 從區網 IP 用 http 開發時不可用，故留一個 fallback。
 */
export function newClientMessageId() {
  if (globalThis.crypto?.randomUUID) {
    return crypto.randomUUID()
  }

  const b = new Uint8Array(16)
  crypto.getRandomValues(b)
  b[6] = (b[6] & 0x0f) | 0x40 // version 4
  b[8] = (b[8] & 0x3f) | 0x80 // variant
  const h = [...b].map((x) => x.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}