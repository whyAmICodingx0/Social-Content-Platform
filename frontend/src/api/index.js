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

  like: (id) => api.put(`/posts/${id}/like`),
  unlike: (id) => api.del(`/posts/${id}/like`),
  feed: (params) => api.get('/feed' + toQuery(params)),
}

export const usersApi = {
  get: (username) => api.get(`/users/${encodeURIComponent(username)}`),

  // P2-3
  follow: (username) => api.put(`/users/${encodeURIComponent(username)}/follow`),
  unfollow: (username) => api.del(`/users/${encodeURIComponent(username)}/follow`),
  followers: (username, params) =>
    api.get(`/users/${encodeURIComponent(username)}/followers` + toQuery(params)),
  following: (username, params) =>
    api.get(`/users/${encodeURIComponent(username)}/following` + toQuery(params)),
}

export const tagsApi = {
  list: (params) => api.get('/tags' + toQuery(params)),
}

export const commentsApi = {
  list: (postId, params) =>
    api.get(`/posts/${postId}/comments` + toQuery(params)),
  create: (postId, content) =>
    api.post(`/posts/${postId}/comments`, { content }),
  update: (commentId, content) =>
    api.patch(`/comments/${commentId}`, { content }),
  remove: (commentId) => api.del(`/comments/${commentId}`),
}

export const conversationsApi = {
  // find-or-create，冪等 —— 開啟對話頁時呼叫
  findOrCreate: (username) => api.post('/conversations', { username }),

  list: (params) => api.get('/conversations' + toQuery(params)),

  // cursor 分頁：before / after 互斥（P3-3）
  messages: (conversationId, params) =>
    api.get(`/conversations/${conversationId}/messages` + toQuery(params)),

  sendMessage: (conversationId, clientMessageId, content) =>
    api.post(`/conversations/${conversationId}/messages`, {
      client_message_id: clientMessageId,
      content,
    }),

  unreadCount: () => api.get('/conversations/unread-count'),

  markRead: (conversationId, lastReadMessageId) =>
    api.post(`/conversations/${conversationId}/read`, {
      last_read_message_id: lastReadMessageId,
    }),
}

export const notificationsApi = {
  list: (params) => api.get('/notifications' + toQuery(params)),
  unreadCount: () => api.get('/notifications/unread-count'),
  markRead: (ids) => api.post('/notifications/read', { ids }),

  // ⚠️ 刻意不傳 body：client.js 的 request() 只在 body !== undefined 時
  // 才設 Content-Type 與 body —— 這正是後端 read-all 要求的形狀
  // （它不呼叫 BindStrict，空 body 才不會被判成 400）。
  markAllRead: () => api.post('/notifications/read-all'),
}

export const searchApi = {
  posts: (params) => api.get('/search/posts' + toQuery(params)),
  users: (params) => api.get('/search/users' + toQuery(params)),
}

function toQuery(params) {
  if (!params) return ''
  const usable = Object.entries(params).filter(
    ([, v]) => v !== undefined && v !== null && v !== ''
  )
  if (usable.length === 0) return ''
  return '?' + new URLSearchParams(usable).toString()
}