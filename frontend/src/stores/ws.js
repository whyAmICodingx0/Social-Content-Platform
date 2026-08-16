import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * WebSocket 連線管理。
 *
 * 設計要點：
 *  - 前端不做心跳。瀏覽器的 WebSocket API 沒有 ping 方法，
 *    ping/pong 由協定層自動處理（server 送 ping、瀏覽器自動回 pong）。
 *  - 斷線後指數退避重連，並加入 jitter 避免同時重連。
 *  - WS 不保證送達；漏收的訊息由重連後的 REST `?after=` 補齊（P3-3）。
 */
export const useWsStore = defineStore('ws', () => {
  // 'closed' | 'connecting' | 'open'
  const status = ref('closed')

  let socket = null
  let retries = 0
  let retryTimer = null

  /**
   * 世代編號：每次 connect() 或 disconnect() 都會遞增。
   *
   * ⚠️ 這不是可有可無的保險。沒有它會出現這個 bug：
   * 舊 socket 的 onclose 在新連線建立「之後」才觸發，於是把
   * socket 設成 null、status 設成 'closed' —— 但新連線其實是開的。
   * 後果是 store 失去對活著連線的參照（disconnect 關不掉它）、
   * connect() 的守衛失效（可能開出第二條連線）、燈號說謊。
   *
   * 所有 handler 都先比對捕獲的世代，不符就直接返回。
   * 有了它就不需要 intentionalClose 旗標 ——
   * disconnect() 遞增世代後，舊 socket 的 onclose 在世代檢查
   * 那一行就返回，根本走不到排程重連。
   */
  let generation = 0

  // 事件訂閱：type → Set<handler>
  const handlers = new Map()

  function on(type, fn) {
    if (!handlers.has(type)) handlers.set(type, new Set())
    handlers.get(type).add(fn)
    // 回傳取消訂閱的函式，供元件 onUnmounted 使用
    return () => handlers.get(type)?.delete(fn)
  }

  function emit(type, data) {
    handlers.get(type)?.forEach((fn) => {
      try {
        fn(data)
      } catch (err) {
        console.error(`ws handler error (${type}):`, err)
      }
    })
  }

  function wsUrl() {
    // 頁面是 https 時必須用 wss，否則瀏覽器會擋下混合內容
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${window.location.host}/api/v1/ws`
  }

  function connect() {
    if (
      socket &&
      (socket.readyState === WebSocket.OPEN ||
        socket.readyState === WebSocket.CONNECTING)
    ) {
      return
    }

    clearTimeout(retryTimer)

    generation += 1
    const myGen = generation

    status.value = 'connecting'
    const ws = new WebSocket(wsUrl())
    socket = ws

    ws.onopen = () => {
      if (myGen !== generation) return
      status.value = 'open'
      retries = 0
      emit('open')
    }

    ws.onmessage = (event) => {
      if (myGen !== generation) return
      let msg
      try {
        msg = JSON.parse(event.data)
      } catch {
        console.warn('ws: 收到無法解析的訊息', event.data)
        return
      }
      if (msg?.type) emit(msg.type, msg.data)
    }

    ws.onerror = () => {
      // onerror 之後一定會觸發 onclose，重連邏輯集中在那裡
    }

    ws.onclose = () => {
      // 過期的世代 —— 這條 socket 已經被取代或主動關閉，什麼都不做
      if (myGen !== generation) return
      status.value = 'closed'
      socket = null
      emit('close')
      scheduleReconnect(myGen)
    }
  }

  function scheduleReconnect(myGen) {
    clearTimeout(retryTimer)

    // 指數退避：1s → 2s → 4s → 8s → 16s，上限約 30 秒
    const base = Math.min(1000 * 2 ** retries, 30000)

    // ±20% jitter：伺服器重啟時所有連線同時斷開，
    // 沒有隨機化的話大家會在同一秒重連，撞上還在冷啟動的服務。
    // 因為 jitter 在 cap 之後套用，最高檔實際落在 24~36 秒。
    const jitter = base * (Math.random() * 0.4 - 0.2)

    retries += 1
    retryTimer = setTimeout(() => {
      // 期間若有人呼叫 connect() 或 disconnect()，世代已變 → 放棄這次重連
      if (myGen !== generation) return
      connect()
    }, base + jitter)
  }

  function disconnect() {
    // 遞增世代 → 現有 socket 的所有 handler 立即失效，
    // 其 onclose 不會排程重連
    generation += 1
    clearTimeout(retryTimer)
    retries = 0

    if (socket) {
      socket.close()
      socket = null
    }
    status.value = 'closed'
  }

  return { status, connect, disconnect, on }
})