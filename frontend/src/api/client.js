const BASE = '/api/v1'

/**
 * ApiError：把後端的錯誤合約包成 JS 例外。
 * 後端回應格式：{ error: { code, message, details? } }
 * 前端一律用 err.code 判斷分支（不要用 message，那是給人看的）。
 */
export class ApiError extends Error {
  constructor(status, code, message, details) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }
}

async function request(method, path, body) {
  const options = {
    method,
    // 同源請求預設就會帶 cookie；明確寫出來讓意圖清楚
    credentials: 'same-origin',
    headers: {},
  }

  // 只在真的有 body 時設定 Content-Type：
  // 後端 CSRF middleware 要求有 body 的寫入請求必須是 application/json，
  // 但無 body 的請求（如 logout）不該帶這個 header。
  if (body !== undefined) {
    options.headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(body)
  }

  let res
  try {
    res = await fetch(BASE + path, options)
  } catch {
    throw new ApiError(0, 'NETWORK_ERROR', '無法連線到伺服器')
  }

  // 204 No Content：沒有 body 可以解析
  if (res.status === 204) return null

  let payload = null
  try {
    payload = await res.json()
  } catch {
    // 某些錯誤回應可能沒有 body，忽略解析失敗
  }

  if (!res.ok) {
    const e = payload?.error ?? {}
    throw new ApiError(
      res.status,
      e.code ?? 'UNKNOWN_ERROR',
      e.message ?? '發生未預期的錯誤',
      e.details
    )
  }

  return payload
}

export const api = {
  get: (path) => request('GET', path),
  post: (path, body) => request('POST', path, body),
  patch: (path, body) => request('PATCH', path, body),
  del: (path) => request('DELETE', path),
}