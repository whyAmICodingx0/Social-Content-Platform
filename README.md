# Social Content Platform

一個結合部落格發文與社交互動的內容平台後端，以 Go 實作。目前完成 Phase 1（認證、個人檔案、文章與標籤），前端與社交功能開發中。

> 個人專案，用於實踐全端開發流程：從資料庫 schema 設計、API 合約制定，到分層實作與逐步驗收。

---

## 目前狀態

| 項目 | 狀態 |
|---|---|
| Phase 1 後端 | ✅ 完成（15 / 15 端點） |
| Phase 1 前端 | ✅ 完成 |
| Phase 2（點讚、留言、關注、Feed） | 📋 規劃中 |
| Phase 3（WebSocket 聊天） | 📋 規劃中 |
| Phase 4（通知、搜尋） | 📋 規劃中 |

`db/schema.sql` 自定案起**零變更**——所有後續功能都在初始 schema 上完成。

---

## 畫面

| 首頁 | 文章頁 |
|---|---|
| ![首頁](docs/screenshots/home.png) | ![文章頁](docs/screenshots/post.png) |

| 編輯器 | 個人頁 |
|---|---|
| ![編輯器](docs/screenshots/editor.png) | ![個人頁](docs/screenshots/profile.png) |

---

## 技術棧

**後端**
- Go 1.22+ / Gin
- PostgreSQL 17（pgx/v5 + pgxpool）
- Redis 7（go-redis/v9）
- Google OAuth 2.0（golang.org/x/oauth2）

**前端**（開發中）
- Vue 3（Composition API）、Pinia、Vue Router、Vite

**開發環境**
- Docker Compose（PostgreSQL + Redis）

---

## 快速開始

### 需求

- Go 1.22 以上
- Docker Desktop
- Google Cloud Console 的 OAuth 2.0 憑證

### 1. 啟動資料庫服務

```bash
docker compose up -d
```

啟動 PostgreSQL（`localhost:5432`）與 Redis（`localhost:6379`）。

### 2. 建立資料表

用任意資料庫工具（DBeaver / psql）連上 `social_dev` 資料庫，執行 `db/schema.sql`。

```bash
# 或用指令
docker compose exec -T db psql -U app -d social_dev < db/schema.sql
```

### 3. 設定環境變數

複製 `.env.example` 為 `.env`，填入 Google OAuth 憑證：

```
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback
```

Google Cloud Console 設定重點：
- OAuth consent screen 選 **External**，並把測試帳號加入 **Test users**
- Credentials 建立 **Web application** 類型的 OAuth client ID
- Authorized redirect URI 必須與 `GOOGLE_REDIRECT_URL` 完全一致

### 4. 啟動後端

```bash
cd backend
go run ./cmd/server
```

驗證：

```bash
curl http://localhost:8080/healthz
# {"status":"ok","db":"connected","redis":"connected"}
```

登入：瀏覽器開啟 `http://localhost:8080/api/v1/auth/google/login`

---

### 5. 啟動前端

```bash
cd frontend
npm install
npm run dev
```

開啟 `http://localhost:5173`。

前端透過 Vite proxy 將 `/api` 轉發到後端的 `:8080`，因此瀏覽器視為同源，
HttpOnly cookie 可正常運作，不需要設定 CORS。

---

## 專案結構

```
backend/
├── cmd/server/          # 進入點：組裝所有元件並啟動
└── internal/
    ├── api/             # 統一回應格式、錯誤碼、嚴格 JSON 綁定、分頁工具
    ├── config/          # 環境變數載入、保留字清單、正式環境護欄
    ├── cookies/         # 三種 cookie 的統一屬性管理
    ├── googleoauth/     # Google OAuth 流程封裝
    ├── handler/         # HTTP 層：解析請求、呼叫 service、組成回應
    ├── middleware/      # CSRF、認證（required / optional 兩種模式）
    ├── repository/      # 資料存取層：SQL 與交易
    ├── service/         # 業務規則：驗證、權限、正規化
    └── store/           # Redis：session / OAuth state / pending signup
db/
└── schema.sql           # 資料庫 schema（5 張表）
docs/
└── api-spec.md          # API 合約（15 個端點的完整規格）
```

```
frontend/
├── src/
│   ├── api/             # API client 與各端點呼叫函式
│   ├── assets/styles/   # 設計 token、全域樣式、文章排版
│   ├── components/      # 可重用元件
│   ├── router/          # 路由與守衛
│   ├── stores/          # Pinia（登入狀態）
│   ├── utils/           # Markdown 渲染、日期格式化
│   └── views/           # 頁面
└── vite.config.js       # 含 /api proxy 設定
```

### 分層原則

```
handler  → 只處理 HTTP：解析、呼叫、回應
service  → 只處理業務規則：驗證、權限判斷、正規化
repository → 只處理資料庫：SQL、交易、錯誤轉譯
```

一個具體效益：`WHERE deleted_at IS NULL` 只寫在 repository 層，上層永遠拿不到已刪除的資料，不存在「某支查詢忘記加條件」的可能。

---

## 資料庫設計

五張表：

| 表 | 用途 |
|---|---|
| `users` | 使用者身分與個人檔案 |
| `oauth_accounts` | 第三方登入綁定（目前 Google，設計上支援多 provider） |
| `posts` | 文章（Markdown 原文） |
| `tags` | 標籤主檔 |
| `post_tags` | 文章與標籤的多對多關聯 |

主要設計選擇：

- **UUID v4 主鍵**：ID 會出現在 API 回應中，避免整數自增造成的可枚舉性
- **OAuth 拆表**：`users` 存「這個人是誰」，`oauth_accounts` 存「他怎麼證明自己」。未來新增 GitHub 登入不需改動 `users`
- **不儲存 Google token**：僅在登入當下驗證身分，取得 profile 後即丟棄，不持有高敏感憑證
- **軟刪除限定 `users` 與 `posts`**：關聯表直接實刪
- **Partial unique index**：所有唯一性索引都帶 `WHERE deleted_at IS NULL`，讓已刪除資料不佔用 username / slug
- **無 trigger**：`updated_at` 由應用層在每次 UPDATE 明確帶入，行為可預測、易追蹤

---

## 認證設計

Google OAuth 2.0 + Redis server-side session + HttpOnly Cookie（不使用 JWT）。

### 為什麼是 session 而非 JWT

Session 可即時撤銷（刪除即失效），JWT 簽發後在過期前無法收回，需額外的黑名單機制才能登出——而黑名單本身又是狀態，複雜度反而更高。本專案單一後端服務，session 的查詢成本可忽略。

### 註冊流程：pending signup

`users.username` 為 `NOT NULL`，但 Google 不會提供 username。常見做法是先建立帳號、填入暫時值，再要求使用者改名——這會在資料庫留下「半成品使用者」。

本專案改為：

1. Google 驗證完成後，**不建立任何資料列**，將 profile 暫存 Redis（30 分鐘 TTL）
2. 使用者選定 username 後，才在**單一交易**中建立 `users` + `oauth_accounts`

效果：中途放棄註冊時 Redis 自動過期，資料庫保持乾淨；`username NOT NULL` 的約束從未被放寬。

### 並行安全

判斷「此 Google 身分是否已註冊」一律以 `(provider, provider_user_id)` 為錨點，而非依賴哪個 unique constraint 被違反——因為 `users` 先於 `oauth_accounts` 插入，重複提交時會先撞到 email 唯一索引，永遠走不到 OAuth 那條分支。

交易中發生任何 unique violation 時：rollback → 在交易外重新查詢 OAuth 綁定 → 依結果收斂（綁定已存在則視同登入回 200，否則依原 constraint 回對應的 409）。

### 安全措施

- **OAuth state 綁定瀏覽器**：state 同時存入 Redis 與 HttpOnly cookie，callback 需三者相符（防 login CSRF）
- **State 一次性消費**：使用 Redis `GETDEL` 原子操作，避免 check-then-act 的競態
- **Session fixation 防護**：session ID 一律由伺服器新產生，不接受客戶端提供的值
- **CSRF 多層防護**：SameSite=Lax + 寫入端點強制 `application/json`（415）+ Origin 白名單（403）
- **降級策略**：Redis 不可用時，需認證的端點 fail closed（503），選擇性認證的端點降級為匿名——降級只會縮小權限，不會擴大

---

## 前端設計

### 登入狀態的判斷

`sid` cookie 是 HttpOnly，JavaScript 讀不到——這是後端刻意的設計。
因此前端判斷登入狀態的唯一方式是呼叫 `GET /me`：200 代表已登入、401 代表未登入。

App 啟動時執行一次，並以共用的 Promise 確保路由守衛能等到結果，
避免頁面載入瞬間把已登入的使用者誤判為訪客。

### Markdown 渲染與 XSS 防護

文章以 Markdown 原文儲存，渲染在前端進行：`marked` 轉為 HTML 後，
**必須經過 `DOMPurify` 消毒**才放入 `v-html`。

Markdown 規格允許嵌入 HTML，未消毒等同開放 XSS。雖然 cookie 是 HttpOnly、
無法被竊取，但注入的腳本仍會在使用者的登入身分下執行 API 請求。

### 篩選狀態放在網址

標籤篩選與分頁狀態存在 query string（`/?tag=go&page=2`）而非元件狀態，
讓網址可分享、瀏覽器返回鍵可用、重新整理後不遺失。

### 樣式架構

不使用 UI 框架。設計 token（顏色、字級、間距）集中於 `tokens.css`，
元件樣式寫在各自的 `<style scoped>`，template 只放語意化 class 名稱。
這讓「調整外觀」與「調整邏輯」能乾淨切開，也讓整體換風格只需修改單一檔案。

---

## API

Base URL：`/api/v1`

回應格式統一為 `{ "data": ... }`，錯誤為 `{ "error": { "code", "message", "details?" } }`。前端以 `code` 判斷分支。

| Method | Path | 認證 | 說明 |
|---|---|---|---|
| GET | `/auth/google/login` | — | 導向 Google 授權 |
| GET | `/auth/google/callback` | — | 授權回跳 |
| POST | `/auth/signup` | pending | 選定 username 完成註冊 |
| POST | `/auth/logout` | — | 登出（冪等） |
| GET | `/me` | ✓ | 取得自己的完整資料 |
| PATCH | `/me` | ✓ | 更新個人檔案 |
| GET | `/users/{username}` | — | 公開個人頁 |
| GET | `/users/{username}/posts` | — | 某使用者的已發布文章 |
| POST | `/posts` | ✓ | 建立文章 |
| GET | `/posts` | — | 全站已發布文章 |
| GET | `/me/posts` | ✓ | 自己的文章（含草稿） |
| GET | `/users/{username}/posts/{slug}` | 選擇性 | 讀取單篇 |
| PATCH | `/posts/{id}` | ✓ 作者 | 更新文章 |
| DELETE | `/posts/{id}` | ✓ 作者 | 軟刪除 |
| GET | `/tags` | — | 標籤列表 |

完整規格見 [`docs/api-spec.md`](docs/api-spec.md)。

---

## 幾個實作細節

### Slug 唯一性以資料庫為權威

文章 slug 在同一作者下唯一。產生時**不採「先查詢再寫入」**——那存在 TOCTOU 競態：兩個併發請求可能同時通過檢查，其中一個必然在寫入時失敗。

實際做法是直接寫入，捕獲 unique violation（SQLSTATE 23505）後換一個候選 slug，**重試整個交易**。因為 PostgreSQL 在交易內任何語句失敗後即進入 aborted 狀態，無法在原交易中續行。

### 404 與 403 的分野

- 讀取他人的**草稿** → `404`：回 403 等於承認「這篇存在但你不能看」，洩漏了草稿的存在
- 修改他人的**已發布文章** → `403`：文章本就公開，明確拒絕對正當使用者體驗更好

判準是「這個資源的存在本身是否為秘密」。

### 列表查詢避免 N+1

列出文章時，標籤以單一查詢 `WHERE post_id = ANY($1)` 一次取回後在應用層分組，而非每篇文章各查一次。查詢次數固定為常數，不隨文章數增長。

### 排序的確定性

所有列表排序都帶第二排序鍵 `id`。SQL 對排序鍵相同的資料列不保證固定順序，缺少 tie-break 會導致分頁時出現重複或遺漏。

公開列表依 `published_at` 排序；「我的文章」因包含草稿（`published_at` 為 NULL）改依 `created_at`。

---

## 已知限制

以下項目為有意識的取捨，非疏漏：

| 項目 | 說明 |
|---|---|
| Rate limiting | 尚未實作，對外部署前需補上 |
| 多裝置 session 管理 | 無法「登出所有裝置」，需要 session 反向索引 |
| CSRF token | 目前依賴 SameSite；跨網域部署時需升級 |
| Cursor 分頁 | 目前為 offset 分頁，資料量大時需更換 |
| 中文 slug 與標籤 | 中文標題產生隨機 slug、中文標籤會被忽略，需拼音轉換或改用 Unicode 鍵 |
| Markdown 摘要 | 以字串處理去除標記，非完整 parser |
| 前端測試 | 尚未撰寫；目前以手動驗收為主 |
| 圖片上傳 | 頭像與文章圖片皆為外部網址，無上傳功能 |

---

## Roadmap

- [x] Phase 1 後端：認證、個人檔案、文章 CRUD、標籤
- [ ] Phase 1 前端：Vue 3 介面
- [ ] Phase 2：點讚、留言、關注、Feed
- [ ] Phase 3：WebSocket 即時聊天
- [ ] Phase 4：通知系統、搜尋

---

## License

MIT