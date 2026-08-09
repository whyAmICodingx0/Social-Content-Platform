import { api } from './client'

export const authApi = {
  me: () => api.get('/me'),
  updateMe: (data) => api.patch('/me', data),
  signup: (data) => api.post('/auth/signup', data),
  logout: () => api.post('/auth/logout'),
  // 登入是頁面跳轉，不是 fetch —— 讓瀏覽器整頁導向後端
  loginUrl: '/api/v1/auth/google/login',
}

export const postsApi = {
  list: (params) => api.get('/posts' + toQuery(params)),
  listMine: (params) => api.get('/me/posts' + toQuery(params)),
  listByUser: (username, params) =>
    api.get(`/users/${encodeURIComponent(username)}/posts` + toQuery(params)),
  get: (username, slug) =>
    api.get(`/users/${encodeURIComponent(username)}/posts/${encodeURIComponent(slug)}`),
  create: (data) => api.post('/posts', data),
  update: (id, data) => api.patch(`/posts/${id}`, data),
  remove: (id) => api.del(`/posts/${id}`),
}

export const usersApi = {
  get: (username) => api.get(`/users/${encodeURIComponent(username)}`),
}

export const tagsApi = {
  list: (params) => api.get('/tags' + toQuery(params)),
}

function toQuery(params) {
  if (!params) return ''
  const usable = Object.entries(params).filter(
    ([, v]) => v !== undefined && v !== null && v !== ''
  )
  if (usable.length === 0) return ''
  return '?' + new URLSearchParams(usable).toString()
}