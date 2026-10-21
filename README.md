# Inkwell — Social Content Platform

一個結合長文寫作與社交互動的內容平台：Markdown 發文、圖片與影片上傳、按讚留言、追蹤與動態牆、即時私訊、通知系統與全文搜尋。以 Go 與 Vue 3 從零實作。

**線上 Demo：** https://inkwell-hq36.onrender.com

> 部署於免費方案，閒置後首次載入需等待約 30–60 秒（服務冷啟動）。

---

## 目前狀態

| 階段 | 內容 | 狀態 |
|---|---|---|
| Phase 1 | Google 登入、個人檔案、文章 CRUD、Markdown、標籤 | ✅ |
| Phase 2 | 按讚、留言、追蹤、動態牆 | ✅ |
| Phase 3 | WebSocket 即時私訊、聊天歷史、未讀數 | ✅ |
| Phase 4 | 通知系統、全文搜尋 | ✅ |
| Phase 5 | 媒體上傳：頭像、文章圖片、影片附件 | ✅ |

**40 個 REST 端點 + 1 個 WebSocket 端點、14 張資料表、13 個前端頁面。**

Phase 1 的資料庫設計沒有被推翻：後續四個階段只做加法——新增表、索引與 `pg_trgm` 擴充，以及 Phase 5 為 `users` 新增一個 nullable 欄位；沒有修改或刪除任何既有欄位。

---

## 畫面

| 首頁 | 文章頁 |
|---|---|
| ![首頁](docs/screenshots/home.png) | ![文章頁](docs/screenshots/post.png) |

| 編輯器 | 個人頁 |
|---|---|
| ![編輯器](docs/screenshots/editor.png) | ![個人頁](docs/screenshots/profile.png) |

| 即時私訊 | 通知 |
|---|---|
| ![私訊](docs/screenshots/messages.png) | ![通知](docs/screenshots/notifications.png) |

> 建議補拍：搜尋結果頁、動態牆、含圖片與影片附件的文章與編輯器。

---

## 功能

**內容**
- Markdown 撰寫與即時預覽
- 草稿 / 發布狀態切換
- 標籤（發文時隱式建立，最多 5 個）與標籤篩選
- 全文搜尋（文章與使用者）

**媒體**
- 文章插入圖片（上傳前自動縮小，插在游標位置）
- 影片附件（每篇 1 部，60 秒、30MB 以內）
- 上傳進度、失敗重試

**社交**
- 文章按讚
- 留言（單層，可編輯、可刪除；文章作者可移除他人留言）
- 追蹤使用者
- 動態牆：追蹤對象與自己的文章

**即時**
- 一對一私訊，WebSocket 即時送達
- 聊天歷史（cursor 分頁）、未讀數
- 斷線自動重連並補齊漏收的訊息
- 通知系統（按讚、留言、追蹤），即時紅點

**帳號**
- Google OAuth 登入
- 公開個人頁 `/@username`
- 個人檔案編輯、頭像上傳

---

## 技術棧

**後端**
- Go 1.26.4 / Gin
- PostgreSQL 17（pgx/v5 + pgxpool）
- Redis 7（go-redis/v9）
- gorilla/websocket
- Google OAuth 2.0（golang.org/x/oauth2）
- goose（migration）
- pg_trgm（搜尋）
- Cloudinary（媒體儲存、轉檔、CDN；以 `net/http` 自寫簽章與 Admin API 呼叫，未使用 SDK）

**前端**
- Vue 3（Composition API）、Pinia、Vue Router、Vite
- marked + DOMPurify（Markdown 渲染與消毒）
- 手寫 CSS，無 UI 框架

**部署**
- Render（Docker，Go binary 以 `//go:embed` 內嵌 Vue 建置產物）
- Neon（PostgreSQL）、Upstash（Redis）、Cloudinary（媒體）
- GitHub Actions（Upstash 保活，見「維運」）

---

## 快速開始

### 需求

- Go 1.26.4 以上（`backend/go.mod` 的 `go` 指令）、Node.js 20 以上、Docker Desktop
- Google Cloud Console 的 OAuth 2.0 憑證
- Cloudinary 帳號（選用；沒有的話，上傳相關端點回 503，其他功能照常）

### 1. 啟動資料庫服務

```bash
docker compose up -d
```

啟動 PostgreSQL（`localhost:5432`）與 Redis（`localhost:6379`）。

### 2. 建立資料表

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest

export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://app:devpassword@localhost:5432/social_dev?sslmode=disable"
goose -dir db/migrations up
```

（Windows PowerShell 用 `$env:GOOSE_DRIVER='postgres'` 的形式。）

### 3. 設定環境變數

複製 `backend/.env.example` 為 `backend/.env`，填入 Google OAuth 憑證：

```
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback
```

Google Cloud Console 設定重點：
- OAuth consent screen 選 **External**
- Credentials 建立 **Web application** 類型的 OAuth client ID
- Authorized redirect URI 必須與 `GOOGLE_REDIRECT_URL` 完全一致

要使用媒體上傳，再加上 Cloudinary 的三個值：

```
CLOUDINARY_CLOUD_NAME=your-cloud-name
CLOUDINARY_API_KEY=your-api-key
CLOUDINARY_API_SECRET=your-api-secret
```

Cloudinary 設定重點：
- 新帳號預設是 **dynamic folder mode**，本專案依賴這個模式（歸屬以 `asset_folder` 驗證）
- Settings → Upload → Upload presets 建立三個 **Signed** preset，**Overwrite 關閉、不設 folder**：

| Preset | 類型 | 允許格式 | 進站轉換 |
|---|---|---|---|
| `inkwell_avatar` | Image | `jpg,jpeg,png,webp` | `c_limit,w_1024` |
| `inkwell_post_image` | Image | `jpg,jpeg,png,webp` | `c_limit,w_2400` |
| `inkwell_post_video` | Video | `mp4,webm,mov` | 無 |

- `CLOUDINARY_API_SECRET` 只放在後端環境變數，**絕不可出現在前端或 git**

### 4. 啟動後端

```bash
cd backend
go run ./cmd/server
```

```bash
curl http://localhost:8080/healthz
# {"status":"ok","db":"connected","redis":"connected"}
```

### 5. 啟動前端

```bash
cd frontend
npm install
npm run dev
```

開啟 `http://localhost:5173`。

前端透過 Vite proxy 將 `/api` 轉發到後端的 `:8080`，因此瀏覽器視為同源，HttpOnly cookie 可正常運作，不需要設定 CORS。WebSocket 走同一個 proxy（`ws: true`）。

---

## 專案結構

```
backend/
├── cmd/server/          # 進入點：組裝所有元件並啟動
└── internal/
    ├── api/             # 統一回應格式、錯誤碼、嚴格 JSON 綁定、分頁工具
    ├── cloudinary/      # Cloudinary 簽章、Admin API 反查、刪除
    ├── config/          # 環境變數載入、保留字清單、正式環境護欄
    ├── cookies/         # 三種 cookie 的統一屬性管理
    ├── googleoauth/     # Google OAuth 流程封裝
    ├── handler/         # HTTP 層：解析請求、呼叫 service、組成回應
    ├── middleware/      # CSRF、認證（required / optional 兩種模式）
    ├── repository/      # 資料存取層：SQL 與交易
    ├── service/         # 業務規則：驗證、權限、正規化
    ├── store/           # Redis：session / OAuth state / pending signup / 上傳配額
    ├── web/             # 內嵌前端靜態檔與 SPA fallback
    └── ws/              # WebSocket：Hub、連線管理、事件推送
frontend/
└── src/
    ├── api/             # API client 與各端點呼叫函式
    ├── assets/styles/   # 設計 token、全域樣式、文章排版
    ├── components/      # 可重用元件（含頭像與媒體上傳元件）
    ├── composables/     # 可重用的狀態邏輯（上傳流程）
    ├── router/          # 路由與守衛
    ├── stores/          # Pinia（登入、WebSocket、未讀、通知）
    ├── utils/           # Markdown 渲染、日期格式化、UUID、圖片壓縮、影片檢查
    └── views/           # 頁面
db/
├── migrations/          # schema 變更歷史（唯一真相）
└── schema.sql           # 完整快照（衍生）
docs/
└── api-spec.md          # API 合約（40 個端點的完整規格）
.github/workflows/
└── upstash-keepalive.yml  # Upstash 保活（見「維運」）
```

### 分層原則

```
handler    → 只處理 HTTP：解析、呼叫、回應
service    → 只處理業務規則：驗證、權限判斷、正規化
repository → 只處理資料庫：SQL、交易、錯誤轉譯
```

一個具體效益：`WHERE deleted_at IS NULL` 只寫在 repository 層，上層永遠拿不到已刪除的資料，不存在「某支查詢忘記加條件」的可能。

---

## 資料庫設計

14 張表：

| 表 | 用途 | 刪除策略 |
|---|---|---|
| `users` | 使用者身分與個人檔案 | soft delete |
| `oauth_accounts` | 第三方登入綁定（設計上支援多 provider） | hard delete |
| `posts` | 文章（Markdown 原文） | soft delete |
| `tags` / `post_tags` | 標籤與多對多關聯 | hard delete |
| `post_likes` | 文章按讚 | hard delete |
| `comments` | 單層留言 | soft delete |
| `follows` | 使用者自關聯多對多 | hard delete |
| `conversations` | 一對一對話 | hard delete |
| `messages` | 訊息 | 不刪除 |
| `conversation_reads` | 每人在每個對話的已讀位置 | hard delete |
| `notifications` | 通知 | hard delete |
| `media` | 已通過驗證的上傳檔案（Cloudinary） | 不刪（孤兒以 SQL 列出、人工清理） |
| `post_media` | 文章擁有哪些媒體 | 隨編輯整組替換 |

主要設計選擇：

- **UUID v4 主鍵**：ID 會出現在 API 回應中，避免整數自增造成的可枚舉性
- **OAuth 拆表**：`users` 存「這個人是誰」，`oauth_accounts` 存「他怎麼證明自己」。未來新增 GitHub 登入不需改動 `users`
- **不儲存 Google token**：僅在登入當下驗證身分，取得 profile 後即丟棄
- **軟刪除只用於內容型資料**：關聯表（讚、標籤、追蹤）直接實刪
- **Partial unique index**：唯一性索引都帶 `WHERE deleted_at IS NULL`，讓已刪除資料不佔用 username / slug
- **無 trigger**：`updated_at` 由應用層明確帶入，行為可預測
- **不存計數欄位**：`like_count` / `comment_count` 即時聚合，避免兩份真相不一致
- **頭像的兩個欄位各司其職**：`avatar_url` 決定顯示哪張圖（可能來自 Google、外部網址或上傳），`avatar_media_id` 只記錄「這張是不是本站託管的」，供清理時判斷

### 幾個約束層的設計

**對話的唯一性由一個 CHECK 同時保證兩件事**：

```sql
CONSTRAINT conversations_pair_key UNIQUE (user_low_id, user_high_id),
CONSTRAINT conversations_ordered  CHECK  (user_low_id < user_high_id)
```

兩個 user id 排序後存入，`UNIQUE` 讓「A↔B」與「B↔A」必然是同一列；`CHECK` 的 `<`（而非 `<>`）同時保證順序正確與「不能自己跟自己聊天」。

**通知的去重靠 `NULLS NOT DISTINCT`**：

```sql
CREATE UNIQUE INDEX notifications_dedup_key
    ON notifications (recipient_id, actor_id, type, entity_id)
    NULLS NOT DISTINCT;
```

追蹤類型的 `entity_id` 為 NULL，而 PostgreSQL 預設把 NULL 視為互不相同——沒有 `NULLS NOT DISTINCT`（PG 15+），追蹤通知的去重會完全失效。

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

交易中發生任何 unique violation 時：rollback → 在交易外重新查詢 OAuth 綁定 → 依結果收斂。

### 安全措施

- **OAuth state 綁定瀏覽器**：state 同時存入 Redis 與 HttpOnly cookie，callback 需三者相符（防 login CSRF）
- **State 一次性消費**：使用 Redis `GETDEL` 原子操作，避免 check-then-act 的競態
- **Session fixation 防護**：session ID 一律由伺服器新產生
- **CSRF 多層防護**：SameSite=Lax + 寫入端點強制 `application/json`（415）+ Origin 白名單（403）
- **WebSocket Origin 檢查**：握手是 GET 且不受 SameSite 保護，必須另外驗證 Origin（防 Cross-Site WebSocket Hijacking）
- **降級策略**：Redis 不可用時，需認證的端點 fail closed（503），選擇性認證的端點降級為匿名——降級只會縮小權限，不會擴大

---

## 即時通訊設計

### 職責分工：HTTP 寫入，WebSocket 只推送

訊息寫入走 `POST /conversations/{id}/messages`，WebSocket 只負責把已寫入的訊息推給對方。

這讓驗證、權限、錯誤格式、交易處理全部沿用既有的 HTTP 流程，而不是在 WebSocket 訊息裡重新實作一遍。WebSocket 的職責縮小到「找到對方的連線並寫入」，是可以獨立驗收的一件事。

**PostgreSQL 是唯一權威，WebSocket 推送不保證送達。** 漏收的訊息由重連後的補齊機制解決。

### 連線管理

一條連線 = 兩個 goroutine（readPump 唯一讀取者、writePump 唯一寫入者）。

**gorilla/websocket 不允許併發寫入**——兩個 goroutine 同時寫會直接 panic。因此所有寫入（含 ping frame）都必須發生在 writePump，其他地方只能往有界的 send channel 投遞。

⚠️ 一個 Go channel 的規則值得記住：**有多個 sender 時，receiver 不可關閉 channel**。`send` 的寫入者是任何推送訊息的 goroutine，而關閉是從讀取端觸發的——若去 `close(send)`，往已關閉 channel 寫入會直接 panic 且無法用 `select` 或 `recover` 避開，而那個 panic 會發生在 HTTP handler 的 goroutine（那裡沒有 recover）。正確做法是另開一個 `done` channel 當結束訊號。

### 心跳

**瀏覽器的 WebSocket API 沒有暴露 ping 方法**——ping/pong 是協定層由瀏覽器自動處理的。因此心跳由 server 發起（每 54 秒送 ping frame），瀏覽器自動回 pong，server 的 pong handler 延長 read deadline。前端不需要任何心跳程式碼。

沒有這套機制，手機切背景、拔網路線都不會觸發 close event，死連線會永遠佔著記憶體與 goroutine。

### 連線期間的 session 重驗

每次收到 pong 時順便重驗一次 session。兩種失敗要區分：

| 情況 | 處理 |
|---|---|
| Redis 回報查無此 session | **關閉連線**（憑證確實已失效） |
| Redis 連不上或逾時 | **不關閉**（容忍暫時性故障） |

這看似與「Redis 掛掉時 required auth fail closed」衝突，但語意不同：那條是「不知道你是誰就不給進門」；這裡是「**已經在門內的人，不因為門房打瞌睡而被趕出去**」。

### 訊息的冪等與樂觀更新

前端產生 `client_message_id`（UUID），配合 `UNIQUE(sender_id, client_message_id)` 保證重試不會產生兩則訊息。同一個 id 也是樂觀更新的比對鍵：送出時先顯示 pending 訊息，收到 201 或 WebSocket 回音時據以替換。

---

## 搜尋設計

搜尋以 `ILIKE` 保證命中正確性，`pg_trgm` 提供 GIN 索引與 `word_similarity` 排序。

**中文的處理**：PostgreSQL 內建的 full-text search 對中文不斷詞（`to_tsvector` 會把整句當成一個詞），而中文斷詞擴充（zhparser / pg_jieba）需要編譯安裝，Neon 免費層不支援自訂擴充。因此改用 trigram + ILIKE，對中英文都可用。

**2 字元查詢無法使用索引**：GIN trigram 索引要能支援 `LIKE`，pattern 裡必須有至少 3 個連續字元可抽出 trigram。`%程式%` 的字面部分只有 2 個字，抽不出任何 trigram——這是原理限制，不是 planner 挑錯。

下限仍維持 2 字元，因為改成 3 會讓「程式」「台灣」「音樂」這類常見中文兩字詞完全搜不了。代價是這類查詢一律 Seq Scan。這是刻意接受的取捨。

**排序用 `word_similarity` 而非 `similarity`**：後者以兩邊 trigram 集合的聯集做正規化，200 字的摘要對上 2 字的查詢，分數會被稀釋到接近 0（實測 6 字的 haystack 就已降到 0.111）。`word_similarity` 取 haystack 中任意連續片段的最佳相似度，不被整體長度影響。

也不用 `strict_word_similarity`——它要求片段對齊詞邊界，而中文沒有詞邊界，會退化成 `similarity`。

**查詢字串會 escape `\ % _`**：不 escape 的話，使用者搜「%」會匹配全部文章。這不是安全漏洞（參數化查詢仍防 SQL injection），但是明顯的行為錯誤。

---

## 媒體上傳設計

頭像、文章圖片、影片附件都存在 Cloudinary。這是專案「能用 PostgreSQL 解決就不引入新服務」原則的第二個例外（第一個是 Redis），理由是 Render 免費方案只有 0.1 CPU、512MB 記憶體與暫時檔案系統——自己跑 ffmpeg 轉檔會 OOM，檔案也會在重啟時消失。

### 流程：檔案不經過 Go 伺服器

```
瀏覽器                         Go 後端                        Cloudinary
  │  1. 壓縮（圖片）/ 檢查（影片）    │                              │
  │──2. POST /media/signature ───→│ 產生 public_id、簽章、扣配額    │
  │←── fields ─────────────────────│                              │
  │──3. 直傳（FormData = fields + file）─────────────────────────→│
  │←── asset_id, public_id ────────────────────────────────────────│
  │──4. POST /media ───────────────→│──5. Admin API 反查 ─────────→│
  │                                 │←── 實際的大小、格式、長度 ─────│
  │                                 │ 驗證通過 → 建立 media          │
  │←── media（id、url）──────────────│ 驗證失敗 → 刪除該檔案          │
```

### 不信任前端回報的任何 metadata

前端只回報三個識別欄位（`purpose`、`asset_id`、`public_id`）。網址、尺寸、大小、影片長度全部由後端用 Cloudinary 的 Admin API 反查取得。

沒有這一步的話，任何人都能直接呼叫回報端點，把任意網址（包括別人的資源、甚至別的網站）塞進資料庫。

歸屬驗證用的是 Cloudinary 的 `asset_folder`：簽章時把 `inkwell/{user_id}` 簽進去，反查時要求完全相符。歸屬不符時**不刪檔**——否則任何人都能借本站的後端刪掉別人的檔案。

### 讓簽章只能用一次

Cloudinary 的簽章本身沒有 nonce，在一小時的有效期內可以上傳無限多個檔案——這讓「每日 50 次簽章」的配額形同虛設。

解法是由後端產生 `public_id`（128 bit 隨機值），並把 `overwrite=false` 一起簽進去。同一個簽章第二次上傳時，Cloudinary 不會建立新檔案，而是回傳既有檔案並帶 `existing: true`。於是：

- 配額真的在限制檔案數（每人每日 50 個）
- 網址不可猜測
- **前端可以安全地用同一個簽章重試**：上次其實傳成功、只是回應掉了的話，重試只會拿回同一個檔案

### 前端處理只是最佳化，不是限制

圖片上傳前用 Canvas 縮小（頭像最長邊 512、文章圖片 1600）並轉成 JPEG，能省下大部分的上傳時間與額度，也順帶移除手機照片的 EXIF（包括 GPS 位置）。

但真正的限制在後端反查——惡意客戶端可以完全繞過前端。兩條相反的失敗處理規則：

| 情況 | 前端行為 | 理由 |
|---|---|---|
| 圖片壓縮失敗 | **中止**，不送原檔 | 放行會改送大好幾倍的原檔，使用者卻以為送出的是壓縮版 |
| 影片讀不到長度 | **放行** | 影片本來就送原檔；後端仍會擋下超過 60 秒的影片。而讀不到很常見（iPhone 的 HEVC `.mov` 在 Windows 的 Chrome 上解不開） |

### 後端不解析 Markdown

文章圖片寫在 Markdown 裡（`![](url)`），但「這篇文章擁有哪些媒體」由發文時送出的 `media_ids` 決定，後端完全不看內文。

用正規表示式抽 `![]()` 會在程式碼區塊、行內程式碼、HTML `<img>` 上出錯，而**錯的方向是擋掉合法的發文**。取而代之的是編輯器在存檔時只送「網址還在內文裡」的圖片——前端用字串比對，不是解析。

### 文章頁不會偷偷下載影片

影片的 `<video>` 設了 `preload="none"` 與封面圖：沒按播放就不下載。流量是 Cloudinary 免費額度的主要成本（一部 30MB 的影片完整播放約 33 次就是 1 credit），而這種浪費在畫面上完全看不出來。

### Admin API 預設不回傳影片長度

Cloudinary 的文件與三個官方 SDK 都沒有明確說明。實測後發現影片的資源查詢預設不含 `duration`，要加 `?media_metadata=true` 才會出現。

因為長度驗證設計成 fail closed（拿不到長度就拒絕），這個缺漏表現為一個明確的錯誤訊息——如果設計成「拿不到就放行」，60 秒的限制會悄悄失效而且沒人發現。

---

## 前端設計

### 登入狀態的判斷

`sid` cookie 是 HttpOnly，JavaScript 讀不到——這是後端刻意的設計。因此前端判斷登入狀態的唯一方式是呼叫 `GET /me`：200 代表已登入、401 代表未登入。

App 啟動時執行一次，並以共用的 Promise 確保路由守衛能等到結果，避免頁面載入瞬間把已登入的使用者誤判為訪客。

### Markdown 渲染與 XSS 防護

文章以 Markdown 原文儲存，渲染在前端進行：`marked` 轉為 HTML 後，**必須經過 `DOMPurify` 消毒**才放入 `v-html`。

Markdown 規格允許嵌入 HTML，未消毒等同開放 XSS。雖然 cookie 是 HttpOnly、無法被竊取，但注入的腳本仍會在使用者的登入身分下執行 API 請求。

留言與訊息則採**純文字渲染**（保留換行但不解析 Markdown），少一個風險面。

### WebSocket 重連的世代編號

斷線後採指數退避重連（1s → 2s → 4s…，上限約 30 秒，帶 ±20% jitter 避免所有連線同時重試）。

每次 `connect()` 或 `disconnect()` 都會遞增一個世代編號，所有事件 handler 先比對捕獲的世代。沒有它會出現這個 bug：舊 socket 的 `onclose` 在新連線建立之後才觸發，於是把 socket 設成 null、狀態設成 closed——但新連線其實是開的。後果是 store 失去對活著連線的參照、狀態顯示錯誤、甚至開出第二條連線。

### 樂觀更新與回滾

按讚、追蹤、傳訊息採樂觀更新：點擊後立即改變畫面，不等 API 回應。請求失敗時**回滾至原值**——沒有回滾的樂觀更新會讓畫面顯示實際不存在的狀態。

成功後仍以伺服器回傳的計數為準，因為期間可能有其他使用者也按了讚。

### 篩選狀態放在網址

標籤篩選、分頁、搜尋關鍵字都存在 query string（`/search?q=go&type=posts&page=2`）而非元件狀態，讓網址可分享、瀏覽器返回鍵可用、重新整理後不遺失。

### 樣式架構

不使用 UI 框架。設計 token（顏色、字級、間距）集中於 `tokens.css`，元件樣式寫在各自的 `<style scoped>`，template 只放語意化 class 名稱。這讓「調整外觀」與「調整邏輯」能乾淨切開，也讓整體換風格只需修改單一檔案。

---

## 幾個實作細節

### Slug 唯一性以資料庫為權威

文章 slug 在同一作者下唯一。產生時**不採「先查詢再寫入」**——那存在 TOCTOU 競態：兩個併發請求可能同時通過檢查，其中一個必然在寫入時失敗。

實際做法是直接寫入，捕獲 unique violation（SQLSTATE 23505）後換一個候選 slug，**重試整個交易**——因為 PostgreSQL 在交易內任何語句失敗後即進入 aborted 狀態，無法在原交易中續行。

### 404 與 403 的分野

- 讀取他人的**草稿** → `404`：回 403 等於承認「這篇存在但你不能看」，洩漏了草稿的存在
- 修改他人的**已發布文章** → `403`：文章本就公開，明確拒絕對正當使用者體驗更好
- 存取不屬於自己的**對話** → `404`：同樣不洩漏存在性

判準是「這個資源的存在本身是否為秘密」。

### 計數聚合避免笛卡爾乘積

同時 JOIN `post_likes` 與 `comments` 會產生乘積（3 個讚 × 4 則留言 = 12 列，兩邊計數都變成 12）。正確做法是 `LEFT JOIN LATERAL` 讓兩邊各自聚合，互不放大。

`COUNT(DISTINCT)` 雖能算對結果，但那 12 列仍被實體化——只是把錯的算對。

### 列表查詢避免 N+1

列出文章時，標籤以單一查詢 `WHERE post_id = ANY($1)` 一次取回後在應用層分組，而非每篇文章各查一次。查詢次數固定為常數，不隨文章數增長。

### 排序的確定性

所有列表排序都帶第二排序鍵 `id`。SQL 對排序鍵相同的資料列不保證固定順序，缺少 tie-break 會導致分頁時出現重複或遺漏。

### 聊天歷史用 cursor 分頁

專案原則是 offset 分頁，**聊天訊息是唯一例外**——這不是效能問題，是正確性問題。使用者往上捲載入 `offset=20` 的那一刻，對方傳了 3 則新訊息進來，整個序列往後推 3 格，會重複拿到已顯示過的訊息或漏掉 3 則。

cursor 以「某一則訊息的位置」為錨點，伺服器解析成 `(created_at, id)` 做複合比較，新訊息插入不影響錨點。

`after=` 模式（重連補齊）的回應帶 `has_more`，前端**必須迴圈抓取直到 false**——離線一週後回來，一次只回 30 筆，若假設一次呼叫就補齊，中間的訊息會永遠遺失且沒有任何錯誤訊息。

### 已讀位置只前進不後退

使用者開兩個分頁，A 滑到最新標記已讀、B 還停在舊位置也送出標記——順序一亂，未讀數就會憑空長回。

```sql
ON CONFLICT (conversation_id, user_id) DO UPDATE
SET    last_read_at = EXCLUDED.last_read_at, ...
WHERE  EXCLUDED.last_read_at > conversation_reads.last_read_at;
```

最後一行的 `WHERE` 讓舊的標記無法覆蓋新的。

### 未讀數的 NULL 陷阱

從未讀過的對話沒有 `conversation_reads` 那一列，`LEFT JOIN` 出來是 NULL，而 **`created_at > NULL` 的結果是 NULL 不是 true**——未讀數會全部算成 0。必須用 `COALESCE(last_read_at, '-infinity'::timestamptz)`。

### 通知去重與「幽靈紅點」

通知以 `UNIQUE(recipient_id, actor_id, type, entity_id)` 去重，避免使用者連點按讚洗版對方的通知欄。

這讓「主操作成功」與「有新通知」脫鉤，因此 **`ON CONFLICT DO NOTHING` 影響 0 列時，絕對不能推送 WebSocket 事件**——否則前端紅點 +1 但列表無變化，產生一個清不掉的幽靈紅點（沒有那筆通知可以標為已讀）。

「DB 操作沒回錯就代表成功」在這裡不成立：`ON CONFLICT DO NOTHING` 影響 0 列時，它是「成功地什麼都沒做」。

### 通知寫入不可沿用 request context

主操作 commit 後，client 可能已斷線（使用者關頁面、冷啟動逾時），此時 `c.Request.Context()` 已被 cancel，通知的 INSERT 會直接失敗且靜默——變成「按讚成功但通知永久遺失」。

使用 `context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)`。

### 通知是同步寫入，不開 goroutine

通知寫入是主操作 commit 之後的次要副作用，失敗只記 log 不影響回應。但它**同步執行**，不包在 goroutine 裡：一來 goroutine 的 panic 會炸掉整個程序（gin 的 recovery middleware 管不到別的 goroutine），二來沒有 graceful shutdown 的等待機制，服務重啟時飛在半空的 goroutine 會直接消失。

代價只是回應多等一次單筆 INSERT。

### 資料修改型 CTE 看不到自己的寫入

標記已讀後要回傳新的未讀數，**不能**用 `WITH updated AS (UPDATE ...) SELECT count(*) ...` 一次算完——data-modifying CTE 與主查詢共用同一個 snapshot，主查詢看不到 CTE 的寫入效果，會回傳「更新前」的數字。

必須拆成兩句 SQL。這種 bug 手測時很難發現（數字確實有變小），要到前端接起來才會看出「紅點永遠差幾格」。

---

## 已知限制

以下項目為有意識的取捨，非疏漏：

| 項目 | 說明 |
|---|---|
| WebSocket 單一 instance | Hub 存在程序記憶體中，多 instance 需 Redis Pub/Sub |
| Rate limiting | 只有上傳簽章有每日配額（每人 50 個）；登入相關端點尚未限流 |
| 自動化測試 | WebSocket Hub 的併發測試（`go test -race`）與 Phase 5 的 Cloudinary 簽章、媒體驗證單元測試；其餘以手動驗收為主 |
| 多裝置 session 管理 | 無法「登出所有裝置」，需要 session 反向索引 |
| CSRF token | 目前依賴 SameSite；跨網域部署時需升級 |
| Cursor 分頁 | 僅聊天訊息採用，其餘為 offset 分頁 |
| 2 字元搜尋無索引 | trigram 抽取的原理限制（見「搜尋設計」） |
| 搜尋詞序敏感 | 搜「程式設計」找不到「設計程式」——trigram 是子字串比對而非語意斷詞 |
| 搜尋不索引內文 | 只索引標題與摘要（Neon 免費層 0.5GB 限制） |
| 中文 slug 與標籤 | 中文標題產生隨機 slug、中文標籤會被忽略 |
| 草稿裡的媒體 | Cloudinary 的網址是公開的：拿到網址的人可以看到草稿裡的圖片與影片（網址含 128 bit 隨機值，不可猜測，但不受權限保護） |
| 不自動清理媒體 | 沒被使用的檔案以 SQL 列出、人工清理；上傳後沒有回報的檔案只能在 Cloudinary 後台檢查 |
| 影片 | 每篇最多 1 部、無自適應串流（HLS）；第一次播放需等待轉檔 |
| Google 頭像 | 註冊時帶入的是指向 Google 的外部連結，可能失效（失效時顯示字母頭像）；使用者可自行上傳頭像取代 |
| Cloudinary 免費額度 | 圖片、影片、儲存、流量共用；額度耗盡或帳號停用時全站圖片同時失效 |
| 訊息編輯 / 刪除、群組聊天 | 未實作 |
| 已讀回條、正在輸入 | 未實作 |
| 留言分頁 | 前端一次載入前 100 則 |
| 免費方案冷啟動 | Render 閒置 15 分鐘後休眠，首次請求需 30–60 秒喚醒 |

---

## 維運

部署在四個免費方案上（Render、Neon、Upstash、Cloudinary），各自的閒置與額度政策不同：

- **Upstash 免費資料庫閒置一段時間會被封存**，封存後連線位址失效，網站無法登入。`.github/workflows/upstash-keepalive.yml` 每週一、四用**唯讀 token** 讀取一個固定的 sentinel key，並比對它的值——所以它同時是監控：資料庫被封存、被換掉、或 secret 設錯，workflow 都會失敗並寄信。需要在 repo 的 Actions secrets 設定 `UPSTASH_REDIS_REST_URL` 與 `UPSTASH_REDIS_REST_READONLY_TOKEN`，並在 Upstash 手動 `SET scp:keepalive:sentinel inkwell-prod`（這是 Redis 裡唯一的永久 key；重建資料庫時要重設）
- **public repo 連續 60 天沒有 commit，GitHub 會停用排程 workflow**，需要到 Actions 頁面重新啟用
- 保活**刻意不經過 Render、不呼叫 `/healthz`**：那會喚醒休眠中的服務（消耗 instance hours）與資料庫（消耗 Neon 的 compute 額度）
- Cloudinary 的流量是主要成本，所以文章頁的影片設為 `preload="none"`

---

## Roadmap

- [x] Phase 1：認證、個人檔案、文章 CRUD、標籤
- [x] Phase 2：按讚、留言、追蹤、動態牆
- [x] Phase 3：WebSocket 即時私訊、聊天歷史、未讀數
- [x] Phase 4：通知系統、全文搜尋
- [x] Phase 5：媒體上傳（頭像、文章圖片、影片附件）
- [ ] 測試擴充：Phase 1–4 的純函式與關鍵 repository
- [ ] 登入相關端點的 rate limiting

---

## License

MIT