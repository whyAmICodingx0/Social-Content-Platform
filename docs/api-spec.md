# Social Content Platform — REST API Specification v5.0

- **專案**：Social Content Platform（站名 Inkwell）
- **範圍**：Phase 1（Auth / Users / Posts / Tags）+ Phase 2（Likes / Comments / Follows / Feed）+ Phase 3（Messaging / WebSocket）+ Phase 4（Notifications / Search）+ Phase 5（Media：頭像、文章圖片、影片附件）
- **狀態**：v5.0 定稿（前後端合約基準線，取代 v4.0）
- **實作進度**：**Phase 1–5 全部完成並上線**——**40 個 REST 端點 + 1 個 WebSocket 端點**、14 張資料表、13 個前端頁面，決策與教訓編號至 #173。
- **線上網址**：https://inkwell-hq36.onrender.com
- **Migration**：goose；`db/migrations/` 為變更歷史的唯一真相，`db/schema.sql` 為衍生快照
- **登入架構**：Google OAuth 2.0（身分驗證）+ Redis server-side session（登入狀態）+ HttpOnly Cookie（攜帶 session ID）
- **媒體**：Cloudinary，簽章直傳（檔案不經過 Go 伺服器），回報後由後端以 Admin API 反查驗證
- **不採用**：JWT、PostgreSQL sessions 表、username 可為 NULL
- **API 風格定位**：resource-oriented REST API（Richardson 成熟度 Level 2；採 session-based auth，不追求嚴格 stateless 與 HATEOAS）

## v4.0 → v5.0 變更摘要（Phase 5）

新增 2 個端點、2 張資料表、`users` 新增 1 個欄位。**3 個既有端點有行為擴充**，皆為非破壞性（只新增選填欄位或回應欄位）。

| 區塊 | 內容 |
|---|---|
| 第 25 章（新） | Media API：上傳簽章、上傳回報（後端反查驗證）、purpose 規格、Delivery URL |
| 第 26 章（新） | 媒體與既有資源的整合：頭像、文章的 `media_ids`、Post Detail 的 `media`、摘要規則 |
| 第 27 章（新） | Phase 5 的橫向規則：不信任前端 metadata、dynamic folder mode、Admin API 紀律、草稿媒體洩漏、孤兒 |
| 6.1 | `PATCH /me` 新增 `avatar_media_id`（6.1.2，含合約變更 #147） |
| 7.1 / 7.5 | `POST /posts`、`PATCH /posts/{id}` 新增 `media_ids` |
| 7.4 / 11.2 | Post Detail 新增 `media` |
| 11.10 / 11.11（新） | Media 與 Post Detail 媒體項目的回應形狀 |
| 3.2 | 新增錯誤碼 `RATE_LIMITED`（429） |
| 0.3 | Cloudinary 列為第二個執行期例外 |
| 10 | 端點總覽擴充為 **40 個 REST + 1 WS** |
| 12.5（新） | Schema：`media`、`post_media`、`users.avatar_media_id` |

### P5-0 合約中已被推翻的條文（以本文件為準）

| P5-0 原文 | 現行 | 原因 |
|---|---|---|
| 簽 `folder`；驗證 `public_id` 前綴 | 簽並驗證 **`asset_folder`**（#134） | 帳號為 dynamic folder mode，`folder` 不反映在 `public_id` 上 |
| 簽章有效期 10 分鐘 | **1 小時**（#136） | Cloudinary 固定從 `timestamp` 起算 1 小時 |
| 每人每日 50 次「簽章」 | 每日 50 次 = **50 個檔案**（#135、#137） | 原設計的簽章可重複使用；改為後端產生 `public_id` + 簽入 `overwrite=false` 後變成單次 |
| Post Detail 只回傳影片 | **回傳全部媒體**，前端自行過濾（#133） | 否則編輯器拿不到圖片的 id，存檔時會清掉圖片關聯 |
| 影片封面 `so_1` | **`so_10p`**（#172，合約變更） | 短於 1 秒的影片會超出範圍 |
| 超額時「停止新活動、不產生帳單」 | **【未證實】** | 找不到一手來源；實際的保護是 #137 的配額 |
| 孤兒查詢只看 `post_media` | 只看**未刪除**文章的 `post_media`（#166） | 只掛在已刪除文章上的媒體永遠不會被算成孤兒 |

### 文件更正（非行為變更，對照實際程式與環境）

以下都是**文件寫錯或過時**，程式行為沒有改變：

| 位置 | 原文 | 更正 |
|---|---|---|
| 6.1.1 | 送出 `null` 視同送空字串 | **`null` 視同欄位缺席（不更新）**——Go 的 `encoding/json` 對兩者都產生 `nil`，同一節的說明本來就這樣寫 |
| 1.3 | 唯讀欄位回 `"read-only field"` | 回 **`"unknown field"`**——唯讀欄位不定義在 request struct 裡，與未知欄位由同一個機制擋下（6.1 的實作說明本來就這樣寫） |
| 第 10 章、4.8、6.2、6.3、7.2 | `GET /users/{username}`、`GET /users/{username}/posts`、`GET /posts` 為「無 / 純公開」；optional auth「目前只有讀單篇」 | 三者自 Phase 2 起都是 **optional auth**（決策 #47、18.2），`main.go` 也是 `auth.Optional()`。這幾處是 Phase 1 的舊值沒有跟著改 |
| 3.3 | 範例程式碼寫 `log.Printf("<handler>.<method>: ...")` | 改成具體範例。**這段佔位字被原封不動抄進了 `user.go` 與 `post.go`**（已在 P5-2、P5-3 修正），其他 handler 可能也有 |
| 4.2 / 4.3 / 4.4 | Redis key 只列三個 | 補上 `scp:upload_quota:*`（Phase 5）與 `scp:keepalive:sentinel`（**唯一的永久 key**） |
| 12.1 | 「Phase 1（自定案起零變更）」 | Phase 5 為 `users` 新增了 `avatar_media_id`，這句已不成立 |
| 12.4 | 「四個索引」 | 實際是**三個**（另外四個 trigram 索引在後面另列） |
| 附錄 C | rate limiting「部署對外前」處理 | OAuth 已發布、已對外；改為列入待辦 |

---

## v3.0 → v4.0 變更摘要（Phase 4）

新增 6 個端點、1 張資料表、1 個 PostgreSQL 擴充（`pg_trgm`）。既有端點無行為變更。

| 區塊 | 內容 |
|---|---|
| 第 22 章（新） | Notifications API：列表、未讀數、標記已讀（指定 / 全部） |
| 第 23 章（新） | Search API：文章搜尋、使用者搜尋 |
| 第 24 章（新） | Phase 4 的橫向規則：去重與幽靈紅點、best-effort 寫入、不過濾原則、搜尋的中文限制 |
| 11.8 / 11.9（新） | Notification 與 SearchUser 的回應形狀 |
| 10 | 端點總覽擴充為 **38 個 REST + 1 WS** |
| 12 | Schema：新增 `notifications` 表與四個 GIN trigram 索引 |
| 20.6 | WebSocket 新增 `notification.created` 事件型別 |

### ⚠️ 端點計數的更正

v3.0 的正文與 Instructions v8 曾出現「31」與「32」兩個數字。以實際 router 註冊逐行清點確認：**Phase 3 結束時為 32 個 REST 端點 + 1 個 WebSocket**（P3-4 追加的 `GET /conversations/unread-count` 讓原規劃的 5 個變成 6 個）。

Phase 4 新增 6 個 → **總計 38 個 REST 端點 + 1 個 WebSocket 端點**。本文件的第 10 章為唯一權威。

---

## v2.0 → v3.0 變更摘要（Phase 3）

新增 5 個端點、3 張資料表、1 個 WebSocket 端點。既有端點無行為變更。

| 區塊 | 內容 |
|---|---|
| 第 19 章（新） | Messaging API：對話 find-or-create、對話列表、送出訊息、歷史訊息、標記已讀、未讀總數 |
| 第 20 章（新） | WebSocket 合約：握手驗證、Hub 模型、心跳、session 重驗、event envelope、送達保證 |
| 第 21 章（新） | Phase 3 的橫向規則：cursor 分頁、冪等策略、非參與者 404、已讀單調性 |
| 11.6 / 11.7（新） | Conversation 與 Message 的回應形狀 |
| 10 | 端點總覽擴充為 31 個 |
| 12 | Schema：新增 3 張表（`conversations`、`messages`、`conversation_reads`） |

**唯一的分頁原則例外**：`GET /conversations/{id}/messages` 採 cursor（keyset）分頁，其餘端點沿用 offset。理由見 21.1。

---

## v1.5 → v2.0 變更摘要（Phase 2）

新增 9 個必要端點 + 2 個選做端點、3 張資料表。既有端點的回應形狀擴充（非破壞性）。

| 區塊 | 內容 |
|---|---|
| 第 14 章（新） | Likes API：`PUT` / `DELETE /posts/{id}/like` |
| 第 15 章（新） | Comments API：列表、新增、編輯、刪除 |
| 第 16 章（新） | Follows API：`PUT` / `DELETE /users/{u}/follow` + 兩個列表端點 |
| 第 17 章（新） | Feed API：`GET /feed` |
| 第 18 章（新） | Phase 2 的橫向規則：草稿可見性、個人化欄位與降級、計數聚合、soft delete 關聯政策 |
| 11.1 / 11.2 / 11.3 | User 與 Post 回應形狀擴充（計數與個人化欄位） |
| 10 | 端點總覽擴充為 26 個 |
| 12 | Schema：新增 3 張表（`post_likes`、`comments`、`follows`） |

**既有 15 個端點的行為變更**：`GET /posts`、`GET /users/{u}/posts`、`GET /users/{u}` 改為 optional auth（因新增個人化欄位）。其餘不變。

---

## v1.4 → v1.5 變更摘要（前端實作回饋）

前端完成後，將實作中確認的三項行為回填合約：

| 章節 | 變更 | 性質 |
|---|---|---|
| 5.6（新） | **導向類端點的錯誤處理原則**：`/auth/google/login` 與 `/callback` 的任何失敗一律 `302` 導向 `AUTH_ERROR_URL?error={code}`，不回 JSON | 行為變更 |
| 5.1 / 5.2 | 依 5.6 改寫錯誤情境；state 驗證失敗由 `400 VALIDATION_ERROR` 改為 `?error=invalid_state`；新增 `missing_code`、`service_unavailable` 兩個代碼 | 行為變更 |
| 7.5 | 補充說明「讀取用 slug、寫入用 id」的不對稱是刻意設計，並記錄前端編輯流程的因應方式 | 文件補充 |

**行為變更的影響範圍僅限那兩個 OAuth 導向端點**，其餘 13 個端點的合約完全不變。schema 仍無任何變更。

---

## v1.3 → v1.4 變更摘要（實作回饋收斂）

Phase 1 後端 15 個端點全部完成後，將實作過程中確認的行為細節回填合約。**無行為變更，僅補足先前未明文的規則**（這些規則在實作與驗收中已成立，本版讓文件與程式碼一致）：

| 章節 | 補充內容 |
|---|---|
| 6.1.1（新） | `PATCH` 的三態欄位語意：缺席 = 不更新、空字串 = 清空、`null` 視同空字串（Go json 無法區分後兩者，MVP 簡化）；`display_name` 不可清空；`avatar_url` 只驗長度不驗格式；長度以字元數計 |
| 6.3 | **修訂**：使用者不存在時回 `200` + 空陣列（原寫 404）。理由：不洩漏 username 是否存在、前端少一個分支 |
| 7.1 | `tags` 規則改為指向新增的 8.4 |
| 8.1 | `post_count` 的完整計算條件；`post_count` 為 0 的標籤仍列出（含 LEFT JOIN ON vs WHERE 的實作提示） |
| 8.3 | 不存在的 tag 篩選回空陣列 |
| 8.4（新） | 標籤正規化的完整六步規則、`name` 與 `slug` 同值的 MVP 簡化、中文標籤的已知取捨 |

schema 仍**無任何變更**。

---

## v1.2 → v1.3 變更摘要（實作回饋）

本版唯一實質變更：**5.3 signup 的並行安全與 idempotent 收斂**（新增 5.3.1）。

起因：v1.2 假設「撞 `oauth_accounts_provider_uid_key` → 視同已註冊」，但實作時發現 transaction 內 `users` 先插入，相同 Google 身分的重複／並行提交會**先撞 `users_email_key`**，永遠走不到該分支。修訂後改以 `(provider, provider_user_id)` 為唯一判斷錨點：任何 23505 → rollback → 交易外重查綁定 → 依重查結果收斂。同時明訂 aborted transaction 規則、軟刪 user 的異常分支、未預期 constraint 回 500，以及「驗證先於快路徑」的順序。

其餘章節與 v1.2 相同。schema 仍**無任何變更**。

---

## v1.1 → v1.2 變更摘要（第二輪審查結果）

1. **OAuth state 綁定瀏覽器**（5.1 / 5.2）：新增 `oauth_state` cookie，callback 需「query == cookie 且 Redis 一次性存在」同時成立，防 login CSRF。
2. **signup 失敗收斂策略**（5.3）：明文定義寫入順序與各失敗點的收斂結果；撞 `oauth_accounts` 唯一鍵改為「視同已註冊 → 直接登入」。
3. 新增 **`409 EMAIL_TAKEN`**：signup 撞 `users_email_key` 的行為定義（不自動連結）。
4. **CSRF 防護政策獨立成章**（4.13）：SameSite=Lax + 強制 JSON Content-Type（415）+ Origin 檢查 + GET 不變更狀態。
5. **auth middleware 分兩種模式**（4.8）：required / **optional**（無效憑證視同匿名，不回 401；Redis 掛掉時 optional 降級為匿名）。
6. **排序 tie-break**（9.2）：所有列表排序補第二排序鍵 `id`；`GET /tags` 排序補確定性規則。
7. **slug 產生改為「寫入重試」**（7.6）：以 DB unique violation（23505）為權威，取代先查再寫。
8. **文章寫入交易化**（7.1 / 7.5）：posts + tags upsert + post_tags 必須同一 transaction。
9. **Redis key 加應用前綴 `scp:`**（4.2）；環境隔離原則明文化。
10. **術語表**（1.2）：資源 id（UUID v4）與憑證 token（CSPRNG base64url）嚴格區分。
11. **嚴格 body 解析**（1.3）：未知欄位一律 400；`PATCH /me` 的 `username` / `email` 由「靜默忽略」改為 **400（read-only field）**。
12. **保留字擴充與 username 正規化管線**（5.3）。
13. 其他：logout 順手 DEL pending key（4.10）、session fixation 防護明文化（4.7）、刪帳號政策備註（附錄 B）、帳號連結（account linking）未來設計備註（附錄 B）、已知缺口清單（附錄 C）。

**Schema 影響：無。以上全部是應用層與合約層變更，`db/schema.sql` 維持不動。**

---

## 0. 閱讀指南與文件約定

### 0.1 這份文件是什麼

這是一份「合約」。前端照這份打 API，後端照這份實作。任何一邊要改欄位、路徑、格式，都必須先改這份文件、通知另一邊。

### 0.2 標記慣例

- **【事實】**：技術上就是這樣運作，或已拍板的決策。
- **【建議】**：取捨建議，可否決。
- **【決策】**：已拍板，是合約的一部分。

### 0.3 原則例外記錄

本專案 MVP 原則之一為「能用 PostgreSQL 解決的事情，先不要引入額外服務」。目前有**兩個執行期例外**：

> **Redis**，僅用於短暫狀態：登入 session、OAuth state、pending signup，以及 Phase 5 的上傳配額計數器（決策 #137）。理由：TTL 自動過期省去清理邏輯、皆為可拋棄狀態（丟失只需重新登入）、業界標準組合具學習價值。永久資料一律只放 PostgreSQL。
>
> **唯一的例外**是維運用的 `scp:keepalive:sentinel`：永久 key、不屬於應用資料、程式不讀也不寫，只給 Upstash 保活 workflow 讀取（4.2）。

> **Cloudinary**（Phase 5，決策 #108），用於媒體的儲存、轉檔與 CDN。理由：Render 免費方案為 0.1 CPU / 512MB / 暫時檔案系統，無法自跑 ffmpeg 轉檔；Neon 免費層 0.5GB，blob 不該進 PostgreSQL；純儲存方案（R2、Supabase）解決不了影片轉檔。**代價**：專案第一次依賴「用量計費」的外部服務，額度耗盡或帳號停用時**全站圖片一起失效**（#121）。

（goose 是開發期工具，不算執行期依賴。）

### 0.4 Schema 依據

本文件的資料庫依據為 repo 內的 **`db/schema.sql`（已定案並執行）**。與 API 相關的對應：

- API 的 `content` 對應 DB 的 `posts.content_md`。
- `posts.excerpt`、`posts.published_at` 已存在於 schema，直接使用。
- `users.username` 為 **`NOT NULL`**；註冊流程（5.2、5.3）即是為了在不放寬此約束的前提下完成 onboarding。
- soft delete 僅限 `users`、`posts`。**所有對這兩張表的查詢必須帶 `WHERE deleted_at IS NULL`**（封裝於 repository 層）。
- 無任何 trigger；`updated_at` 由 Go 在每次 UPDATE 時自行帶入 `now()`。
- 本文件引用的 constraint / index 名稱（錯誤處理分支依據）：`users_username_key`、`users_email_key`、`posts_author_slug_key`、`oauth_accounts_provider_uid_key`；Phase 5 新增 `media_provider_asset_key`（25.3 的冪等收斂）。

---

## 1. API 共通設計規則

| 項目 | 規則 |
|---|---|
| **Base URL** | `/api/v1` |
| **Request Content-Type** | `application/json`（有 body 的請求，**強制**，見 4.13） |
| **Response Content-Type** | `application/json`（OAuth redirect 端點除外，為 302） |
| **字元編碼** | UTF-8 |
| **JSON 欄位命名** | `snake_case` |
| **時間格式** | RFC 3339，一律 UTC |
| **認證方式** | HttpOnly Cookie 攜帶 session ID，不使用 Authorization header |
| **HTTP method** | `GET` 讀取、`POST` 建立、`PATCH` 部分更新、`DELETE` 刪除 |

- `PATCH`：只送要改的欄位，沒送的維持原值。
- `DELETE`：對 `posts` 是 soft delete。
- **idempotent（冪等）**：同一請求送 1 次和送 N 次結果相同。`DELETE`、`POST /auth/logout` 要求冪等。

### 1.2 識別碼與憑證的術語區分【決策】

| 類別 | 格式 | 出現位置 | 例子 |
|---|---|---|---|
| **資源識別碼（resource id）** | UUID v4 字串（DB `gen_random_uuid()`） | URL path、JSON body | `users.id`、`posts.id`、`tags.id` |
| **憑證 token（bearer token）** | CSPRNG 32 bytes → base64url（約 43 字元），**不是 UUID** | 只出現在 Cookie 與 Redis key，**絕不**出現在 JSON body 或 URL | session ID、OAuth state、pending token |

【決策】兩者不得混用：資源 UUID 絕不可充當憑證（可預測 / 可枚舉風險）；憑證 token 絕不可存入 PostgreSQL 當識別碼。

### 1.3 嚴格 request body 解析【決策】

- 所有 JSON body 以**嚴格模式**解析：出現合約未定義的欄位 → `400 VALIDATION_ERROR`，`details.fields.<name>: "unknown field"`。
- 出現已知但唯讀的欄位（如 `PATCH /me` 的 `username`、`email`）→ 同樣是 `400 VALIDATION_ERROR`、`details.fields.<name>: "unknown field"`。實作上唯讀欄位**不定義在 request struct 裡**，與未知欄位由同一個機制擋下（v5.0 更正：原文寫 `"read-only field"`，實際不是）。
- 【事實 / Go 實作】對應 `json.Decoder` 的 `DisallowUnknownFields()`；任務 E 的 request 綁定 helper 統一落實。
- 理由：靜默忽略會把前端 bug 藏起來（以為改成功了其實沒有）；嚴格拒絕讓合約違規在第一時間爆出來。

---

## 2. 統一成功回應格式

### 2.1 單一資源

```json
{ "data": { "id": "a1b2c3d4-....", "title": "My first post" } }
```

### 2.2 列表資源（含分頁）

```json
{
  "data": [ { "id": "..." }, { "id": "..." } ],
  "pagination": { "page": 1, "limit": 20, "total": 57, "total_pages": 3, "has_next": true }
}
```

### 2.3 無回傳內容

`DELETE`、`logout` 成功回 **`204 No Content`**。

---

## 3. 統一錯誤回應格式與 error code

### 3.1 錯誤格式

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": {
      "fields": { "username": "must be 3-30 chars, lowercase letters, numbers or underscore" }
    }
  }
}
```

- **前端以 `code` 判斷分支，不要用 `message`**。
- `details`：選填。

### 3.2 Error code 對照表

| code | HTTP status | 意義 | 典型情境 |
|---|---|---|---|
| `VALIDATION_ERROR` | 400 | 請求格式 / 欄位不合法（含未知欄位、唯讀欄位） | 缺必填、格式錯、多送欄位 |
| `UNAUTHENTICATED` | 401 | 受保護端點：未登入 / session 無效或過期 / pending 無效或過期 | 沒帶 cookie、Redis 查無 key |
| `FORBIDDEN` | 403 | 已登入但無權操作；或 Origin 檢查不通過（4.13） | 改別人的文章 |
| `NOT_FOUND` | 404 | 資源不存在 / 已 soft delete / 無權看到的 draft | 錯的 slug、已刪文章 |
| `USERNAME_TAKEN` | 409 | username 已被使用 | 註冊撞名 |
| `EMAIL_TAKEN` | 409 | email 已有既有帳號（不同 Google 身分撞 email） | 見 5.3 |
| `SLUG_CONFLICT` | 409 | slug 重試上限後仍衝突 | 極端情況（7.6） |
| `CONFLICT` | 409 | 其他資源狀態衝突 | 已登入者呼叫 signup |
| `UNSUPPORTED_MEDIA_TYPE` | 415 | 寫入端點的 Content-Type 不是 application/json | HTML form / 錯誤 client |
| `INTERNAL_ERROR` | 500 | 伺服器內部錯誤 | 未預期例外 |
| `CANNOT_MESSAGE_SELF` | 400 | 對自己開對話 | Phase 3（19.1） |
| `INVALID_CURSOR` | 400 | `before`/`after` 錨點不存在或不屬於本對話 | Phase 3（19.4） |
| `RATE_LIMITED` | 429 | 超過使用配額 | 每日上傳簽章超過 50 次（25.2，Phase 5） |
| `SERVICE_UNAVAILABLE` | 503 | 依賴服務暫時不可用 | Redis 連不上（4.12）；Cloudinary 未設定或 Admin API 失敗（第 25 章） |

> 【事實 / 安全設計】受保護端點的 `UNAUTHENTICATED` 統一涵蓋所有「憑證無效」情境，不對外區分，避免洩漏 session 是否存在。前端行為一律「導去登入」。

### 3.3 錯誤處理的實作通則【決策，v3.0 新增】

**所有 handler 的錯誤分支中，`default`（非預期錯誤）必須 `log.Printf`。**

`default` 的定義就是「我沒有預料到的錯誤」——那正是最需要線索的地方，卻最容易變成唯一沒有 log 的路徑。對外仍回統一的 `SERVICE_UNAVAILABLE`（不洩漏內部細節），log 只給維運者看。

```go
	default:
		// 「post handler: Create」要換成實際的 handler 與方法名稱，
		// 出錯時才能從 log 直接看出是哪一支端點
		log.Printf("post handler: Create: unexpected error: %v", err)
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
```

⚠️ v5.0 之前這裡的範例是 `log.Printf("<handler>.<method>: ...")`。**那段佔位字被原封不動抄進了 `user.go` 與 `post.go`**，出錯時 log 只會印出這串字，看不出位置。新增 handler 時照上面的具體格式寫：`<檔名> handler: <方法名>: unexpected error: %v`。
> 公開端點（含 optional auth）**永遠不會因為憑證問題回 401**，見 4.8。

---

## 4. Redis Session、Cookie 與 CSRF 合約

### 4.1 Cookie 一覽

| Cookie | 用途 | 生命週期 |
|---|---|---|
| `sid` | 正式登入 session | 7 天（與 Redis TTL 對齊） |
| `pending_sid` | pending signup（已通過 Google 驗證、尚未選 username） | 30 分鐘 |
| `oauth_state` | OAuth state 的瀏覽器綁定（5.1 / 5.2） | 10 分鐘 |

三者都只存不可預測的 CSPRNG token（1.2），不存任何使用者資料、不存 Google token。

### 4.2 Redis key 命名【決策】

```
scp:session:{session_id}                  登入 session，TTL 7 天
scp:oauth_state:{state}                   OAuth CSRF 防護（一次性），TTL 10 分鐘
scp:pending_signup:{token}                待完成註冊的 Google 資料，TTL 30 分鐘
scp:upload_quota:{user_id}:{YYYY-MM-DD}   上傳簽章的每日計數（Phase 5，UTC 日期），TTL 26 小時
scp:keepalive:sentinel                    Upstash 保活用的固定值（維運），無 TTL
```

- 前綴 `scp:`（social content platform）集中定義於 config 常數，不得散落字串。
- ⚠️ **`scp:keepalive:sentinel` 是唯一的永久 key**，也是唯一不由程式寫入的 key：它由人在 Upstash Console 手動設定（值為 `inkwell-prod`），只被 GitHub Actions 的保活 workflow 以唯讀 token 讀取。**重建 Upstash 資料庫時必須重新 `SET`**，否則保活 workflow 會失敗（這正是它作為監控的設計：資料庫被換掉時會報警）。
- 【決策 / 環境隔離】不同環境（dev / prod）**必須使用不同的 Redis 實例**（dev 即 docker 容器）；前綴是「不慎共用實例」時的第二道保險，不是隔離的主要手段。
- （正式環境管理 key 用 `SCAN`，不要用 `KEYS`。）

### 4.3 Redis value 格式

**scp:session:{id}**：

```json
{ "user_id": "a1b2c3d4-....", "created_at": "2026-07-12T08:30:00Z" }
```

**scp:pending_signup:{token}**：

```json
{
  "provider": "google",
  "provider_user_id": "1077...(sub)",
  "email": "alice@example.com",
  "name": "Alice Chen",
  "picture": "https://lh3.googleusercontent.com/...",
  "created_at": "2026-07-12T08:30:00Z"
}
```

**scp:upload_quota:{user_id}:{date}**：整數（`INCR` 的計數），見 25.2。

**scp:keepalive:sentinel**：字串 `inkwell-prod`。

- session 只放 `user_id`（每次請求以此查 DB 取最新資料）。
- 一律不放 Google access / refresh token。

### 4.4 TTL

| key | TTL | 說明 |
|---|---|---|
| `scp:session:*` | 604800（7 天） | 【決策】絕對過期。進階：sliding session，Phase 1 不做 |
| `scp:oauth_state:*` | 600 | 授權流程理應數十秒內完成 |
| `scp:pending_signup:*` | 1800 | 過期＝放棄註冊，重走 Google 登入即可，DB 不留痕跡 |
| `scp:upload_quota:*` | 93600（26 小時） | key 本身帶 UTC 日期，TTL 只是讓它自動消失；比 24 小時多 2 小時的餘裕。`INCR` 與 `EXPIRE` 包在 `MULTI` / `EXEC` 裡（#137） |
| `scp:keepalive:sentinel` | 無 | 永久。手動設定，見 4.2 |

### 4.5 Cookie 屬性

| 屬性 | `sid` | `pending_sid` | `oauth_state` |
|---|---|---|---|
| `HttpOnly` | true | true | true |
| `Secure` | 環境相依（4.6） | 環境相依 | 環境相依 |
| `SameSite` | `Lax` | `Lax` | `Lax` |
| `Path` | `/` | `/` | `/` |
| `Max-Age` | 604800 | 1800 | 600 |
| `Domain` | 不設定（host-only） | 不設定 | 不設定 |

> 名詞：`HttpOnly` = JS 讀不到，防 XSS 竊取；`Secure` = 只在 HTTPS 傳送；`SameSite=Lax` = 跨站請求大多不帶 cookie，但外站點連結進來的 top-level GET 導航仍會帶（OAuth 回跳依賴這點）。

### 4.6 本機開發 vs 正式環境

| 屬性 | 本機（http://localhost） | 正式（HTTPS，前後端同站） |
|---|---|---|
| `Secure` | `false`（否則瀏覽器不送 cookie，登入直接壞） | `true` |

用環境變數控制（如 `COOKIE_SECURE`），不要寫死。

> 【未來提醒—跨站部署】若前後端部署在不同網域，需 `SameSite=None` + `Secure=true`、CORS `Allow-Credentials: true` 且指定明確來源、前端 `credentials: 'include'`，**且必須升級 CSRF token 機制（見 4.13 第 6 條）**。Phase 1 不實作。

### 4.7 session 建立流程

發生在兩處：老用戶 callback 成功（5.2）與新用戶 signup 成功（5.3）。步驟：

1. 產生 session ID：CSPRNG（Go `crypto/rand`）32 bytes → base64url。
2. `SET scp:session:{session_id} <json> EX 604800`。
3. `Set-Cookie: sid={session_id}`（屬性見 4.5 / 4.6）。

【決策 / 安全 — session fixation 防護】session ID **一律由伺服器新產生**，絕不沿用任何客戶端提供的值（cookie、query、header 都不行）。

### 4.8 auth middleware：required 與 optional 兩種模式【決策】

**Required auth**（端點總覽標 `required` 者）：

1. 讀 `sid` cookie。沒有 → `401 UNAUTHENTICATED`。
2. `GET scp:session:{sid}`。
   - Redis 連線失敗 → `503 SERVICE_UNAVAILABLE`（fail closed）。
   - 查無 key → `401 UNAUTHENTICATED`。
3. 以 `user_id` 載入使用者（`deleted_at IS NULL`；已刪 → `401`）。
4. 掛到 request context，續行。

**Optional auth**（公開但行為隨身分變化的端點）：

| 端點 | 隨身分變化的部分 |
|---|---|
| `GET /users/{u}/posts/{slug}` | draft 可見性（僅作者）、`liked_by_me` |
| `GET /posts`、`GET /users/{u}/posts`（Phase 2） | `liked_by_me` |
| `GET /users/{u}`（Phase 2） | `followed_by_me` |
| `GET /search/posts`（Phase 4） | `liked_by_me` |

（v5.0 更正：原文寫「目前只有讀單篇」，是 Phase 1 的舊值；Phase 2 起就是上表，見決策 #47、18.2。）

行為：

1. 沒帶 `sid` → 匿名續行。
2. 帶了 `sid` 但查無 / 無效 / 使用者已刪 → **視同匿名續行，不回 401**。
3. 帶了 `sid` 但 **Redis 連線失敗** → **降級為匿名續行**（不 503）。理由【事實】：降級為匿名只會讓權限變小（draft 看不到、個人化欄位為 `false`、published 照常），是 fail-safe 方向；這與 4.12「公開讀取不因 Redis 而失敗」一致。
4. 驗證成功 → 掛上使用者，續行。

**純公開端點**（`GET /tags`、`GET /posts/{id}/comments`、`GET /search/users`、追蹤 / 粉絲列表等）：完全不碰 Redis。

### 4.9 session 過期行為

- TTL 到期 key 自動消失 → 受保護端點回 `401`。
- 前端收到 401：清空登入狀態、導去登入頁。
- 【建議】後端回 401 時順手 `Set-Cookie: sid=; Max-Age=0`。

### 4.10 登出

`POST /api/v1/auth/logout`：

1. 讀 `sid` cookie，若有 → `DEL scp:session:{sid}`。
2. 讀 `pending_sid` cookie，若有 → `DEL scp:pending_signup:{token}`。
3. 清除 `sid`、`pending_sid` 兩個 cookie（`Max-Age=0`）。
4. 回 `204 No Content`。

【決策 / 冪等】session 不存在、沒帶 cookie、Redis 掛掉都照樣清 cookie 並回 204（登出以「使用者端狀態被清掉」為準，Redis 殘留 key 自然過期）。

### 4.11 受保護端點的憑證錯誤統一回應

一律 `401` + `{ "error": { "code": "UNAUTHENTICATED", "message": "Authentication required" } }`。pending 過期同樣適用（前端導回登入重走 Google）。

### 4.12 Redis 無法連線的處理原則

【決策】**required auth 一律 fail closed；optional auth 與公開讀取降級續行。**

| 情境 | 處理 |
|---|---|
| required auth 驗證時 Redis 掛 | `503 SERVICE_UNAVAILABLE`，不放行 |
| callback / signup 需讀寫 Redis 時掛 | `503 SERVICE_UNAVAILABLE` |
| optional auth 時 Redis 掛 | 降級為匿名續行（4.8） |
| 上傳簽章時配額計數失敗（Phase 5） | 放行並記 log（#137）。實務上很少發生：此端點為 required auth，Redis 掛掉時通常在認證階段就已 503 |
| 純公開端點 | 不碰 Redis，照常回應 |
| 登出 | 照樣清 cookie 回 204 |
| 所有情況 | 記 error log；對外不洩漏 Redis 細節 |

### 4.13 CSRF 防護政策【決策，明文】

> CSRF（Cross-Site Request Forgery）：攻擊者的網站誘使**你的瀏覽器**帶著你的 cookie 對本站發出你不知情的請求。防護對象是「瀏覽器自動帶 cookie」這件事。

Phase 1 部署前提：**前後端同站（same-site）**。防護採多層疊加：

1. **`SameSite=Lax`（主防線）**：【事實】跨站發出的 POST / PATCH / DELETE 不會帶上本站 cookie，經典 form CSRF 直接失效。
2. **強制 Content-Type（第二層）**：所有有 body 的寫入端點要求 `Content-Type: application/json`，否則 `415 UNSUPPORTED_MEDIA_TYPE`。【事實】HTML form 只能送出 `application/x-www-form-urlencoded`、`multipart/form-data`、`text/plain` 三種，偽造不出 JSON content type（除非走 fetch，而 fetch 跨站又被第 1、3 層擋）。
3. **Origin 檢查（第三層）**：middleware 對所有不安全方法（POST / PATCH / DELETE）檢查 `Origin` header：存在且不在白名單（env `FRONTEND_ORIGIN`）→ `403 FORBIDDEN`；不存在（curl / Postman 等非瀏覽器客戶端）→ 放行（CSRF 威脅只存在於瀏覽器帶 cookie 的情境）。
4. **鐵則：GET 絕不變更狀態**。任何會寫入的操作不得做成 GET（`SameSite=Lax` 對 top-level GET 導航會帶 cookie，GET 變更狀態會打穿第 1 層）。
5. **OAuth 流程的 CSRF**：由 `state`（5.1 / 5.2 的 cookie 綁定 + Redis 一次性）獨立防護。
6. **未來跨站部署**：`SameSite=None` 會使第 1 層失效，屆時**必須**加 CSRF token（double-submit cookie 或同步 token），屬破壞性變更，開新版本處理。

---

## 5. Auth API

> OAuth 導向端點回應為 `302`，不是 JSON。

### 5.1 `GET /api/v1/auth/google/login`

啟動 Google 登入。

- **登入需求**：無
- **行為**：
  1. 產生隨機 `state`（CSPRNG token，見 1.2）。
  2. `SET scp:oauth_state:{state} "1" EX 600`。
  3. `Set-Cookie: oauth_state={state}`（屬性見 4.5）——**把 state 綁定到發起登入的這個瀏覽器**。
  4. `302` 導向 Google 授權頁：`client_id`、`redirect_uri`、`scope=openid email profile`、`state`、`response_type=code`。
- **回應**：`302 Found`
- **錯誤情境**：Redis 不可用（無法寫入 state）→ `302` 導向 `AUTH_ERROR_URL?error=service_unavailable`（見 5.6）

> 【事實 / 為什麼要綁定】若 state 只驗「Redis 存在」，攻擊者可用自己發起的合法 `code + state` 組成 callback URL 誘騙受害者點擊，受害者瀏覽器通過驗證後**被登入成攻擊者的帳號**（login CSRF），後續操作全進攻擊者帳號。綁定 cookie 後，攻擊者的 state 與受害者瀏覽器的 cookie 對不上，攻擊失效。
>
> 【進階備註】PKCE 可再加一層防 code 攔截；本專案為 confidential client（持有 client_secret）且已綁定 state，Phase 1 不加。

### 5.2 `GET /api/v1/auth/google/callback`

Google 授權後回跳。**本端點不建立任何 users 列**。

- **登入需求**：無
- **Query params**：`code`、`state`
- **行為**：
  1. **驗證 state（三項同時成立）**：query 有 `state`；`oauth_state` cookie 存在且**等於** query 的 state；`GETDEL scp:oauth_state:{state}` 存在。任一不成立 → `302` 導向 `AUTH_ERROR_URL?error=invalid_state`（v1.5 修訂，原為 `400`；理由見 5.6）。不論成敗都清除 `oauth_state` cookie（一次性）。
  2. 用 `code` 向 Google 換 token，取 userinfo：`sub`、`email`、`email_verified`、`name`、`picture`。
     - 【事實 / 安全】`email_verified` 為 false → 拒絕並導向前端錯誤頁（未驗證的 email 可被冒用）。
  3. 查 `oauth_accounts` 是否已有 `(provider='google', provider_user_id=sub)`：
     - **有（老用戶）** → 載入 user（`deleted_at IS NULL`）。
       - user 正常 → 建 session（4.7）→ `302` 導回首頁 `/`。
       - user 已軟刪 → 【決策】視為資料異常：記 error log、`302` 導向前端錯誤頁。（依附錄 B 的刪帳號政策，此狀態不應出現；此為防禦性規則。）
     - **無（新用戶）** → 產生 pending token → `SET scp:pending_signup:{token} <google資料JSON> EX 1800` → `Set-Cookie: pending_sid={token}` → `302` 導去前端 `/onboarding`。**此刻資料庫沒有任何寫入。**
- **錯誤情境【v1.5 修訂：全部改為 302 導向，見 5.6】**：

| 情況 | 導向 |
|---|---|
| 使用者在 Google 按拒絕（query 帶 `error`） | `?error=access_denied` |
| state 驗證失敗（缺少 / 與 cookie 不符 / Redis 查無） | `?error=invalid_state` |
| query 缺少 `code` | `?error=missing_code` |
| 向 Google 換 token 或取 userinfo 失敗 | `?error=google_exchange_failed` |
| `email_verified` 為 false | `?error=email_not_verified` |
| OAuth 綁定存在但 user 已軟刪（資料異常） | 記 error log → `?error=account_unavailable` |
| Redis 或其他依賴不可用 | `?error=service_unavailable` |

導向目標一律為 `AUTH_ERROR_URL`（環境變數，指向前端登入頁）。

### 5.6 導向類端點的錯誤處理原則【決策，v1.5 新增】

`GET /auth/google/login` 與 `GET /auth/google/callback` 是**瀏覽器導向端點**——使用者是在網址列的導航過程中抵達，不是由 JavaScript 以 `fetch` 呼叫。

【決策】**這兩個端點的任何失敗一律回 `302`，導向 `AUTH_ERROR_URL?error={code}`，不得回傳 JSON。**

- 理由：JSON 錯誤回應的讀者是程式碼，但這兩個端點的失敗結果會直接呈現在瀏覽器畫面上——使用者看到的是一坨裸露的 JSON，且停留在後端網域，無法回到應用程式。導向前端則能顯示可讀的訊息並提供重試入口。
- 【事實】安全性不受影響：驗證邏輯完全不變，只有失敗後的呈現方式不同。錯誤代碼是刻意粗粒度的，不洩漏內部細節。
- 前端負責把 `error` 代碼對映成人類可讀訊息；未知代碼應退回通用訊息。
- **適用範圍僅限這兩個端點**。`POST /auth/signup`、`POST /auth/logout`、`GET /me` 及其餘所有端點都是 fetch 呼叫的 JSON API，維持第 3 章的錯誤格式。

### 5.3 `POST /api/v1/auth/signup`

完成註冊：選定 username，**此刻才建立 users + oauth_accounts**。

- **登入需求**：需持有有效 `pending_sid`；若請求同時帶**有效的正式 session（sid）** → `409 CONFLICT`。
- **Request body**：

```json
{ "username": "alice", "display_name": "Alice" }
```

**username 正規化管線【決策】**（依序執行）：

1. trim 前後空白。
2. Unicode 正規化（NFKC；全形字元轉半形等。因最終字元集僅收 ASCII，此步主要是讓「ａｌｉｃｅ」這類輸入被正確理解，而非直接打回）。
3. 轉小寫。
4. Regex 驗證：`^[a-z0-9_]{3,30}$`。不合 → `400`。
5. 保留字檢查（清單見下）。命中 → `400`。
6. **最終仲裁交給 DB**：`users_username_key` partial unique index。應用層檢查只是 UX，權威是 DB constraint。

**保留字清單【決策】**（集中放 config，可擴充）：
`me, admin, administrator, api, auth, login, logout, signup, signin, signout, onboarding, users, user, posts, post, tags, tag, settings, account, accounts, profile, support, help, system, moderator, mod, root, null, undefined, official, security, abuse, about, terms, privacy, search, explore, new, edit, delete, static, assets, www, mail, feed, home`

- `display_name`：選填，1–50 字；未給則預設用 Google 的 `name`。

#### 5.3.1 並行安全與 idempotent 收斂【決策，v1.3 修訂】

本端點必須支援相同 Google 身分的重複提交與並行提交。**判斷「已註冊」的錨點一律是 `(provider, provider_user_id)`，不得以 unique violation 撞到哪個 constraint 作為依據。**

**處理流程（順序即合約）**：

1. 讀取並驗證 `pending_signup`（**先不刪**）。若請求持有有效正式 session → `409 CONFLICT`。
2. 嚴格解析、驗證並正規化 request body。**驗證先於一切**：格式錯誤一律 `400`，不因是否已註冊而改變（錯誤優先級一致性）。
3. 建立帳號前，先依 `(provider, provider_user_id)` 查詢 `oauth_accounts`（快路徑）：
   - 綁定存在且對應 user 未刪除 → 視為先前註冊已完成：為既有 user 建正式 session、清除 pending、回 **`200 OK`**。
   - 綁定存在但 user 已軟刪 → 資料異常（依附錄 B-1 不應出現）：記 error log、回 `500 INTERNAL_ERROR`。
   - 不存在 → 續行。
4. **Transaction**：`INSERT users`（username、email、display_name、avatar_url ← Google `picture`）→ `INSERT oauth_accounts` → `COMMIT`。
5. 成功 → 建立正式 session（4.7）→ `DEL scp:pending_signup:{token}` 與清除 `pending_sid` cookie（**best-effort**，失敗不影響回應）→ `Set-Cookie: sid=...` → 回 **`201 Created`**。

**Unique violation（23505）處理**：

【事實】transaction 內任何語句出錯後，交易即進入 aborted 狀態（SQLSTATE 25P02），**必須先 rollback，不得在 aborted transaction 內繼續查詢**。

rollback 後，**重新**依 `(provider, provider_user_id)` 查詢 `oauth_accounts`（此查詢在交易外執行）：

| 重查結果 | 回應 |
|---|---|
| 綁定已存在且 user 未刪除 | 其他並行請求或先前請求已完成註冊 → 不建立第二個 user，為既有 user 建 session、清 pending、回 **`200 OK`** |
| 綁定存在但 user 已軟刪 | 記 error log → `500 INTERNAL_ERROR` |
| 綁定仍不存在，原 constraint = `users_username_key` | `409 USERNAME_TAKEN`（pending 保留，使用者換名直接重試） |
| 綁定仍不存在，原 constraint = `users_email_key` | `409 EMAIL_TAKEN`（【決策】**不自動連結**：不同 Google `sub` 撞同一 email 代表身分歸屬不明，寧可擋下，引導使用者用原本的 Google 帳號登入） |
| 綁定仍不存在，其他未預期 constraint | 記 error log → `500 INTERNAL_ERROR` |

**設計理由**：

- `users` 先於 `oauth_accounts` 插入，故相同 Google 身分的並行 signup 多半**先撞 `users_email_key`**（同帳號 email 相同），不一定走到 `oauth_accounts_provider_uid_key`。因此 constraint 種類不能當「已註冊」的判斷依據；唯有重查綁定才能正確區分三種情況：同身分的並行／重複提交、不同身分真撞 email、username 確實被他人佔用。
- 【事實 / 無新增 race window】PostgreSQL 對併發唯一性衝突的行為：後到的 INSERT 會**阻塞等待**先到交易的結果，先到者 commit 後才回傳 23505。因此收到 23505 的那一刻，對方的 users + oauth_accounts 必已原子性 commit，rollback 後的重查**保證**看得到綁定。
- 未預期 constraint 回 `500` 而非 `503` 的理由：現行 schema 下此分支實質不可達（僅三個具名 unique constraint，主鍵為 `gen_random_uuid()`）；若觸發必為程式或 schema 漂移的 bug，屬非暫時性故障，log 為第一優先。

**失敗收斂表【決策】**（PostgreSQL 與 Redis 是兩個系統，不可能原子；本表定義每個失敗窗口的收斂結果）：

| 失敗點 | 系統狀態 | 使用者體感 | 收斂方式 |
|---|---|---|---|
| 第 4 步 transaction 失敗（非 23505） | DB 無任何寫入，pending 仍在 | 收到錯誤 | 直接重送 signup |
| 第 5 步建 session 失敗 | **帳號已建立**、未登入、pending 仍在 | `503` | 重走 Google 登入 → callback 走老用戶路徑直接登入 |
| 第 5 步 DEL pending 失敗 | 帳號已建立、已登入、pending 殘留 | 無感 | 殘留 key 30 分鐘自動過期；期間重送 signup → 快路徑認出綁定 → `200` 視同登入 |

結論【事實】：所有失敗窗口的最壞結果都是「多走一次 Google 登入」，不存在資料損毀或永久卡死狀態。

- **回應**：全新帳號 → `201` + Me shape（5.5）；走「視同已註冊」收斂路徑 → `200` + Me shape。

> 【決策 / MVP】username **一次設定、Phase 1 不開放修改**。進階版本：可改 username + 舊 handle 重導。
> 【建議 / 進階】即時檢查 username 可用性的端點（`GET /auth/username-availability`）屬體驗優化，Phase 1 不做。

### 5.4 `POST /api/v1/auth/logout`

- **登入需求**：無（冪等）
- **行為**：見 4.10
- **回應**：`204 No Content` + 清除 cookie

### 5.5 `GET /api/v1/me`

- **登入需求**：要（required auth）
- **回應 `200`**：

```json
{
  "data": {
    "id": "a1b2c3d4-....",
    "username": "alice",
    "display_name": "Alice",
    "bio": "Hello world",
    "avatar_url": "https://...",
    "email": "alice@example.com",
    "created_at": "2026-07-01T00:00:00Z",
    "updated_at": "2026-07-10T12:00:00Z"
  }
}
```

- `username` 保證有值。`email` 只在本端點出現。
- **錯誤**：未登入 → `401`；Redis 掛 → `503`。

---

## 6. Users API

### 6.1 `PATCH /api/v1/me`

更新自己的個人檔案。

- **登入需求**：要
- **允許欄位**（皆選填）：`display_name`（1–50 字）、`bio`（上限 500 字）、`avatar_url`（上限 500 字）、**`avatar_media_id`**（Phase 5，見 6.1.2）
- 【決策，v1.2 變更】body 出現 `username` 或 `email` → `400 VALIDATION_ERROR`；出現任何未知欄位 → `400`（1.3 嚴格解析）。**不再靜默忽略。**
  - 【實作說明】唯讀欄位與未知欄位由同一個機制處理：request struct 不定義 `username` / `email`，`DisallowUnknownFields` 即擋下並回 400。
- **回應 `200`**：Me shape。
- 後端維護 `updated_at`（決策 #10，無 trigger）。
- **錯誤**：格式不合 `400`；未登入 `401`；Content-Type 不對 `415`。

#### 6.1.1 部分更新的欄位語意【決策，v1.4 補】

`PATCH` 的每個欄位有三種可能狀態，本專案採**簡化語意**：

| body | 行為 |
|---|---|
| 欄位缺席（如 `{}`） | **不更新**該欄位 |
| 送出值（如 `{"bio": "x"}`） | 更新為該值 |
| 送出空字串（`{"bio": ""}`） | **清空**（存為 NULL）。`display_name` 例外：空值視同不更新 |
| 送出 `null` | **視同欄位缺席（不更新）**（見下方說明；v5.0 更正） |

- 【事實 / 簡化理由】Go 的 `encoding/json` 對「欄位不存在」與「值為 null」都會讓 `*string` 維持 `nil`，兩者無法區分。要完整區分需 `json.RawMessage` 或自訂 UnmarshalJSON，複雜度顯著上升。MVP 採簡化語意：`null` 與「缺席」行為相同（不更新）；要清空欄位請送空字串。
- `display_name` **不允許清空**（它是顯示用的招牌，空值語意不佳）；`bio`、`avatar_url` 可清空。
- 【MVP 取捨】`avatar_url` **只驗證長度上限 500，不驗證 URL 格式或可達性**。Phase 5 起可改用上傳頭像（6.1.2），網址由後端產生。
- 長度計算一律以**字元數**（`utf8.RuneCountInString`）而非 byte 數，避免中文使用者被誤判超長。

#### 6.1.2 頭像：`avatar_url` 與 `avatar_media_id`【決策 #111、#147，Phase 5】

| 欄位 | 角色 |
|---|---|
| `avatar_url` | **顯示權威**：畫面上顯示哪張圖。來源可能是 Google OAuth、使用者貼的外部網址，或上傳後由後端寫入 |
| `avatar_media_id` | **生命週期指標**：這張頭像是不是本站託管的。NULL = 不是（不歸本站管理）。**不出現在任何回應中** |

**規則**：

| 請求內容 | 行為 |
|---|---|
| 只給 `avatar_media_id` | 驗證後 → `avatar_url = media.url`、`avatar_media_id = 該 id` |
| 只給 `avatar_url`，**與目前的值不同** | 設定 `avatar_url`，`avatar_media_id = NULL` |
| 只給 `avatar_url`，**與目前的值相同** | **兩者都不變**（決策 #147） |
| `avatar_url` 給空字串 | 兩者都清空 |
| **同時給兩者** | `400 VALIDATION_ERROR`（`details.fields.avatar_media_id`）——語意衝突，不猜使用者的意圖 |

**`avatar_media_id` 的驗證**：合法 UUID、存在、`owner_id` 為本人、`purpose = 'avatar'`、`status = 'ready'`。任一不符 → `400`，`details.fields.avatar_media_id: "invalid avatar media"`，**刻意不區分原因**。UUID 格式在查資料庫**之前**檢查——否則不合法的字串會讓 PostgreSQL 回 22P02，變成 503。

**決策 #147（合約變更，P5-2）**：設定頁每次儲存都送出整份表單，包含沒改的 `avatar_url`。若沒有「相同值不變」這條，上傳頭像後只要改一次簡介，`avatar_media_id` 就會被清成 NULL——畫面不變，但這張頭像會被孤兒查詢誤判為孤兒。這條由後端保證，不依賴客戶端「只送有改的欄位」。

【事實 / SQL】一句 `UPDATE` 就能表達上述規則，因為 `SET` 子句等號右邊讀到的欄位一律是**更新前**的值：比對 `avatar_url` 是否「與目前的值相同」時，即使同一句也正在改 `avatar_url`，讀到的仍是舊值。

換頭像時**舊的 media 列不刪**，成為孤兒（27.6）。

### 6.2 `GET /api/v1/users/{username}`

- **登入需求**：無（**optional auth**，Phase 2 起，見 4.8、18.2）
- **回應 `200`**：User 公開 shape（11.1）：`id`、`username`、`display_name`、`bio`、`avatar_url`、`created_at`，加上 Phase 2 的 `follower_count`、`following_count`、`followed_by_me`（不含 email）。
- username 比對大小寫不敏感。
- **錯誤**：查無 / 已 soft delete → `404`。

### 6.3 `GET /api/v1/users/{username}/posts`

- **登入需求**：無（**optional auth**，Phase 2 起；影響 `liked_by_me`）
- **Query**：分頁與排序（第 9 章）
- 只回 `status='published'` 且 `deleted_at IS NULL`。作者已 soft delete 時，其文章一律不出現。
- **回應 `200`**：Post Summary 陣列 + `pagination`
- 【決策，v1.4 修訂】**使用者不存在時回 `200` + 空陣列**（原 v1.3 寫 `404`）。理由：(1) 不洩漏某 username 是否存在；(2) 前端少一個錯誤分支，列表為空與使用者不存在的畫面處理相同。若需要區分，前端可另外呼叫 `GET /users/{username}`（該端點仍回 404）。

---

## 7. Posts API

### 7.1 `POST /api/v1/posts`

- **登入需求**：要
- **Request body**：

```json
{
  "title": "My first post",
  "content": "# Hello\n\nThis is **markdown**.",
  "status": "draft",
  "tags": ["go", "backend"],
  "media_ids": ["uuid", "uuid"]
}
```

- **欄位規則**【決策】：
  - `title`：必填，1–200 字。
  - `content`：必填，Markdown 原文（存入 `content_md`）。
  - `status`：選填，`draft` / `published`，預設 `draft`；建立即 `published` → `published_at = now()`。
  - `tags`：選填，最多 5 個（超過 → `400`）；正規化規則見 8.4。
  - `media_ids`：選填（Phase 5），這篇文章擁有的媒體。驗證與錯誤見 26.2。
- **slug**：後端從 title 產生（7.6）。
- **excerpt**：寫入時從 markdown 產生存入（去語法取前約 200 字；Phase 5 起圖片整段移除、連結只留文字，見 26.4）。
- 【決策，v1.2；Phase 5 擴充】**本端點所有寫入（posts、tags upsert、post_tags、post_media）必須在同一個 transaction 內完成**，不允許半成品狀態。tags upsert 用 `INSERT ... ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name RETURNING id`（【事實 / 常見坑】`DO NOTHING` 不回傳列、拿不到既有 id，所以用 `DO UPDATE` 形式）。
- **回應 `201`**：Post Detail（含 `media`，26.3）。
- **錯誤**：`400` / `401` / `415` / `409 SLUG_CONFLICT`（極端）。

### 7.2 `GET /api/v1/posts`

- **登入需求**：無（**optional auth**，Phase 2 起；影響 `liked_by_me`）
- **Query**：`page`、`limit`、`sort`、`tag`
- 只回 published 且未刪。
- **回應 `200`**：Post Summary 陣列 + `pagination`

### 7.3 `GET /api/v1/me/posts`

- **登入需求**：要
- **Query**：`page`、`limit`、`sort`、`status=draft|published`
- **回應 `200`**：Post Summary 陣列 + `pagination`

### 7.4 `GET /api/v1/users/{username}/posts/{slug}`

- **登入需求**：無（**optional auth**，見 4.8）
- **回應 `200`**：Post Detail（含完整 `content`；Phase 5 起含 `media`，見 26.3）
- **可見性**：published 且未刪 → 任何人；draft → 僅作者（optional auth 驗出的本人），其他人一律 `404`；已刪 → `404`。
- **錯誤**：找不到 / 無權看 → `404`（不回 403，避免洩漏 draft 存在）。

### 7.5 `PATCH /api/v1/posts/{id}` 與 `DELETE /api/v1/posts/{id}`

> **【v1.5 補充】讀寫識別方式不對稱是刻意設計**
>
> 讀取單篇用「作者 + slug」（7.4），寫入用「文章 id」。Phase 1 **不提供** `GET /posts/{id}`。
>
> 前端編輯流程因此為：以 `GET /users/{username}/posts/{slug}` 取得文章（回應含 `id`）→ 用該 `id` 呼叫 `PATCH` / `DELETE`。編輯頁網址設計為 `/@username/slug/edit`。
>
> 此設計可行的前提是**改標題不改 slug**（決策 #5），因此編輯後網址仍然有效。若未來支援手動修改 slug，需一併新增 `GET /posts/{id}`。

**PATCH**

- **登入需求**：要，且必須是作者
- **允許欄位**（皆選填）：`title`、`content`、`status`、`tags`、`media_ids`（Phase 5）
- **行為**：
  - 更新欄位、content 有變則重算 `excerpt`（26.4）、維護 `updated_at`。
  - `tags` 提供 → 整組取代重建 `post_tags`（`[]` 清空，不傳不動）。
  - `media_ids` 提供 → 整組取代 `post_media`（`[]` 清空，不傳或 `null` 不動，26.2）。
  - `draft → published` 且 `published_at IS NULL` → 設 `published_at = now()`（語意為**首次發布時間**：改回 draft 不清除、再發布不更新）。
  - 改 `title` 不改 slug。
  - 【決策，v1.2；Phase 5 擴充】**與 7.1 相同的 transaction 要求**（posts 更新 + tags upsert + post_tags 重建 + post_media 重建同一交易）。
- **回應 `200`**：Post Detail（含 `media`）
- **錯誤**：`401` / `403 FORBIDDEN`（非作者）/ `404` / `400` / `415`。

**DELETE（soft delete）**

- **登入需求**：要，且必須是作者
- **行為**：設 `deleted_at = now()`。找不到未刪目標 → `404`。
- **回應**：`204`

### 7.6 slug 產生規則【決策，v1.2 改為寫入重試】

1. 由 `title` 產生 base slug：轉小寫 → 非英數轉 `-` → 去頭尾與重複 `-` → 截斷 80 字。轉換後為空（如全中文標題）→ 直接用短亂數（`post-a1b2c3`）。
2. 【事實 / 為什麼不能「先查再寫」】先 SELECT 檢查再 INSERT 存在 TOCTOU race（time-of-check to time-of-use）：兩個併發請求同時通過檢查，其中一個必在寫入時失敗。**唯一性的權威是 DB 的 `posts_author_slug_key`，不是應用層檢查。**
3. 流程：以候選 slug 執行 transaction（7.1）→ 捕獲 unique_violation（SQLSTATE `23505`，constraint = `posts_author_slug_key`）→ 產生下一個候選（`-2`、`-3`…）→ **重試整個 transaction**（【事實】PostgreSQL 中語句失敗會使當前交易中止，不能在同一交易內接著寫）。
4. 重試上限 5 次 → 改用短亂數尾碼再試 1 次 → 仍失敗 → `409 SLUG_CONFLICT`。
5. slug 建立時固定；改標題不改 slug。進階：手動改 slug + 舊 slug 重導，Phase 1 不做。

### 7.7 draft / published 行為彙整

| 行為 | draft | published |
|---|---|---|
| 出現在 `GET /posts` | 否 | 是 |
| 出現在 `GET /users/{u}/posts` | 否 | 是 |
| 出現在 `GET /me/posts` | 是 | 是 |
| 經 `GET /users/{u}/posts/{slug}` 讀取 | 僅作者；他人 404 | 任何人 |
| 可被作者 PATCH / DELETE | 是 | 是 |

### 7.8 權限規則彙整

- 建立文章：任何登入者。更新 / 刪除：僅作者（比對 session 的 `user_id` 與 `posts.author_id`）。非作者 `403`；未登入 `401`。

### 7.9 soft delete 行為彙整

- `DELETE` 只設 `deleted_at`。所有 `posts` / `users` 查詢必帶 `WHERE deleted_at IS NULL`（repository 層封裝）。
- 已刪資源對所有端點視為不存在（404）。
- 唯一性與 soft delete 共存：schema 的 partial unique index 已處理；刪文後同作者可重用 slug。

---

## 8. Tags API

### 8.1 `GET /api/v1/tags`

- **登入需求**：無
- **Query**：`page`、`limit`、`sort=popular|name`（預設 `popular`）
- **排序【決策，v1.2 補確定性】**：`popular` = `post_count DESC, slug ASC`；`name` = `slug ASC`。
- **回應 `200`**：`{ id, name, slug, post_count }` 陣列 + `pagination`。
- 【決策，v1.4 補】`post_count` 只計「`status='published'`、`posts.deleted_at IS NULL`、且作者 `users.deleted_at IS NULL`」的文章。
- 【決策，v1.4 補】**`post_count` 為 0 的標籤仍會出現在列表中**（標籤一旦建立就不會消失，即使其文章全部轉為草稿或被刪除）。
  - 【實作提示】上述過濾條件必須寫在 `LEFT JOIN ... ON` 子句而非 `WHERE`：寫在 `WHERE` 會把沒有符合文章的標籤整列濾掉，`post_count` 永遠不會是 0。
- 【已知缺口】目前沒有清理孤兒標籤（`post_count` 長期為 0）的機制，Phase 1 資料量下不構成問題。

### 8.2 標籤的建立（無獨立 POST）

【決策 / MVP】標籤在建立 / 更新文章時隱式 upsert（見 7.1 的 ON CONFLICT 寫法），與文章寫入同一 transaction。

### 8.3 用標籤篩選文章

`GET /api/v1/posts?tag={slug}`（單一標籤）。不存在的 tag → `200` + 空陣列，不回 404。

多標籤 AND / OR 屬進階，Phase 1 不做。

### 8.4 標籤正規化規則【決策，v1.4 補】

建立 / 更新文章時，後端對每個標籤依序執行：

1. trim 前後空白 → 轉小寫。
2. 非英數字元轉 `-`，收斂連續 `-`，去除頭尾 `-`（與 slug 相同的規則）。
3. 截斷至 50 字元。
4. **正規化後為空字串者直接忽略**（不報錯）。純中文或純符號的標籤會落入此情況。
5. 去重（同一篇文章內）。
6. 去重後數量超過 5 → `400 VALIDATION_ERROR`（`details.fields.tags`）。

- 【MVP 簡化】`tags.name` 與 `tags.slug` 存入**相同的值**（顯示名稱即 slug）。進階版本可保留原始大小寫作為 `name`、slug 化後作為 `slug`，屆時需處理「同 slug 不同 name」的併用問題。
- 【事實 / 中文提醒】此規則使中文標籤無法建立（步驟 4 會忽略）。與 slug 相同，屬 Phase 1 已知取捨；進階版本可改用允許 Unicode 的標籤鍵。

---

## 9. 分頁、排序與篩選

### 9.1 分頁（offset）

`page`（預設 1，最小 1）+ `limit`（預設 20，最大 100）。超界夾住。回應含 `pagination`（2.2）。

> 【進階】大資料量改 cursor 分頁，屬破壞性變更，屆時開新版本。

### 9.2 排序【決策，v1.2 補 tie-break】

【事實】SQL 對排序鍵相同的列**不保證**任何固定順序；沒有第二排序鍵時，同一頁在兩次查詢間可能出現重複或漏列，分頁會不穩定。因此所有列表一律加 `id` 作 tie-break：

| 端點 | `sort=newest`（預設） | `sort=oldest` |
|---|---|---|
| `GET /posts`、`GET /users/{u}/posts` | `ORDER BY published_at DESC, id DESC` | `ORDER BY published_at ASC, id ASC` |
| `GET /me/posts` | `ORDER BY created_at DESC, id DESC` | `ORDER BY created_at ASC, id ASC` |

- 非法 `sort` 值 → 回退 `newest`。
- 【事實】此變更**不需要**改索引：tie-break 影響的是結果確定性，索引只影響速度；Phase 1 規模下現有索引足夠。未來調優才考慮把 `id` 併入複合索引，屬非必要優化。

### 9.3 篩選

| 端點 | 篩選 |
|---|---|
| `GET /posts` | `tag={slug}` |
| `GET /me/posts` | `status=draft|published` |

---

## 10. 端點總覽

| # | Method | Path | 認證 | 說明 |
|---|---|---|---|---|
| 1 | GET | `/api/v1/auth/google/login` | 無 | 導向 Google 授權（設 state cookie） |
| 2 | GET | `/api/v1/auth/google/callback` | 無 | 回跳；老用戶建 session、新用戶進 pending |
| 3 | POST | `/api/v1/auth/signup` | pending | 選 username、建帳號、建 session |
| 4 | POST | `/api/v1/auth/logout` | 無（冪等） | 登出、清 session / pending 與 cookie |
| 5 | GET | `/api/v1/me` | required | 目前登入者完整檔案 |
| 6 | PATCH | `/api/v1/me` | required | 更新個人檔案（username/email 唯讀） |
| 7 | GET | `/api/v1/users/{username}` | **optional** | 公開個人頁 |
| 8 | GET | `/api/v1/users/{username}/posts` | **optional** | 某人已發布文章列表 |
| 9 | POST | `/api/v1/posts` | required | 建立文章 |
| 10 | GET | `/api/v1/posts` | **optional** | 全站已發布文章列表 |
| 11 | GET | `/api/v1/me/posts` | required | 自己文章（含 draft） |
| 12 | GET | `/api/v1/users/{username}/posts/{slug}` | **optional** | 讀單篇（draft 僅作者） |
| 13 | PATCH | `/api/v1/posts/{id}` | required（作者） | 更新文章 |
| 14 | DELETE | `/api/v1/posts/{id}` | required（作者） | soft delete |
| 15 | GET | `/api/v1/tags` | 無 | 標籤列表 |
| **— Phase 2 —** | | | | |
| 16 | PUT | `/api/v1/posts/{id}/like` | required | 按讚（冪等） |
| 17 | DELETE | `/api/v1/posts/{id}/like` | required | 取消讚（冪等） |
| 18 | GET | `/api/v1/posts/{id}/comments` | 公開 | 留言列表 |
| 19 | POST | `/api/v1/posts/{id}/comments` | required | 新增留言 |
| 20 | PATCH | `/api/v1/comments/{id}` | required（留言作者） | 編輯留言 |
| 21 | DELETE | `/api/v1/comments/{id}` | required（留言作者或文章作者） | 刪除留言 |
| 22 | PUT | `/api/v1/users/{username}/follow` | required | 追蹤（冪等） |
| 23 | DELETE | `/api/v1/users/{username}/follow` | required | 取消追蹤（冪等） |
| 24 | GET | `/api/v1/feed` | required | 追蹤動態 |
| 25 | GET | `/api/v1/users/{username}/followers` | 否 | 粉絲列表 |
| 26 | GET | `/api/v1/users/{username}/following` | 否 | 追蹤中列表 |
| **— Phase 3 —** | | | | |
| 27 | POST | `/api/v1/conversations` | required | 建立或取得與某人的對話（冪等） |
| 28 | GET | `/api/v1/conversations` | required | 我的對話列表 |
| 29 | GET | `/api/v1/conversations/unread-count` | required | 未讀總數（header 紅點） |
| 30 | GET | `/api/v1/conversations/{id}/messages` | required | 歷史訊息（**cursor 分頁**） |
| 31 | POST | `/api/v1/conversations/{id}/messages` | required | 送出訊息（冪等） |
| 32 | POST | `/api/v1/conversations/{id}/read` | required | 標記已讀（只前進不後退） |
| **— Phase 4 —** | | | | |
| 33 | GET | `/api/v1/notifications` | required | 通知列表 |
| 34 | GET | `/api/v1/notifications/unread-count` | required | 未讀數（header 紅點） |
| 35 | POST | `/api/v1/notifications/read` | required | 標記指定幾筆為已讀 |
| 36 | POST | `/api/v1/notifications/read-all` | required | 全部標記已讀（無 body） |
| 37 | GET | `/api/v1/search/posts` | **optional** | 搜尋文章 |
| 38 | GET | `/api/v1/search/users` | 否 | 搜尋使用者 |
| **— Phase 5 —** | | | | |
| 39 | POST | `/api/v1/media/signature` | required | 取得單次上傳簽章（每人每日 50 次） |
| 40 | POST | `/api/v1/media` | required | 回報上傳完成；後端反查驗證後建立 media |
| — | GET | `/api/v1/ws` | required | WebSocket 握手（非 REST，見第 20 章） |

> **端點總計：40 個 REST 端點 + 1 個 WebSocket 端點。**
> Phase 1 有 15 個、Phase 2 有 11 個（9 個必要 + 2 個選做）、Phase 3 有 6 個、Phase 4 有 6 個、Phase 5 有 2 個。
> （P3-0 原本規劃 Phase 3 為 5 個端點，P3-4 實作時追加了 `GET /conversations/unread-count`——
> header 的未讀紅點需要一個輕量端點，不該為了一個數字扛整個對話列表的成本。
> Phase 4 的 `GET /notifications/unread-count` 同理。）
>
> **所有 `/conversations/*` 端點對非參與者一律回 404**（不回 403，見 21.3）。
>
> **Phase 4 對既有端點無任何行為變更。**
>
> **Phase 5 對既有端點的擴充（皆為非破壞性）**：`PATCH /me` 新增 `avatar_media_id`（6.1.2）；`POST /posts`、`PATCH /posts/{id}` 新增 `media_ids`（26.2）；Post Detail（#12 以及 #9、#13 的回應）新增 `media`（26.3）；摘要的產生規則變更（26.4）。

**Phase 2 對既有端點的認證變更**：`GET /posts`、`GET /users/{u}/posts`、`GET /users/{u}` 由純公開改為 **optional auth**（回應新增個人化欄位）。`GET /posts/{id}/comments` **刻意維持純公開**——回應不含個人化欄位，這不是漏改。

---

---

# Phase 2 章節

## 14. Likes API

### 14.1 `PUT /api/v1/posts/{id}/like` 與 `DELETE /api/v1/posts/{id}/like`

【決策】採 `PUT` / `DELETE` 而非 `POST` 切換式設計。切換式 API **不是冪等的**——網路重試或使用者連點兩下會得到相反狀態，且客戶端無法從回應判斷最終狀態。

| Method | 語意 | 冪等性 |
|---|---|---|
| `PUT` | 「讓這篇文章處於已按讚狀態」 | ✅ 重複執行仍是已按讚 |
| `DELETE` | 「讓這篇文章處於未按讚狀態」 | ✅ 重複執行仍是未按讚 |

- **認證**：required
- **實作**：`PUT` → `INSERT ... ON CONFLICT DO NOTHING`；`DELETE` → 直接 `DELETE`
- **回應**：**兩者皆 `200`**

```json
{ "data": { "like_count": 12, "liked_by_me": true } }
```

- ⚠️ **`DELETE` 回 `200` + body 而非 `204`**：前端需要更新後的計數。這是與 `DELETE /posts/{id}` 的刻意差異——那裡沒有有用的資料可回，這裡有。
- 已按讚時再 `PUT` → 仍 `200`（不是 409）；未按讚時再 `DELETE` → 仍 `200`（不是 404）。目標狀態已達成即為成功。
- **錯誤**：文章不存在 / 已刪 / **為草稿** → `404`（見 18.1）；未登入 → `401`

---

## 15. Comments API

### 15.1 端點一覽

| Method | Path | 認證 |
|---|---|---|
| `GET` | `/api/v1/posts/{id}/comments` | **公開** |
| `POST` | `/api/v1/posts/{id}/comments` | required |
| `PATCH` | `/api/v1/comments/{id}` | required（留言作者） |
| `DELETE` | `/api/v1/comments/{id}` | required（留言作者**或**文章作者） |

留言以**文章 id** 而非 slug 定位（與寫入端點的慣例一致；留言區在文章載入後才呼叫，前端此時已有 id）。

### 15.2 `content` 驗證【決策】

1. 先 `TrimSpace`，trim 後為空字串 → `400 VALIDATION_ERROR`
2. 長度上限 **1000 個 Unicode rune**

⚠️ **實作必須用 `utf8.RuneCountInString`**，不可用 `len(string)`，也不可用 Gin binding 的 `max` tag——【事實】兩者算的都是 byte，一個中文字 3 bytes，中文使用者會在 333 字就被擋下。

### 15.3 排序與分頁

`ORDER BY created_at ASC, id ASC`（留言正序閱讀較自然；tie-break 規則同 9.2）。分頁規則同 9.1。

### 15.4 權限規則【決策】

| 操作 | 誰可以 |
|---|---|
| **編輯** | **只有留言作者** |
| **刪除** | 留言作者 **或** 文章作者 |

這個不對稱是刻意的：文章作者有「移除不當內容」的正當需求，但沒有「修改他人發言」的正當性。非授權者 → `403 FORBIDDEN`。

### 15.5 其他規則

- 刪除為 **soft delete**（設 `deleted_at`），資料列保留
- 已刪留言**不出現在列表中**，不做「此留言已刪除」的佔位顯示
- 留言列表必須 JOIN `users` 並排除已軟刪的留言者（見 18.4）
- 回應形狀見 11.4

---

## 16. Follows API

### 16.1 `PUT` / `DELETE /api/v1/users/{username}/follow`

語意與冪等性同 14.1。

- **認證**：required
- **回應**：`200`

```json
{ "data": { "follower_count": 42, "followed_by_me": true } }
```

- **錯誤**：
  - **追蹤自己** → `400 VALIDATION_ERROR`（`details.fields.username`）
  - 對象不存在 / 已軟刪 → `404`
  - 未登入 → `401`

【決策 / 雙層防護】不能追蹤自己由兩層保證：資料庫的 `CHECK (follower_id <> followee_id)` 是無法繞過的最終防線；應用層先行檢查以提供可讀的錯誤訊息。原則同 slug 唯一性（DB 是權威，應用層負責體驗）。

> 取消追蹤自己不是錯誤，只是無事發生，不需特別處理。

### 16.2 關注列表端點

| Method | Path | 說明 |
|---|---|---|
| `GET` | `/api/v1/users/{username}/followers` | 追蹤此人的使用者 |
| `GET` | `/api/v1/users/{username}/following` | 此人追蹤的使用者 |

- **認證**：公開
- 回應為 FollowUser 陣列（11.5）+ `pagination`
- 排序：`follows.created_at DESC, users.id DESC`
- 兩者皆排除已軟刪的使用者

⚠️ **方向定義**（`follows` 是自關聯多對多，方向有意義）：

| 想知道 | 查詢 |
|---|---|
| A 追蹤了誰（following） | `WHERE follower_id = A`，取 `followee_id` |
| 誰追蹤 A（followers） | `WHERE followee_id = A`，取 `follower_id` |

---

## 17. Feed API

### 17.1 `GET /api/v1/feed`

- **認證**：required
- **內容**：**我追蹤的人 + 我自己**的已發布文章
- **排序**：`published_at DESC, id DESC`
- **分頁**：同 9.1
- **回應**：Post Summary 陣列 + `pagination`

包含自己的文章是刻意的——發文後在 feed 看不到自己的內容很違反直覺，且實作上只是多一個 `OR` 條件。

### 17.2 實作方式：pull 模型【決策】

**不建 `feeds` 表**。Feed 是即時查詢：

```sql
WHERE p.author_id = $viewer
   OR p.author_id IN (SELECT followee_id FROM follows WHERE follower_id = $viewer)
```

push 模型（發文時扇出寫入每個追蹤者的 feed 表）適合追蹤者上萬的規模，在本專案的規模下純粹是複雜度。【事實】此決策**可逆**：日後真的需要，加一張 feed 表並改寫查詢即可，**API 合約完全不用動**。

### 17.3 空狀態【決策】

沒有追蹤任何人（且自己也沒有已發布文章）時 → **`200` + 空陣列**。

**不做「改回傳全站文章」的 fallback**，理由：同一端點在不同狀態下回傳語意完全不同的資料，而回應中**沒有欄位能讓前端分辨**。結果是前端無法標示「這是推薦內容，不是你的追蹤」，使用者會誤以為追蹤的人發了這些文。

**空狀態的 UI 是前端的責任**：顯示提示 + 導向首頁的探索連結。探索直接用既有的 `GET /posts`，不需要新端點。

【未來】若要做真正的推薦（熱門、相似興趣），另開 `GET /explore`，語意獨立、排序邏輯可自由設計。

---

## 18. Phase 2 的橫向規則

### 18.1 草稿的可見性【決策】

草稿（`status = 'draft'`）**不可被按讚、不可被留言**。三個端點行為完全一致，**沒有作者例外**：

| 操作 | 對象是草稿時 |
|---|---|
| `PUT` / `DELETE` like | `404 NOT_FOUND`（任何人，含作者本人） |
| `POST` comment | `404 NOT_FOUND`（任何人，含作者本人） |
| `GET` comments | `404 NOT_FOUND`（任何人，含作者本人） |

理由：草稿還不是公開內容，讚數與留言在此沒有意義。回 `404` 而非 `403` 延續既有原則——對非作者不洩漏草稿存在；對作者本人也回 `404` 是為了行為一致（同一端點不因身分而有不同錯誤碼，且少一個分支）。

> 【與文章本身的差異】`GET /users/{u}/posts/{slug}` **作者可以讀自己的草稿**（編輯功能需要它）。差別在於「文章內容對作者有意義」，而「草稿的留言區」對誰都沒有意義。

### 18.2 個人化欄位與 optional auth【決策】

新增的 `liked_by_me` 與 `followed_by_me` 因人而異，使幾個原本純公開的端點改為 **optional auth**：

| 端點 | 認證 | 原因 |
|---|---|---|
| `GET /posts` | **optional** | 回傳 `liked_by_me` |
| `GET /users/{u}/posts` | **optional** | 回傳 `liked_by_me` |
| `GET /users/{u}/posts/{slug}` | optional（原本即是） | draft 可見性 + `liked_by_me` |
| `GET /users/{u}` | **optional** | 回傳 `followed_by_me` |
| `GET /posts/{id}/comments` | **純公開** | 回應不含個人化欄位——**刻意維持**，不是漏改 |
| `GET /tags` | 純公開 | 無個人化欄位 |

**降級規則**：未登入、session 無效、**Redis 不可用**時，個人化欄位一律回 `false`，**請求本身仍然成功**（不回 503）。延續既有決策：optional auth 與公開讀取降級為匿名續行。實務效果是 Redis 掛掉時網站仍能完整瀏覽，只是所有讚心顯示為未按讚狀態。

⚠️ **實作陷阱**：`EXISTS` 子查詢在未登入時**必須傳真正的 SQL NULL**（Go 用 `*string` 的 nil 或 `pgtype.UUID{Valid:false}`），**不可傳 `uuid.Nil`**。後者是合法的 UUID 值，結果碰巧正確（沒有使用者的 id 是全零）但語意錯誤——這種錯誤最危險的地方是**它不會壞**，測試全過，只在邊界情況才爆。

### 18.3 計數的聚合方式【決策】

**不新增** `like_count` / `comment_count` 欄位。denormalized 計數是兩份真相，任何遺漏就永久不一致，且需在 transaction 內同步維護。【事實】此決策**可逆且不對稱**：先做即時聚合、日後要加欄位很容易；反之要拆掉已漂掉的計數欄位、修補資料則痛苦得多。

⚠️ **必須用 `LEFT JOIN LATERAL` 各自聚合**，**不可**同時 JOIN 兩張明細表：

```sql
-- ❌ 錯誤：3 個讚 × 4 則留言 = 12 列，兩邊 COUNT 都變成 12
LEFT JOIN post_likes pl ON pl.post_id = p.id
LEFT JOIN comments   c  ON c.post_id  = p.id

-- ✅ 正確：各自聚合，互不放大
LEFT JOIN LATERAL (
    SELECT count(*) AS cnt FROM post_likes pl WHERE pl.post_id = p.id
) lc ON true
LEFT JOIN LATERAL (
    SELECT count(*) AS cnt FROM comments c
    WHERE c.post_id = p.id AND c.deleted_at IS NULL
) cc ON true
```

`COUNT(DISTINCT)` 雖能算對結果，但那 12 列仍被實體化——只是把錯的算對，沒有解決浪費。

**留言數必須帶 `deleted_at IS NULL`**，否則已刪留言仍被計入。

### 18.4 Soft delete 的關聯政策【決策】

【事實】`users` 與 `posts` 是 soft delete，因此 FK 的 `ON DELETE CASCADE` **永遠不會觸發**——刪除一篇文章後，它的 `post_likes` 與 `comments` 仍留在資料庫。

【決策】**不做清除**（不寫清理 job、不在 soft delete 時連帶處理）。這些殘留資料不會被任何查詢看到，且保留它們讓未來的「還原文章」成為可能。

**代價是每支查詢都必須自行排除**：

| 查詢 | 排除規則 |
|---|---|
| **讚數** | **不排除**已軟刪使用者的讚（MVP 簡化，不為一個數字多 JOIN `users`） |
| 留言數 | `comments.deleted_at IS NULL` |
| 留言列表 | 留言未刪 **且** 留言者未軟刪（本來就要 JOIN users 取名字頭像，順手排除） |
| 粉絲 / 追蹤中數量與列表 | 對方 `users.deleted_at IS NULL`（個人頁顯示「追蹤 5 人」但其中 2 個已刪會很怪） |
| Feed 與所有文章列表 | 文章與文章作者皆 `deleted_at IS NULL` |

⚠️ **讚數與追蹤數的處理不一致是刻意的**，寫成明文以免實作各寫各的：讚數只是個數字，語意上不需要排除；追蹤數的語意是「幾個活著的使用者」，必須排除。

【建議】把這些條件封裝在 repository 層，不要讓 handler 或 service 需要記得。

---

---

# Phase 3 章節

## 19. Messaging API

### 19.1 `POST /api/v1/conversations`

建立或取得與某人的一對一對話（find-or-create，冪等）。

- **認證**：required
- **Request**：`{ "username": "bob" }`
- **行為**：
  1. 查出目標使用者（`deleted_at IS NULL`）。不存在 → `404`
  2. 目標是自己 → `400 CANNOT_MESSAGE_SELF`（應用層先擋，不讓 DB 的 CHECK 噴 500）
  3. 兩個 user id 排序後 `INSERT ... ON CONFLICT (user_low_id, user_high_id) DO NOTHING RETURNING`
  4. 無回傳列（代表已存在）→ **以獨立的 statement** 重新查詢
- **回應**：新建 → `201`；既有 → `200`。兩者 body 相同（Conversation shape，見 11.6）

> 【事實 / 實作陷阱】步驟 4 不能寫在同一個 CTE 裡：CTE 內的 SELECT 使用「語句開始時」的快照，看不到另一個交易剛 commit 的那一列，會回傳零筆。必須是**獨立的第二個 statement**，才會取得新的快照（READ COMMITTED）。
>
> 這與決策 #19（signup 的交易外重查）不同：`ON CONFLICT DO NOTHING` 不會讓交易 abort，所以問題不在「交易狀態」而在「快照時機」。

> 【設計說明】前端路由是 `/messages/@{username}`，每次開啟對話頁都會呼叫本端點，因此冪等是必要的而非只是好習慣。這也是本專案**不提供 `GET /conversations/{id}`** 的原因——用 username 就能取回 id 與對方資訊。

### 19.2 `GET /api/v1/conversations`

我的對話列表。

- **認證**：required
- **分頁**：offset（沿用決策 #26）
- **排序**：最後一則訊息時間由新到舊，`id` 作 tie-break
- **規則**：
  - **沒有任何訊息的對話不出現**——實作用 `JOIN LATERAL`（非 `LEFT JOIN`）取最後一則訊息，無訊息者自然被濾掉。這順帶解決了「點了傳訊息但沒真的傳」產生的空對話
  - 對方已軟刪的對話排除（「活著的使用者」語意，同決策 #56）
- **回應**：Conversation shape 陣列（含 `last_message` 與 `unread_count`）+ `pagination`

### 19.3 `GET /api/v1/conversations/unread-count`

header 未讀紅點用的輕量端點。

- **認證**：required
- **回應**：`{ "data": { "unread_count": 7 } }`
- 計算範圍與 19.2 一致（排除對方已軟刪的對話、不計自己傳的訊息）

> 【為什麼獨立成端點】它會在每次載入頁面與標記已讀後被呼叫，不該扛整個對話列表的查詢成本。

### 19.4 `GET /api/v1/conversations/{id}/messages`

歷史訊息，**cursor（keyset）分頁**。詳細規則見 21.1。

- **認證**：required；非參與者 → `404`
- **Query**：

| 參數 | 說明 |
|---|---|
| `limit` | 預設 30，上限 100（超界夾住） |
| `before` | message_id — 載入**比它更舊**的（往上捲） |
| `after` | message_id — 載入**比它更新**的（重連補齊） |

- `before` 與 `after` **互斥**，同時給 → `400 VALIDATION_ERROR`
- 兩者都不給 → 回傳**最新的** `limit` 筆
- 錨點不存在或不屬於本對話 → `400 INVALID_CURSOR`
- **回應**：

```json
{
  "data": [ /* Message 陣列，(created_at, id) 遞增 */ ],
  "has_more": true
}
```

⚠️ cursor 分頁**不回傳 `total` 與 `page`**——那是 offset 分頁的概念。

### 19.5 `POST /api/v1/conversations/{id}/messages`

- **認證**：required；非參與者 → `404`
- **Request**：

```json
{
  "client_message_id": "550e8400-e29b-41d4-a716-446655440000",
  "content": "訊息內容"
}
```

- **驗證**：
  - `client_message_id`：必填，**必須是合法 UUID**，否則 `400`
  - `content`：先 `TrimSpace`，trim 後為空 → `400`；上限 **2000 個 Unicode rune**
    - ⚠️ 實作**必須用 `utf8.RuneCountInString`**（`len()` 與 Gin 的 `max` tag 算的是 byte，中文使用者會在 666 字被擋）
- **冪等**：見 21.2
- **回應**：新建 → `201` + Message；重送 → `200` + 既有的 Message
- **副作用**：新建成功後推送 `message.created` 給兩位參與者（含發送者自己的其他連線）。**推送失敗不影響 HTTP 回應**

### 19.6 `POST /api/v1/conversations/{id}/read`

標記已讀。

- **認證**：required；非參與者 → `404`
- **Request**：`{ "last_read_message_id": "uuid" }`
- **驗證**：錨點必須是合法 UUID 且屬於本對話，否則 `400`
- **行為**：見 21.4（只前進不後退）
- **回應**：`{ "data": { "unread_count": 0 } }`

---

## 20. WebSocket 合約

### 20.1 端點

```
GET /api/v1/ws        （HTTP Upgrade）
```

### 20.2 握手驗證【決策】

**兩道檢查，缺一不可**：

1. **Session**：`auth.Required()` middleware 驗 `sid` cookie。失敗 → `401`
2. **Origin allowlist**：檢查 `Origin` header 是否在 `FRONTEND_ORIGINS` 內。**缺少 Origin 也拒絕**。失敗 → `403`

⚠️ **Origin 檢查是必做，不是加分項**。【事實】WebSocket 握手雖然是 HTTP 請求，但**不受 `SameSite` cookie 保護**——跨站發起的 WS 連線仍會帶上 cookie。若不驗 Origin，任何網站都能用你使用者的身分建立連線（Cross-Site WebSocket Hijacking）。

現有的 CSRF middleware 只擋 POST/PATCH/DELETE，而握手是 GET，會直接放行——**必須在 upgrade 前另外檢查**。

### 20.3 Hub 模型

```
Hub: map[user_id] → set of connections
```

- 一個使用者可有**多條連線**（多分頁、多裝置），推送時發給全部
- Hub 存在 **Go 程序記憶體**中——這是本專案唯一「必須在記憶體中」的狀態
- 【限制，明文記錄】**僅支援單一 instance**。多 instance 時 A 連在 server1、B 連在 server2 會無法互通，需要 Redis Pub/Sub。Render 免費方案是單一 instance，符合此限制

**連線管理的必要防護**（不做會壞）：

| 項目 | 理由 |
|---|---|
| 單一 writer goroutine | gorilla/websocket 併發寫會 **panic**，拖垮整個程序 |
| bounded send channel + 慢客戶端斷線 | 否則 goroutine 與記憶體無限累積 |
| read deadline + pong handler | 否則死連線永遠不回收（手機切背景、拔網路都不觸發 close） |
| `SetReadLimit` | 擋掉超大 frame 造成的 OOM |
| goroutine panic recovery | 一條連線的 panic 不能拖垮整個服務 |

⚠️ 【事實 / Go channel 規則】**有多個 sender 時，receiver 不可關閉 channel**。`send` channel 的寫入者是任何推送訊息的 goroutine，而關閉是從讀取端觸發的——若去 `close(send)`，往已關閉 channel 寫入會**直接 panic 且無法用 select 或 recover 避開**，且 panic 發生在 HTTP handler 的 goroutine（那裡沒有 recover）。正確做法是另開一個 `done` channel 當結束訊號，`send` 永遠不關閉、交給 GC。

### 20.4 心跳【決策】

【事實】**瀏覽器的 WebSocket API 沒有暴露 ping 方法**——ping/pong 是協定層由瀏覽器自動處理的。因此「前端定時送心跳」只能靠自訂 JSON 訊息模擬，那是多餘的。

| 角色 | 行為 |
|---|---|
| Server | 每 54 秒送 **ping frame**（`WriteControl`），須明顯小於 `pongWait`(60s) |
| 瀏覽器 | **自動回 pong**（不需要前端程式碼） |
| Server | pong handler 延長 **read deadline** |
| 前端 | **不需要心跳**，靠 `onclose` / `onerror` 觸發重連 |

### 20.5 連線期間的 session 重驗【決策】

握手時驗一次 `sid`，之後這條連線就永遠有效——使用者登出、session 過期、在別裝置撤銷，連線都還活著。

**做法**：連線記住 `sid`，**每次收到 pong 時重驗一次**（Redis GET，須帶 **3 秒 timeout**）。

⚠️ timeout 是必須的：重驗跑在 readPump 的 goroutine 裡且**不在 `ReadMessage` 中**，read deadline 管不到它。沒有 timeout 的話 Redis 一旦 hang 住，那條連線的 goroutine 就永久洩漏且無任何錯誤訊息。

**兩種失敗要區分**：

| 情況 | 處理 | 理由 |
|---|---|---|
| Redis 回報**查無此 session** | **關閉連線** | 憑證確實已失效 |
| **Redis 連不上或逾時** | **不關閉** | 見下 |

⚠️ 這看似與決策 #15（required auth 在 Redis 掛掉時 fail closed 回 503）衝突，但**語意不同**：那條是「不知道你是誰就不給進門」；這裡是「**已經在門內的人，不因為門房打瞌睡而被趕出去**」。使用者已通過握手驗證，暫時無法確認不等於憑證失效。

**次要優化**：`POST /auth/logout` 額外通知 Hub 關閉**同 `sid`** 的連線（不是同 user 的全部連線——別把其他裝置踢掉）。這是**加速撤銷**而非安全保障，兜底是上述的 pong 重驗；必須冪等，找不到連線時不報錯、不影響 logout 的回應。

### 20.6 Event envelope

**Server → Client**：

```json
{
  "type": "message.created",
  "data": { /* Message shape，同 11.7 */ }
}
```

Phase 3 只有 `message.created` 一種事件。`type` 欄位為未來擴充預留。

**Phase 4 新增 `notification.created`**（見 22.9）——當初把 envelope 的 `type` 欄位先定好，這次擴充就不需要改協定，前後端各加一個訂閱分支即可。

⚠️ **`data` 的形狀必須與 HTTP 回應完全相同**。實作上讓同一個序列化函式服務兩個出口，否則兩份實作遲早漂移。

**Client → Server**：**無**。

Phase 3 的 WebSocket 是**單向推送**。所有寫入都走 HTTP，前端不送任何 WS 訊息。伺服器讀取只為偵測斷線與觸發 pong handler，**讀到的內容一律丟棄**。

> 【設計效益】這讓 WS 的攻擊面極小——server 不解析任何客戶端傳來的 WS 訊息，只需 `SetReadLimit` 防止異常大的 frame。

### 20.7 送達保證與重連【決策】

**WS 推送不保證送達。PostgreSQL 是唯一權威。**

**重連流程**：

1. 連線斷開 → 前端**指數退避 + ±20% jitter** 重試（1s → 2s → 4s → 8s → 16s，上限約 30 秒，實際落在 24~36 秒）
2. 重連成功 → 對開啟中的對話呼叫 `GET /messages?after=<本地最後一則的 id>`
3. **迴圈抓取直到 `has_more: false`**（見 21.1）

jitter 的理由：伺服器重啟時所有連線同時斷開，沒有隨機化的話大家會在同一秒重連，撞上還在冷啟動的服務。

> 【實作提醒】前端的重連需要**世代編號**：舊 socket 的 `onclose` 可能在新連線建立之後才觸發，若不檢查世代就會把新連線的 handle 設成 null——store 失去對活著連線的參照、狀態顯示錯誤、甚至開出第二條連線。有了世代編號就不需要「是否主動關閉」的旗標（兩套機制並存只會製造混淆）。

---

## 21. Phase 3 的橫向規則

### 21.1 messages 用 cursor 分頁【MVP 例外，決策】

> **明文例外**：專案原則是 offset 分頁（決策 #26），**`GET /conversations/{id}/messages` 是唯一例外**，其餘端點（含對話列表）不變。

**為什麼必須例外**：這**不是效能問題，是正確性問題**。使用者往上捲載入 `offset=20` 的那一刻，對方傳了 3 則新訊息進來，整個序列往後推 3 格 → 會**重複拿到**已顯示過的訊息，或（反向排序時）**漏掉** 3 則。這一定會發生。

**cursor 內容**：使用 `message_id`，由伺服器解析成 `(created_at, id)` 做複合比較：

```sql
-- before：往上捲
WHERE m.conversation_id = $1
  AND (m.created_at, m.id) < (
        SELECT created_at, id FROM messages
        WHERE id = $2 AND conversation_id = $1   -- 同時驗證錨點屬於本對話
      )
ORDER BY m.created_at DESC, m.id DESC
LIMIT $3 + 1
```

用 message_id 而非編碼字串的理由：前端零額外邏輯（手上本來就有 id）、debug 時貼個 UUID 就能重現。tie-break 完整存在，不會因同一毫秒的多則訊息失效。唯一風險（錨點被刪）在 Phase 3 不存在（不做訊息刪除）。

**回應順序【寫死】**：`before` 天生是 DESC 查詢、`after` 天生是 ASC。若原樣回傳，前端會收到兩種順序。**一律以 `(created_at, id)` 遞增排序回傳**（最舊在前，即畫面由上而下）；`before` 模式在應用層反轉後才回傳。

**`has_more` 分模式定義**：用 `LIMIT n+1` 抓、判斷後丟掉多的那筆。

| 模式 | 意義 | 前端行為 |
|---|---|---|
| `before` | 還有**更舊**的 | 使用者繼續往上捲時再抓 |
| `after` | 還有**更新**的 | **必須繼續迴圈抓取直到 `false`** |

⚠️ **`after` 模式的迴圈是必須的，不是最佳化**。離線一週後回來，`after=<最後已知訊息>` 一次只回 30 筆。若前端假設一次呼叫就補齊，**中間的訊息會永遠遺失**——而且不會有任何錯誤訊息，使用者只會發現對話莫名缺了一段。

### 21.2 訊息的冪等策略【決策】

`client_message_id`（前端產生的 UUID）+ `UNIQUE(sender_id, client_message_id)`。

| 情況 | 回應 |
|---|---|
| 新建成功 | `201` + Message |
| 撞到 unique constraint（23505） | 查出既有訊息 → **`200`** + 該訊息 |

前端重試時**沿用同一個 `client_message_id`**，因此就算前一次其實已寫入成功，也不會產生第二筆。

> 【與決策 #19 的異同】同樣是「23505 後收斂到既有資料」，但 messages 的 INSERT 沒有開啟顯式交易，單一 statement 失敗後連線即可繼續使用，不存在「aborted transaction」的問題。

`client_message_id` 也是**樂觀更新的比對鍵**：前端送出時先以它顯示一則 pending 訊息，收到 201 或 WS 回音時據以替換。

**前端去重規則【寫死】**：收到 `message.created` 時
1. `id` 已存在 → **忽略**（送出的那個分頁已經有了）
2. `client_message_id` 對應到某則 pending → **替換**
3. 否則 → 新增

### 21.3 非參與者一律 404【決策】

所有 `/conversations/{id}/*` 端點的第一步都是驗證 `$me IN (user_low_id, user_high_id)`，**失敗回 `404` 而非 `403`**。

延續決策 #16 與 #43 的同一條原則：**不洩漏資源的存在性**。回 403 等於承認「這個對話存在，只是你不在裡面」。Phase 3 不開特例。

### 21.4 已讀位置只前進不後退【決策】

**這是一定會出事的地方**：使用者開兩個分頁，A 滑到最新標記已讀、B 還停在舊位置也送出標記——順序一亂，未讀數就會**憑空長回**，而後端沒有任何錯誤。

標記已讀的 upsert 必須帶條件：

```sql
INSERT INTO conversation_reads
       (conversation_id, user_id, last_read_message_id, last_read_at, updated_at)
SELECT $1, $2, m.id, m.created_at, now()
FROM   messages m
WHERE  m.id = $3 AND m.conversation_id = $1
ON CONFLICT (conversation_id, user_id) DO UPDATE
SET    last_read_message_id = EXCLUDED.last_read_message_id,
       last_read_at         = EXCLUDED.last_read_at,
       updated_at           = now()
WHERE  EXCLUDED.last_read_at > conversation_reads.last_read_at;
```

兩個重點：
1. **最後一行的 `WHERE`**：舊的標記不會覆蓋新的
2. **`SELECT ... FROM messages WHERE m.conversation_id = $1`**：把「這則訊息真的屬於這個對話」擋在資料庫層

⚠️ `RowsAffected` 為 0 有兩種可能——錨點不合法（該報錯）、或送了較舊的位置（正常跳過）。**兩者無法從結果區分**，所以錨點驗證必須在應用層另外做。

**前端行為**：只在**頁面可見**時標記已讀（`document.hidden` 為 false）。分頁在背景時使用者其實沒看到訊息，標記已讀會讓他錯過通知；切回前景時由 `visibilitychange` 觸發補標記。

### 21.5 未讀數的 NULL 陷阱

【事實】從未讀過的對話**沒有** `conversation_reads` 那一列，`LEFT JOIN` 出來是 NULL，而 **`m.created_at > NULL` 的結果是 NULL 不是 true**——未讀數會全部算成 0。

必須用 `COALESCE(r.last_read_at, '-infinity'::timestamptz)`：任何時間都大於 `-infinity`，所以「從未讀過」正確地等於「全部他人訊息皆未讀」。

未讀數**不計自己傳的訊息**（`sender_id <> $viewer`）。

---

---

# Phase 4 章節

## 22. Notifications API

### 22.1 端點一覽

| Method | Path | 認證 |
|---|---|---|
| `GET` | `/api/v1/notifications` | required |
| `GET` | `/api/v1/notifications/unread-count` | required |
| `POST` | `/api/v1/notifications/read` | required |
| `POST` | `/api/v1/notifications/read-all` | required |

### 22.2 通知的產生規則【決策】

| 事件 | `type` | recipient | `entity_id` |
|---|---|---|---|
| 按讚文章 | `like` | 文章作者 | **post_id** |
| 留言文章 | `comment` | 文章作者 | **comment_id** |
| 追蹤使用者 | `follow` | 被追蹤者 | **NULL** |

**刻意不做的兩種**：

- **私訊不進通知系統**——已有 header 未讀紅點，再產生通知是重複打擾，且會讓通知列表被「XXX 傳訊息給你」淹沒
- **新文章不通知追蹤者**——那是內容推播（動態牆的職責），且一篇文章可能一次產生上百筆通知（扇出寫入），是唯一有規模風險的路徑

**不通知自己**：`if actorID == recipientID { return }`。應用層先擋以省一次 DB 往返，DB 的 `CHECK (recipient_id <> actor_id)` 是最終防線。

### 22.3 產生時機與交易邊界【決策】

**順序**：主操作 commit → 寫 notification → 確認有實際插入 → 才推 WebSocket。

通知是**次要副作用**：寫入失敗只記 log，**不回滾主操作、不回 500**。按讚成功但通知沒產生，使用者的損失是「少收到一則通知」；反過來讓通知失敗導致按讚失敗，是本末倒置。

三個必須遵守的實作規則：

**(a) ⚠️ 不可沿用 request context**

主操作 commit 後，client 可能已斷線（使用者關頁面、冷啟動逾時），此時 `c.Request.Context()` 已被 cancel，通知的 INSERT 會**直接失敗且靜默**——變成「按讚成功但通知永久遺失」。

```go
nctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
defer cancel()
```

（`context.WithoutCancel` 為 Go 1.21 引入。3 秒沿用 WebSocket pong 重驗 session 的同一數值。）

**(b) ⚠️ 同步呼叫，不可包在 goroutine 裡**

看到「best-effort、失敗只記 log」很容易寫成 `go notify(...)`，但那有兩個問題：goroutine 裡的 panic 會炸掉整個程序（gin 的 recovery middleware 管不到別的 goroutine）；且沒有 graceful shutdown 的等待機制，服務重啟時飛在半空的 goroutine 直接消失。

代價只是回應多等一次單筆 INSERT。用 `WithoutCancel` 的整個意義就是「主操作已 commit、回應已注定成功」，這時多等幾毫秒零風險。

**(c) 不需要額外 SELECT**

`comment` service 的 `FindCommentablePost` 本來就回傳 `post.author_id`；`follow` 的 recipient 就是 followee id；`like` 的 `FindLikeablePost` 改為一併回傳 `author_id` 即可。每則通知只多一次 INSERT。

### 22.4 去重與「幽靈紅點」【決策，本章最重要的一條】

**去重**：`UNIQUE(recipient_id, actor_id, type, entity_id) NULLS NOT DISTINCT` + `ON CONFLICT DO NOTHING`。

沒有去重的話，惡意使用者可以連點按讚洗版對方的通知欄。

⚠️ **去重讓「主操作成功」與「有新通知」脫鉤**，因此：

> **`ON CONFLICT DO NOTHING` 影響 0 列時，絕對不能推送 WebSocket 事件。**

若照推，「取消讚再按讚」會推出一個沒有對應通知的事件：前端紅點 +1，重新抓列表卻發現什麼都沒變 → **幽靈紅點，而且清不掉**（沒有那筆通知可以標為已讀）。

【事實】「DB 操作沒回錯就代表成功」在這裡不成立——`ON CONFLICT DO NOTHING` 影響 0 列時，它是「成功地什麼都沒做」。

實作：

```sql
INSERT INTO notifications (recipient_id, actor_id, type, entity_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING
RETURNING id, created_at
```

`pgx.ErrNoRows` → **直接 return，不推 WebSocket**。

用**不指名**的 `ON CONFLICT DO NOTHING`（不寫欄位清單）：此表只有一個 unique index，沒有歧義，且避開 `NULLS NOT DISTINCT` 索引的 inference 邊界。

**去重的三個已知代價**（明文記錄）：

| 代價 | 說明 |
|---|---|
| 取消讚後再按，不會產生新通知 | 舊通知還在。實務上可接受——對方確實已被通知過 |
| 去重的通知保留**首次的 `created_at`** | 重複操作不會讓它回到列表頂端 |
| 若舊通知已被讀過，重複操作不會讓它變回未讀 | 同上 |

### 22.5 `GET /api/v1/notifications`

- **分頁**：offset
- **排序**：`created_at DESC, id DESC`
- **不做任何過濾**（見 24.1）
- **不會自動標記已讀**——標記是獨立端點
- **回應**：Notification 陣列（11.8）+ `pagination`

⚠️ **列表必須用 `LEFT JOIN` 取 actor**——`INNER JOIN` 會讓「actor 已軟刪」的通知整筆消失，導致與 `unread-count` 漂移。

**polymorphic target 的取法**：`LEFT JOIN LATERAL` + `UNION ALL`，兩個分支都帶 `n.type` 守衛，因此最多只有一個分支產生列；`follow` 兩邊都不符 → 全 NULL → `target: null`。

### 22.6 `GET /api/v1/notifications/unread-count`

```json
{ "data": { "unread_count": 7 } }
```

SQL 是**一句不 JOIN 任何表的 COUNT**：

```sql
SELECT count(*) FROM notifications WHERE recipient_id = $1 AND read_at IS NULL
```

【為什麼獨立成端點】它會在每次載入頁面、WebSocket 重連、頁面重新取得焦點時被呼叫，不該扛整個通知列表的查詢成本。

### 22.7 `POST /api/v1/notifications/read`

**Request**：`{ "ids": ["uuid", ...] }`

- `ids`：必填，**1–100 筆**；空陣列 → `400`
- 每個 id 必須是合法 UUID，否則 `400`（在應用層驗證，不讓非法字串進 SQL）

```sql
UPDATE notifications SET read_at = now()
WHERE recipient_id = $1 AND id = ANY($2::uuid[]) AND read_at IS NULL
```

三個要點：

1. **`recipient_id = $1` 是 IDOR 防護**——不能標記別人的通知
2. **他人的 id 或不存在的 id 靜默忽略、不回 404**——回 404 等於洩漏「這筆通知存在」
3. **尾巴的 `read_at IS NULL` 已保證冪等**，不需要 `COALESCE`

**回應**：`{ "data": { "updated": 2, "unread_count": 5 } }`

`updated` 是實際變更的列數（重複標記同一筆時為 0），`unread_count` 讓前端少一次往返。

⚠️ **`unread_count` 必須用第二句獨立的 SQL 查詢，不可用 data-modifying CTE 一次算完**：

```sql
-- ❌ 這樣會回傳「更新前」的未讀數
WITH upd AS (UPDATE notifications SET read_at = now() WHERE ... RETURNING 1)
SELECT (SELECT count(*) FROM upd),
       (SELECT count(*) FROM notifications WHERE recipient_id = $1 AND read_at IS NULL)
```

【事實】PostgreSQL 中 `WITH` 裡的 data-modifying statement 與主查詢**共用同一個 snapshot**，主查詢看不到 CTE 的寫入效果。前端拿到舊數字後把紅點設成那個值，等於完全沒清——而這種 bug 在後端手測時很難發現（數字確實有變小），要到前端接起來才會看出「紅點永遠差幾格」。

兩句之間的 race 已被 24.3 涵蓋，不需要交易。

### 22.8 `POST /api/v1/notifications/read-all`

**無 body。**

```sql
UPDATE notifications SET read_at = now() WHERE recipient_id = $1 AND read_at IS NULL
```

**回應**：`{ "data": { "updated": 7, "unread_count": 0 } }`

【簡化】`unread_count` 依定義就是 0，直接回 0，省一次查詢。

⚠️ 兩個實作注意事項：

1. **不可呼叫嚴格 body 解析（`BindStrict`）**——它對空 body 會回「request body is empty」的 400
2. CSRF middleware **不會擋**：它是 `if ContentLength != 0` 才檢查 Content-Type（同 `PUT /posts/{id}/like` 的既有慣例）

### 22.9 WebSocket 事件與前端更新協定【決策】

| 項目 | 決定 |
|---|---|
| **事件型別** | `notification.created`（envelope 格式同 20.6） |
| **推送對象** | **只推給 recipient** |
| **payload** | **完整通知物件**，與 `GET /notifications` 單筆完全同形狀（用同一個序列化函式產生，避免兩份實作漂移） |
| **紅點** | 收到事件 → **本地 +1**（樂觀），由重連 / focus 的重抓校正 |

⚠️ **不可混用**「收到事件就重抓 unread-count」與「本地 +1」——那會 double count。

**已知的多分頁漂移**（明文記錄）：A 分頁按 `read-all` 後，B 分頁的紅點會停在舊值直到重新取得焦點。屬既有「多裝置 session 管理」缺口，可接受。

### 22.10 通知的重連補齊【決策】

WebSocket 不保證送達，而服務休眠期間產生的通知不會被推送。因此需要與訊息的 `?after=` 等價的補齊機制：

| 時機 | 動作 |
|---|---|
| WebSocket **每次**（重）連線成功 | 重抓 `GET /notifications/unread-count` |
| 通知面板正開著時的重連 | 連列表一起重抓 |
| 頁面重新取得焦點 | 重抓 unread-count |

通知的補齊比訊息簡單得多——只是一個數字，不需要 cursor 迴圈。

---

## 23. Search API

### 23.1 端點一覽

| Method | Path | 認證 |
|---|---|---|
| `GET` | `/api/v1/search/posts` | **optional** |
| `GET` | `/api/v1/search/users` | 純公開 |

`/search/posts` 是 optional auth，因為回應含 `liked_by_me`。⚠️ 未登入時 viewer 必須傳**真正的 SQL NULL**（同 18.2 的規則）。

`/search/users` 純公開：回應不含任何個人化欄位（見 11.9）。

### 23.2 查詢字串處理【決策】

```
1. trim
2. rune 計數 2–100（不是 byte —— 一個中文字 3 bytes）
3. escape `\` `%` `_`（反斜線先處理）
4. pattern 在應用層組好（"%" + escaped + "%"），以單一參數傳入
5. SQL 使用 ILIKE $n ESCAPE ''
```

⚠️ **必須保留兩個版本的查詢字串**：

| 用途 | 版本 |
|---|---|
| `ILIKE` 命中 | **已 escape** 並加上 `%` 的 pattern |
| `word_similarity` 排序 | **未 escape** 的原始 query |

餵 escape 後的版本給 `word_similarity` 會讓使用者搜「50%」變成對「50\%」算 trigram，排序失準。

**不 escape 的後果**：使用者搜 `%` 會匹配全部文章、搜 `_` 會匹配任一字元。這不是安全漏洞（參數化查詢仍防 SQL injection），但是明顯的行為錯誤。

> ⚠️ **測試 escape 時必須用 ≥2 字元的值**（例如 `50%`、`a%b`）。`%`、`_`、`\` 都是單一字元，會先被長度檢查擋成 400，escape 邏輯根本不會被執行到。

### 23.3 `GET /api/v1/search/posts`

- **Query**：`q`（必填）、`page`、`limit`
- **範圍**：`status='published'`、`posts.deleted_at IS NULL`、作者 `deleted_at IS NULL`
- **命中判斷**：

```sql
(p.title ILIKE $pat ESCAPE '' OR p.excerpt ILIKE $pat ESCAPE '')
```

⚠️ **不可寫 `COALESCE(p.excerpt, '')`**，兩個理由：

1. **不需要**——`false OR NULL = NULL → 排除`，而 NULL excerpt 本來就不該匹配任何東西，排除正是想要的語意。（`created_at > NULL` 那類 NULL 陷阱的情境是「NULL 讓**該出現**的列消失」，這裡相反。）
2. **更嚴重**——GIN 索引建在**裸欄位** `excerpt` 上，查詢表達式若是 `COALESCE(excerpt,'')`，planner 無法匹配，索引永遠用不到。等於花儲存空間建了一個結構上不可能被使用的索引。

索引的 partial 條件 `AND excerpt IS NOT NULL` **保留**：`ILIKE` 是 strict operator，PostgreSQL 的 predicate proof 能推出 `IS NOT NULL`，裸欄位寫法對得上。

- **排序**：

```sql
ORDER BY (p.title ILIKE $pat ESCAPE '') DESC,   -- 標題命中的排前面
         word_similarity($q, p.title) DESC,
         p.published_at DESC, p.id DESC           -- tie-break
```

- **回應**：Post Summary 陣列 + `pagination`（沿用既有形狀，前端可重用文章卡片元件）
- **不做關鍵字高亮**；**`q` 與 `tag` 不組合**（明文記錄）

**實作方式**：擴充既有的文章列表查詢而非另寫一支，以繼承 LATERAL 計數、標籤 N+1 解法、viewer NULL 處理與分頁。⚠️ 呼叫時**必須顯式帶 `status='published'`**，不可依賴預設值——同一個查詢也服務「我的文章」（要看草稿）。

### 23.4 排序函式的選擇【決策】

**用 `word_similarity`，不用 `similarity`，也不用 `strict_word_similarity`。**

`similarity()` 以兩邊 trigram 集合的**聯集**做正規化。實測數據：

```
similarity('程式', '我喜歡寫程式')      = 0.1111  (= 1/9)
word_similarity('程式', '我喜歡寫程式') = 0.3333
```

驗算：needle「程式」的 trigram 有 3 個（`"  程"`、`" 程式"`、`"程式 "`），haystack 有 7 個，交集 1，聯集 9 → 1/9 完全吻合。6 個字的 haystack 就已經稀釋到 0.11；200 字的 `excerpt` 分母會大一個數量級以上，分數趨近 0——所以 `GREATEST(similarity(title), similarity(excerpt))` 這類公式看起來對稱、行為完全不對稱（幾乎永遠只取到 title 那一項）。

`word_similarity(needle, haystack)` 取 haystack 中**任意連續片段**的最佳相似度，不被整體長度稀釋。⚠️ **參數順序：短的（query）放第一個。**

**不用 `strict_word_similarity`**：它要求片段對齊**詞邊界**，而中文沒有詞邊界（整句被視為一個 word），會退化成 `similarity`。

**`word_similarity` 的實際行為是「前綴匹配優先 + 短標題優先」**（不是模糊的「貼合度」）。原因是 trigram 的左側 padding：needle 的集合含 `"  程"`、`" 程式"`（前導空白），只有標題開頭命中才對得上。

| 標題 | 對「程式」的分數 |
|---|---|
| 程式 | 1.0 |
| 程式設計入門（開頭命中） | ≈ 0.67 |
| 我今天學了一點程式（結尾命中） | ≈ 0.33 |

### 23.5 `GET /api/v1/search/users`

- **Query**：`q`（同 23.2）、`page`、`limit`
- **範圍**：`users.deleted_at IS NULL`
- **命中**：`(u.username ILIKE $pat ESCAPE '' OR u.display_name ILIKE $pat ESCAPE '')`——同樣**不可**寫 `COALESCE`
- **排序**：`(u.username ILIKE $pat) DESC, word_similarity($q, u.username) DESC, u.created_at DESC, u.id DESC`
- **回應**：SearchUser 陣列（11.9）+ `pagination`

### 23.6 新增錯誤碼

| code | HTTP | 情境 |
|---|---|---|
| `CANNOT_MESSAGE_SELF` | 400 | （Phase 3 已定義） |
| `INVALID_CURSOR` | 400 | （Phase 3 已定義） |

Phase 4 未新增錯誤碼——搜尋的 `q` 不合法走 `VALIDATION_ERROR`，通知的權限問題走既有的 404 / 401。

---

## 24. Phase 4 的橫向規則

### 24.1 「不過濾」原則【決策，結構性理由】

`GET /notifications` 與 `GET /notifications/unread-count` **都不做任何過濾**，條件逐字相同。

| 情況 | 行為 |
|---|---|
| actor 已軟刪 | 通知**仍然出現**，`actor: null`（前端顯示「已刪除的使用者」） |
| target 已刪除 | 通知**仍然出現**，`target: null`（前端顯示「內容已不存在」，不給跳轉連結） |

**為什麼**：若列表過濾掉某些通知而未讀數沒有套同一條件，就會出現**紅點顯示 3、面板只顯示 1，且永遠清不掉**——因為那 2 筆你看不到、也就無法標記已讀。

⚠️ **採「不過濾」的主要理由不是 UX，而是結構**：這讓 `unread-count` 保持為一句**不 JOIN 任何表的 `COUNT(*)`**，**結構上不可能與列表漂移**。若哪天有人在列表加了過濾條件卻忘了未讀數，紅點就會顯示清不掉的數字。

> 【為什麼不能靠慣例推導】專案在這件事上本來就不一致：讚數**不排除**已軟刪使用者（MVP 簡化），而留言列表與追蹤數**必須排除**（語意是「幾個活著的使用者」）。所以通知的處理方式必須明文，不能假設讀者能從既有慣例推出來。

### 24.2 搜尋的效能真相【決策，寫進 README】

**命中判斷由 `ILIKE` 保證，`pg_trgm` 只提供 (a) GIN 索引加速、(b) `word_similarity` 排序函式。**

推論：即使 trigram 對中文表現不如預期，**搜尋功能仍然正確**，只是慢一點、排序笨一點。這讓搜尋的技術風險等級是「低」而非「高」。

⚠️ **2 字元查詢無法使用 GIN trigram 索引**。

【事實】GIN trigram 索引要能支援 `LIKE`/`ILIKE`，pattern 裡必須有**至少 3 個連續字元**可抽出 trigram。`%程式%` 的字面部分只有 2 個字，**抽不出任何 trigram，索引在原理上無法使用**——不是 planner 挑錯、不是 bug。

**下限仍維持 2 字元**：改成 3 是英文本位的誤判，會讓「程式」「台灣」「音樂」這類大量的中文兩字詞完全搜不了。代價是這類查詢一律 Seq Scan。

> 順帶排除一個看似可行的替代方案：改用 `%` 運算子（`title % $q`）雖然能吃索引，但 2 字 needle 的 similarity 遠低於預設門檻 0.3，**會回空結果**。

**README 的誠實描述**：不要寫「用 GIN trigram 索引加速搜尋」——在小資料量下 planner 多半直接 Seq Scan。應寫成「命中由 ILIKE 保證正確性，pg_trgm 提供索引與排序；中文兩字詞抽不出 trigram，2 字查詢無法走索引，這是為了支援常見中文詞而刻意接受的取捨」。

### 24.3 已知的競態與其處理

| 競態 | 處理 |
|---|---|
| `read-all` 執行的瞬間剛好進來一則新通知 → 被一併標成已讀 | **可接受**，明文記錄。毫秒級窗口，後果是使用者少看到一則通知的紅點 |
| 標記已讀與查詢未讀數之間有新通知進來 | 同上，不需要交易 |
| 多分頁：A 標記已讀後 B 的紅點停在舊值 | **可接受**，由 focus 重抓校正（22.9） |

### 24.4 Phase 4 未做的事（明文記錄）

- `DELETE /notifications`（清空全部）——連選做都拿掉
- 通知的即時「已讀狀態同步」到其他分頁
- 搜尋的關鍵字高亮
- `q` 與 `tag` 的組合篩選
- 使用者搜尋的 `followed_by_me`

---

# Phase 5 章節

## 25. Media API

### 25.1 端點一覽與上傳流程

| Method | Path | 認證 | 說明 |
|---|---|---|---|
| POST | `/api/v1/media/signature` | required | 取得單次上傳簽章 |
| POST | `/api/v1/media` | required | 回報上傳完成；後端反查驗證後建立 media |

**上傳流程**（檔案不經過 Go 伺服器，決策 #109）：

```
1. 前端處理檔案
     圖片：Canvas 壓縮（失敗則中止，#123）
     影片：檢查格式、大小、長度（讀不到長度則放行，#167）
2. POST /api/v1/media/signature  { purpose }                 → 簽章（fields）
3. POST https://api.cloudinary.com/v1_1/<cloud>/<type>/upload  → 前端直傳（FormData = fields + file）
4. POST /api/v1/media            { purpose, asset_id, public_id } → 後端以 Admin API 反查驗證 → 建立 media
5. 使用
     頭像：PATCH /me { avatar_media_id }（6.1.2）
     文章：POST / PATCH /posts 的 media_ids（26.2）
```

### 25.2 `POST /api/v1/media/signature`

- **認證**：required
- **Request**：`{ "purpose": "avatar" | "post_image" | "post_video" }`
- **回應 `200`**：

```json
{
  "data": {
    "upload_url": "https://api.cloudinary.com/v1_1/<cloud>/image/upload",
    "resource_type": "image",
    "max_bytes": 5242880,
    "fields": {
      "api_key": "...",
      "asset_folder": "inkwell/<user_id>",
      "overwrite": "false",
      "public_id": "pi_xxxxxxxxxxxxxxxxxxxxxx",
      "signature": "<hex>",
      "timestamp": "1760860000",
      "upload_preset": "inkwell_post_image"
    }
  }
}
```

| 項目 | 規則 |
|---|---|
| `fields` | 前端**原封不動逐一**放進 FormData，再加上 `file`。前端不需要知道有哪些參數 |
| 簽進簽章的參數 | `asset_folder`、`overwrite`、`public_id`、`timestamp`、`upload_preset`。前端改動任何一個，Cloudinary 都會回 Invalid Signature |
| 簽章演算法 | 參數依 key 排序 → `k=v` 以 `&` 串接 → 接上 `api_secret` → SHA-1 hex。`file`、`cloud_name`、`resource_type`、`api_key` 不參與 |
| `api_secret` | 只用於計算簽章，**絕不出現在回應、前端或 git** |
| `public_id` | 後端產生：`{tag}_` + base64url(16 bytes CSPRNG)，tag 為 `av` / `pi` / `pv`（#135） |
| **單次使用** | 同一組 `fields` 再上傳一次，Cloudinary 不建立新檔案，回傳既有檔案並帶 `existing: true`（#135）。這讓前端可以安全地用同一個簽章重試（#149） |
| 有效期 | 自 `timestamp` 起 1 小時（Cloudinary 固定，#136） |
| `max_bytes` | 給前端的即時提示；真正的檢查在 25.3 |

**配額**（決策 #137，本專案第一個 rate limiting）：

| 項目 | 內容 |
|---|---|
| key | `scp:upload_quota:{user_id}:{UTC 日期 YYYY-MM-DD}` |
| 操作 | `MULTI` / `INCR` / `EXPIRE 26h` / `EXEC`——兩個指令同生共死，不會留下永不過期的 key |
| 上限 | 每人每日 50 次。因為簽章是單次的，**等於每日 50 個檔案** |
| 超過 | `429 RATE_LIMITED` |
| Redis 不可用 | 放行並記 log。配額是保護措施而非安全邊界；且此端點為 required auth，Redis 掛掉時本來就過不了認證 |

⚠️ 日期是 UTC：台灣時間早上 8 點前仍算前一天。

**錯誤**：`400`（`purpose` 不合法）、`401`、`415`、`429 RATE_LIMITED`、`503`（Cloudinary 未設定）。

### 25.3 `POST /api/v1/media`

- **認證**：required
- **Request**：`{ "purpose": "post_image", "asset_id": "<32 碼 hex>", "public_id": "pi_xxxxxxxxxxxxxxxxxxxxxx" }`——**只回報三個識別欄位**（#114）
- **回應**：新建 `201`、重送 `200`，body 相同（Media shape，11.10）

**處理順序**：

| # | 步驟 | 失敗時 |
|---|---|---|
| 1 | 格式驗證（不碰外部 API）：`purpose` 合法；`asset_id` 為 32 碼小寫 hex；`public_id` 符合 `^(av\|pi\|pv)_[A-Za-z0-9_-]{22}$` 且 tag 與 `purpose` 一致 | `400`（`details.fields.purpose` / `asset_id` / `public_id`） |
| 2 | **冪等快路徑**（#143）：以 `asset_id` 查 DB。存在且屬於本人 → 直接回 `200`，**不呼叫 Admin API**；屬於他人 → 視同找不到 | — |
| 3 | **Admin API 反查**：`GET /resources/{type}/upload/{public_id}`，**影片加 `?media_metadata=true`**（#171——預設回應沒有 `duration`） | 404 → `400 upload not found`；其他失敗 → `503` |
| 4 | **歸屬驗證**（#134）：`asset_folder` 必須**完全等於** `inkwell/{本人 user_id}`，且 `asset_id`、`public_id` 相符 | `400 upload not found`，**且不刪檔**——否則任何人都能借本站後端刪掉別人的檔案 |
| 5 | **政策驗證**（下表） | **後端刪除該檔**（#140）並回 `400`，`details.fields.upload` = 原因 |
| 6 | 建立 media（`status = 'ready'`）。撞 `media_provider_asset_key`（23505）→ 重查既有列 → 回 `200`（#117） | — |

| `details.fields.upload` | 條件 |
|---|---|
| `unexpected resource type` | `resource_type` 與 purpose 不符 |
| `file format not allowed` | 不在白名單（25.4） |
| `invalid file` | `bytes` ≤ 0 |
| `file too large` | 超過該 purpose 上限。檢查的是**進站轉換之後實際存下的大小**（#139） |
| `video duration unavailable` | 影片缺少長度（fail closed，#138） |
| `video longer than 60 seconds` | 超過 60 秒 |

**刪除被拒絕的檔案**：走 Upload API 的 destroy（不佔 Admin API 額度）、`context.WithoutCancel` + 5 秒 timeout、失敗只記 log。

⚠️ 所有 metadata（`url`、尺寸、大小、長度、格式）**一律採用反查結果**。`url` 與 `thumbnail_url` 由後端依 25.4 的固定變體產生。

**錯誤**：`400`（格式、upload not found、upload rejected）、`401`、`415`、`503`（Cloudinary 未設定或 Admin API 失敗）。

### 25.4 purpose 規格【決策 #119、#122、#172】

| | `avatar` | `post_image` | `post_video` |
|---|---|---|---|
| public_id tag | `av` | `pi` | `pv` |
| upload preset | `inkwell_avatar` | `inkwell_post_image` | `inkwell_post_video` |
| resource_type | image | image | video |
| 大小上限 | 2MB | 5MB | 30MB |
| 長度上限 | — | — | 60 秒 |
| 格式白名單 | jpg、jpeg、png、webp | 同左 | mp4、webm、mov |
| 前端處理 | Canvas → 最長邊 512、JPEG | Canvas → 最長邊 1600、JPEG | 不壓縮；上傳前檢查格式、大小、長度 |
| 顯示 / 播放變體 | `c_fill,g_face,w_512,h_512,f_auto,q_auto` | `c_limit,w_1600,f_auto,q_auto` | `f_mp4,q_auto`（`.mp4`） |
| 封面變體 | — | — | `so_10p,c_limit,w_800,f_jpg,q_auto`（`.jpg`） |

**Upload preset**：三個都是 **Signed**；Overwrite 關閉，另外也把 `overwrite=false` 簽進簽章、不依賴 preset（#142）；**不設 folder**（由簽章帶入 `asset_folder`）。進站轉換：avatar `c_limit,w_1024`、post_image `c_limit,w_2400`、post_video 無。

**格式排除的理由**：SVG 可含主動內容（`<script>`、`onload`），與 DOMPurify 的 XSS 防線衝突；GIF 經 Canvas 壓縮會失去動畫。

### 25.5 Delivery URL【決策 #119、#120、#121】

- 所有 delivery URL 由後端依 purpose 產生並存入 `media.url` / `media.thumbnail_url`，**前端不得自行組合 transformation**
- transformation 在**第一次產生 derived asset 時**計費一次，之後由 CDN 送出只計流量。限制變體的目的是控制 derived asset 的儲存量與可追蹤性
- 不開 HLS / DASH（每個畫質一份 derived asset，額度消耗成倍）
- 圖片以完整網址寫入 `content_md`；**不走自站 302 導向**（每張圖片都會喚醒休眠中的 Render）
- **接受的風險**：Cloudinary 帳號停用或額度耗盡時，全站圖片同時失效；網址格式改版需以 `provider_public_id` 批次重建

---

## 26. 媒體與既有資源的整合

### 26.1 頭像

見 **6.1.2**。

### 26.2 文章媒體：`media_ids`【決策 #112、#128、#157、#159】

適用 `POST /posts` 與 `PATCH /posts/{id}`。

**語意**（同 `tags`）：

| 端點 | 行為 |
|---|---|
| `POST` | 有給就建立關聯；沒給或空陣列 = 沒有媒體 |
| `PATCH` | **有給（含 `[]`）就整組替換**；沒給或 `null` = 不動 |

與 `posts` 的寫入在**同一個 transaction**（#128）。`position` 等於去重後的陣列索引，從 0 開始（#133）。

**處理順序**（#157）：

```
trim → UUID 格式 → 去重（保留第一次出現的位置）→ 總數 ≤ 10
     → 查 DB → 每筆：存在、owner 為本人、status = ready、purpose 為 post_image 或 post_video
     → 影片 ≤ 1
```

上限在**去重之後**才檢查：重複的 id 不回錯。

| `details.fields.media_ids` | 條件 |
|---|---|
| `contains an invalid id` | 不是 UUID |
| `at most 10 media per post` | 去重後超過 10 |
| `contains media that cannot be attached` | 不存在、不是本人的、不是 ready、或是頭像——**刻意不區分**，避免被用來探測別人的 media id 是否存在 |
| `at most 1 video per post` | 影片超過 1 部 |

**後端不 parse Markdown**（#112）：`media_ids` 是「這篇文章擁有哪些媒體」的唯一權威，後端完全不看 `content_md`。圖片的 `post_media` 關聯**只用於生命週期追蹤，不影響渲染**；影片的關聯同時是渲染來源。

**驗證在交易之外**（#159）：P5 的 media 列建立後不會再被修改，唯一的刪除路徑是使用者被刪除時的 CASCADE；萬一發生，`post_media` 的 FK 會讓寫入失敗、整筆回滾。⚠️ 若日後做非同步轉檔（`status` 會變動），須重新評估。

### 26.3 Post Detail 的 `media`【決策 #133、#158】

- 回傳**全部**關聯媒體（含圖片），每筆帶 `purpose`，依 `position` 排序；沒有媒體時是 `[]`，不是 `null`
- `POST /posts` 與 `PATCH /posts/{id}` 的回應同樣帶（皆為 Post Detail shape）
- **Post Summary 不帶**（避免每個列表查詢多一次 JOIN）
- 前端：影片附件從 `media` 過濾 `purpose === 'post_video'`；圖片已在 Markdown 裡，不另外渲染
- 影片由 Vue 直接渲染 `<video>`，**不放寬 DOMPurify**（#110）；`preload="none"` + 封面（#169）

**編輯器的兩條前端規則**（不改變合約，但與合約的「整組替換」語意相關）：

| # | 規則 |
|---|---|
| 161 | 存檔時，網址**已不在內文裡**的圖片不送；影片一律送 |
| 162 | 載入時回應裡**沒有 `media` 陣列**就不送 `media_ids`，並停用新增媒體（#168）——否則會送出 `[]`，把既有關聯靜默清空 |

### 26.4 摘要規則【決策 #160，行為變更】

`excerpt` 產生時：

1. `![說明](網址)` **整段移除**
2. `[文字](網址)` **只留文字**
3. 其餘照舊（移除 `#`、`*`、`_`、`` ` ``、`>`、`[`、`]`，換行轉空白，取前 200 字）

**順序不能反**：圖片語法包含連結語法，先處理連結會留下「!說明」。

**理由**：圖片網址很長（200 字的摘要會被吃掉一半），而且搜尋（第 23 章）索引 `excerpt`——不處理的話，搜「cloudinary」會命中每一篇有圖的文章。

只影響新文章與**之後被編輯**的文章；既有文章的摘要已存在 DB（決策 #6），要到下次編輯才重新產生。

---

## 27. Phase 5 的橫向規則

### 27.1 不信任前端回報的 metadata【決策 #114】

前端只回報 `{purpose, asset_id, public_id}`。沒有反查的話，任何人都能呼叫 `POST /media` 塞任意 `public_id` 與網址進資料庫——包含指向別人的資源、甚至別的網站。

### 27.2 Cloudinary dynamic folder mode【決策 #134】

新帳號一律是 dynamic folder mode：`asset_folder` **不反映在** `public_id` 上，而 `folder` 參數不應使用。所以所有權是以「簽進 `asset_folder` + 反查時比對 `asset_folder`」保證，`public_id` 不承擔任何所有權資訊。

### 27.3 前端處理只是最佳化【決策 #123、#167】

真正的限制由 upload preset 與後端反查執行——惡意客戶端可以完全繞過 Vue，直接呼叫簽章端點與 Cloudinary。

| 情況 | 前端行為 | 理由 |
|---|---|---|
| 圖片壓縮失敗 | **中止，不送原檔** | 放行的話會改送體積大好幾倍的原檔，使用者以為送出的是壓縮版 |
| 影片讀不到長度 | **放行** | 影片本來就送原檔，沒有「更大的檔案」會被送出；後端仍會擋超過 60 秒的影片。且讀不到長度很常見（HEVC 的 `.mov` 在 Windows 的 Chrome 上解不開） |

### 27.4 Admin API 的呼叫紀律【決策 #127、#143、#171】

| 規則 | 理由 |
|---|---|
| timeout 5 秒 | 反查在回報端點的請求路徑上 |
| 失敗時不建立 media 列 | 寧可讓使用者重試，不留未驗證的資料 |
| 不無限重試 | Free 方案有每小時 500 次的上限 |
| **絕不在一般讀取路徑呼叫** | 只在上傳回報時呼叫一次，之後全部用 DB 裡的值 |
| 冪等快路徑先查 DB | 重送不再呼叫 Admin API |
| 影片加 `media_metadata=true` | 預設回應不含 `duration`（P5-4 實測）；仍是同一次呼叫 |

### 27.5 ⚠️ 草稿裡的媒體會洩漏【決策 #124】

決策 #16、#43 一路守著「草稿的存在本身是秘密」（非作者回 404）。但 **Cloudinary 的 delivery URL 是公開的**：草稿裡的圖片或影片，任何拿到網址的人都看得到，不需要登入，也會被 CDN 快取。

| 事實 | 說明 |
|---|---|
| 網址不可猜測 | `public_id` 含 128 bit 的隨機值（#135） |
| 網址不受保護 | 拿到就能看 |

真正的保護需要 signed delivery URL（帶時效簽章），代價是無法長期快取、Markdown 內無法存固定網址（決策 #121 會整個崩掉）。**MVP 接受，記入 README。**

### 27.6 孤兒【決策 #125、#141、#166】

**Phase 5 不自動清理**，只提供列出、不刪除的管理 SQL。

孤兒的來源：上傳後放棄發文、編輯時移除了媒體、換頭像後的舊檔案、**文章被刪除**。

```sql
-- 孤兒：沒有關聯到任何「未刪除」的文章，也不是任何人的頭像
-- （決策 #166：P5-0 的版本漏了 deleted_at，只掛在已刪除文章上的媒體永遠不會被算成孤兒）
SELECT m.id, m.owner_id, m.purpose, m.provider_public_id, m.bytes, m.created_at
FROM media m
WHERE NOT EXISTS (
        SELECT 1 FROM post_media pm
        JOIN posts p ON p.id = pm.post_id
        WHERE pm.media_id = m.id AND p.deleted_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.avatar_media_id = m.id)
  AND m.created_at < now() - interval '24 hours'
ORDER BY m.created_at;
```

⚠️ 這條查詢的驗收**必須雙向測**：造一筆孤兒確認查得到、造一筆已關聯的確認查不到。在沒有孤兒的表上回 0 列，是假通過（決策 #107）。

⚠️ **未來若要自動刪除孤兒，必須先解決 #112 的不同步**：使用者手貼網址（沒走編輯器）的圖片會被誤判為孤兒。目前清理是人工的，誤判只是多看一眼；自動刪除會真的刪掉還在使用的圖片。

**SQL 查不到的孤兒**（#141）：上傳到 Cloudinary 後沒有回報的檔案（例如上傳完就關掉分頁），不會出現在 `media` 表。數量由單次簽章 + 每日配額限制在每人每日 50 個以內；只能在 Media Library 依 `inkwell/` 資料夾人工檢查。

### 27.7 刪除策略

| 情況 | 行為 |
|---|---|
| 文章 soft delete | 不刪 Cloudinary 檔案、不刪 `post_media`（#126，同決策 #52） |
| 換頭像 | 舊的 media 列不刪 |
| `users.avatar_media_id` 指向的 media 被刪 | `ON DELETE SET NULL`（#131）：NULL 本身就是有意義的狀態 |
| 使用者被 hard delete（目前沒有這條路徑） | `media` 與 `post_media` 都 `ON DELETE CASCADE` |

### 27.8 Phase 5 未做的事（明文記錄）

- eager 轉檔與 webhook（#118）。若導入 webhook，決策 #17 的 Origin 白名單會回 403，屆時須新增路由層級例外並改用 Cloudinary 簽章驗證（#132，合約變更預告）
- HLS / DASH（#120）
- 自動清理孤兒（#125）
- signed delivery URL（#124）
- 每篇多部影片
- 文章卡片（Post Summary）顯示影片縮圖
- 圖片的 lazy loading 與 `width` / `height` 屬性（版面跳動）

---

## 11. 附錄 A：對外資料模型（API response shape）

### 11.1 User（公開）／ Me（私有）

- **User（公開，`GET /users/{username}`）**：
  `id`、`username`、`display_name`、`bio`、`avatar_url`、`created_at`
  **+ Phase 2**：`follower_count`、`following_count`、`followed_by_me`
- **Me（私有，`GET /me`）**：公開的**身分欄位** + `email`、`updated_at`
  （不含 Phase 2 的追蹤欄位——`/me` 是身分端點，追蹤數請查 `GET /users/{自己的username}`）
- **Phase 5**：`avatar_media_id` **不出現在任何回應中**（生命週期指標，前端不需要）。`avatar_url` 仍是唯一的顯示欄位（6.1.2）

```json
{
  "id": "uuid",
  "username": "alice",
  "display_name": "Alice",
  "bio": "Hello",
  "avatar_url": "https://...",
  "created_at": "2026-07-01T00:00:00Z",
  "follower_count": 42,
  "following_count": 17,
  "followed_by_me": false
}
```

- `followed_by_me`：未登入 / session 無效 / Redis 不可用時一律 `false`（見 18.2）。

### 11.2 Post Detail

> **Phase 2 擴充**：新增 `like_count`、`comment_count`、`liked_by_me` 三個欄位（非破壞性）。
> **Phase 5 擴充**：新增 `media`（26.3，項目形狀見 11.11）。

```json
{
  "id": "uuid",
  "slug": "my-first-post",
  "title": "My first post",
  "content": "# Hello\n\n...markdown...",
  "excerpt": "純文字摘要…",
  "status": "published",
  "author": { "username": "alice", "display_name": "Alice", "avatar_url": "https://..." },
  "tags": ["go", "backend"],
  "created_at": "2026-07-12T08:00:00Z",
  "updated_at": "2026-07-12T09:00:00Z",
  "published_at": "2026-07-12T08:30:00Z",
  "like_count": 12,
  "comment_count": 3,
  "liked_by_me": false,
  "media": [
    { "id": "uuid", "purpose": "post_image", "url": "https://res.cloudinary.com/...", "thumbnail_url": null, "width": 1600, "height": 1200, "duration": null },
    { "id": "uuid", "purpose": "post_video", "url": "https://res.cloudinary.com/.../f_mp4,q_auto/pv_xxx.mp4", "thumbnail_url": "https://res.cloudinary.com/.../so_10p,c_limit,w_800,f_jpg,q_auto/pv_xxx.jpg", "width": 720, "height": 1280, "duration": 42.5 }
  ]
}
```

（draft 的 `published_at` 為 `null`；`liked_by_me` 在未登入時一律 `false`；沒有媒體時 `media` 為 `[]`。）

### 11.3 Post Summary（列表用，不含 content）

同 Detail 但以 `excerpt` 取代 `content`，且不含 `updated_at`。**同樣包含 Phase 2 的三個計數 / 個人化欄位。** **不含 Phase 5 的 `media`**（避免每個列表查詢多一次 JOIN）。

### 11.4 Comment（Phase 2）

```json
{
  "id": "uuid",
  "content": "留言內容",
  "author": {
    "username": "alice",
    "display_name": "Alice",
    "avatar_url": "https://..."
  },
  "created_at": "2026-08-15T08:00:00Z",
  "updated_at": "2026-08-15T08:05:00Z",
  "edited": true
}
```

- `author` **必含 `username`**：前端以它與 `post.author.username` 比對目前使用者，決定顯示編輯 / 刪除按鈕（`username` 唯一且不可變更，可安全當比對鍵）。因此**不回傳** `can_edit` / `can_delete`，該端點得以維持純公開。
- `edited`：由後端計算（`updated_at > created_at`）。時間比較有微秒精度問題，統一在一處判斷較安全。
- 不回傳 `author.id`（前端用不到，減少不必要的識別碼外露）。

### 11.5 FollowUser（Phase 2 選做端點）

```json
{
  "id": "uuid",
  "username": "bob",
  "display_name": "Bob",
  "avatar_url": "https://...",
  "bio": "Hello"
}
```

- 【決策】`excerpt` 在寫入時產生存入 `posts.excerpt`，讀取零成本；content 更新時重算。
### 11.6 Conversation（Phase 3）

**建立時**（`POST /conversations`）：

```json
{
  "id": "uuid",
  "other_user": {
    "username": "bob",
    "display_name": "Bob",
    "avatar_url": "https://..."
  },
  "created_at": "2026-08-15T08:00:00Z"
}
```

**列表時**（`GET /conversations`）額外含：

```json
{
  "last_message": {
    "id": "uuid",
    "content": "最後一則訊息",
    "is_mine": true,
    "created_at": "2026-08-16T10:00:00Z"
  },
  "unread_count": 3
}
```

- `other_user`：由後端判斷「對方是誰」（比對 `user_low_id` / `user_high_id`），**前端不需要知道 low/high 的存在**
- `last_message.is_mine`：讓前端不必自己比對 username 就能顯示「你：…」前綴
- 列表中的對話**必然有訊息**（無訊息者不出現），故 `last_message` 不會是 null
- `unread_count`：他人傳的、且晚於我的已讀位置的訊息數

### 11.7 Message（Phase 3）

```json
{
  "id": "uuid",
  "conversation_id": "uuid",
  "client_message_id": "uuid",
  "content": "訊息內容",
  "sender": {
    "username": "alice",
    "display_name": "Alice",
    "avatar_url": "https://..."
  },
  "created_at": "2026-08-16T10:00:00Z"
}
```

- 回傳 `client_message_id` 是為了讓前端比對並替換樂觀顯示的 pending 訊息（見 21.2）
- 訊息以**純文字**渲染（`white-space: pre-wrap`），**不走 Markdown**（同決策 #60，少一個 XSS 面）
- ⚠️ **此形狀同時用於 HTTP 回應與 WebSocket 事件的 `data`**，兩者必須完全一致


### 11.8 Notification（Phase 4）

```json
{
  "id": "uuid",
  "type": "like",
  "actor": {
    "username": "bob",
    "display_name": "Bob",
    "avatar_url": "https://..."
  },
  "target": {
    "type": "post",
    "title": "文章標題",
    "url": "/@alice/my-post"
  },
  "is_read": false,
  "created_at": "2026-08-17T10:00:00Z"
}
```

- `type`：`like` / `comment` / `follow`
- `is_read`：**由後端計算**（`read_at IS NOT NULL`），不是資料表欄位（見 12.4）
- `actor` 為 `null` → 觸發者已軟刪，前端顯示「已刪除的使用者」
- `target` 為 `null` → `type=follow`，或相關內容已被刪除（前端顯示「內容已不存在」且不給跳轉連結）
- ⚠️ **`target.type` 恆為 `"post"`**：`target` 表達的是「點下去跳到哪」，而 comment 類型的通知點下去就是那篇文章（`title` 與 `url` 都是文章的）。「這是留言引發的」已由 `notification.type` 表達，`target.type` 再說一次是冗餘且誤導。**前端不需要 `comment` 分支。**
- 留言被軟刪時 `target` 也是 `null`，即使文章還在——這是預期行為

### 11.9 SearchUser（Phase 4）

```json
{
  "id": "uuid",
  "username": "bob",
  "display_name": "Bob",
  "avatar_url": "https://...",
  "bio": "Hello"
}
```

形狀與 11.5 的 FollowUser 相同，前端可重用同一個列表元件。

**刻意不含 `followed_by_me`**——那會讓 `/search/users` 變成 optional auth 且與 `GET /users/{username}` 重複，得不償失。

### 11.10 Media（Phase 5，`POST /media` 的回應）

```json
{
  "id": "uuid",
  "purpose": "post_image",
  "url": "https://res.cloudinary.com/<cloud>/image/upload/c_limit,w_1600,f_auto,q_auto/pi_xxxxxxxxxxxxxxxxxxxxxx",
  "thumbnail_url": null,
  "width": 1600,
  "height": 1200,
  "bytes": 234567,
  "duration": null,
  "created_at": "2026-10-21T10:00:00Z"
}
```

- 所有欄位皆來自 Admin API 反查（27.1），不採信前端回報
- **不輸出**：`status`（P5 一律為 `ready`，輸出會讓前端以為需要處理其他狀態）、`owner_id`、Cloudinary 的 `asset_id` / `public_id`
- `thumbnail_url`、`duration` 只有影片有值；`width`、`height` 在取不到時為 `null`

### 11.11 Post Detail 的媒體項目（Phase 5，26.3）

```json
{
  "id": "uuid",
  "purpose": "post_video",
  "url": "https://res.cloudinary.com/<cloud>/video/upload/f_mp4,q_auto/pv_xxxxxxxxxxxxxxxxxxxxxx.mp4",
  "thumbnail_url": "https://res.cloudinary.com/<cloud>/video/upload/so_10p,c_limit,w_800,f_jpg,q_auto/pv_xxxxxxxxxxxxxxxxxxxxxx.jpg",
  "width": 720,
  "height": 1280,
  "duration": 42.5
}
```

與 11.10 的差別：不含 `bytes` 與 `created_at`（P5-0 §5.5 的形狀）。

---

## 12. Schema

### 12.1 Phase 1

Phase 1 的設計沒有被推翻，後續階段只做加法：新增表、索引與 `pg_trgm` 擴充（Phase 4 在 `posts`、`users` 上建了 trigram 索引），以及 Phase 5 為 `users` 新增一個 nullable 欄位 `avatar_media_id`（12.5）。沒有修改或刪除任何既有欄位。

五張表：`users`、`oauth_accounts`、`posts`、`tags`、`post_tags`。

歷史提案結果：SCP-1 成立（無 sessions 表，session 全在 Redis）、SCP-2 否決（username 維持 `NOT NULL`，改採 pending signup）、SCP-3 / SCP-4 原生已存在。

### 12.2 Phase 2 新增三張表

| 表 | 用途 | 刪除策略 |
|---|---|---|
| `post_likes` | 文章按讚。複合主鍵 `(user_id, post_id)` 保證同一人不能重複按讚，使 `PUT` 天然冪等 | hard delete |
| `comments` | 單層留言 | **soft delete**（內容型資料，與 posts 一致） |
| `follows` | 使用者對使用者的自關聯多對多。`CHECK (follower_id <> followee_id)` 防止追蹤自己 | hard delete |

**未新增**：`like_count` / `comment_count` 欄位（計數即時聚合，見 18.3）、`feeds` 表（Feed 採 pull 模型）、`comments.deleted_by`（MVP 不做稽核）。

### 12.3 Phase 3 新增三張表

| 表 | 用途 | 刪除策略 |
|---|---|---|
| `conversations` | 兩人之間的對話。`UNIQUE(user_low_id, user_high_id)` + **`CHECK(user_low_id < user_high_id)`** | hard delete |
| `messages` | 訊息本體。`UNIQUE(sender_id, client_message_id)` 保證冪等 | **無**（Phase 3 不做編輯／刪除，故無 soft delete） |
| `conversation_reads` | 每人在每個對話的已讀位置。PK `(conversation_id, user_id)` | hard delete |

**設計要點**：

- **`CHECK(user_low_id < user_high_id)` 一個約束同時保證兩件事**：順序正確（否則同一對人會產生兩列）與不能自己跟自己聊天（相等會違反 `<`）
- **`conversation_reads` 獨立成表而非放在 conversations**：若放在 conversations 會變成 `user_low_last_read_at` / `user_high_last_read_at`，每個未讀數查詢都要 `CASE WHEN ... THEN ... ELSE ... END`。寫反了**不會噴錯，只會顯示錯的未讀數**——最難發現的一種 bug
- **沒有 `conversation_reads` 那一列 = 從未讀過**。不設 nullable 欄位表達同一狀態，否則同一件事有兩種表示法，每支查詢都要處理兩種
- **索引** `messages(conversation_id, created_at DESC, id DESC)`：分頁與未讀數共用
- **應用層排序 user id 時必須與 PostgreSQL 一致**：PostgreSQL 的 uuid 型別按 16 bytes 二進位順序比較，Go 端應 parse 後用 `bytes.Compare`。全小寫 canonical 字串比較「碰巧」等價，但那依賴大小寫假設，一旦某處回傳大寫就會靜默排錯，且錯誤只表現為偶發的 CHECK violation

**未新增**：`last_message_at` 欄位（用 `JOIN LATERAL` 取最後訊息）、訊息的 soft delete。

### 12.4 Phase 4 新增一張表與搜尋索引

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

| 表 | 用途 | 刪除策略 |
|---|---|---|
| `notifications` | 通知（like / comment / follow） | hard delete |

**欄位設計要點**：

- **`actor_id` 為 `NOT NULL` + `ON DELETE CASCADE`**。不用 `SET NULL`——`users` 只做 soft delete（見刪帳號政策），該路徑不會被走到；更重要的是 `SET NULL` 搭配下方的 `NULLS NOT DISTINCT` 去重索引會讓多筆不同通知被誤判為同一筆。`CASCADE` 與專案內其他 8 個 `users` 外鍵（分佈於 7 張表，`conversations` 參照兩次）一致
- **不設 `is_read` 欄位**，只用 nullable `read_at`（NULL = 未讀）。兩個欄位表達同一件事就是兩份真相，遲早不一致。回應中的 `is_read` 由後端計算
- **`entity_id` 是 polymorphic，不設 FK**：`like` → post_id、`comment` → **comment_id**（不是 post_id——同一人可在同一篇文章留多則留言，每則都該通知）、`follow` → NULL

**兩個 CHECK**：

```sql
CONSTRAINT notifications_no_self CHECK (recipient_id <> actor_id),
CONSTRAINT notifications_entity_check CHECK (
    (type IN ('like','comment') AND entity_id IS NOT NULL)
 OR (type = 'follow'            AND entity_id IS NULL)
)
```

**三個索引**（另外四個搜尋用的 trigram 索引見下方）：

```sql
-- 列表：某人的通知依時間新到舊（tie-break 用 id）
CREATE INDEX notifications_recipient_idx ON notifications (recipient_id, created_at DESC, id DESC);

-- 未讀數。這不是上一個索引的冗餘 —— 後者無法過濾 read_at，
-- 用它算未讀數要掃過該使用者的全部通知；partial index 只含未讀列
CREATE INDEX notifications_unread_idx ON notifications (recipient_id) WHERE read_at IS NULL;

-- 去重（見下）
CREATE UNIQUE INDEX notifications_dedup_key
    ON notifications (recipient_id, actor_id, type, entity_id) NULLS NOT DISTINCT;
```

⚠️ **`NULLS NOT DISTINCT`（PG 15+）與上方的 `notifications_entity_check` 互鎖，缺一不可**：

- 沒有 `NULLS NOT DISTINCT`：PostgreSQL 預設把 NULL 視為互不相同 → `follow` 類型（`entity_id` 為 NULL）的去重**完全失效**
- 沒有那個 CHECK：無法保證「只有 follow 的 `entity_id` 是 NULL」→ `NULLS NOT DISTINCT` 可能把兩筆不同的 like 誤判為同一筆

**搜尋索引**（見 23.4 的效能說明）：

```sql
CREATE INDEX posts_title_trgm_idx   ON posts USING GIN (title gin_trgm_ops)
    WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX posts_excerpt_trgm_idx ON posts USING GIN (excerpt gin_trgm_ops)
    WHERE status = 'published' AND deleted_at IS NULL AND excerpt IS NOT NULL;
CREATE INDEX users_username_trgm_idx     ON users USING GIN (username gin_trgm_ops)
    WHERE deleted_at IS NULL;
CREATE INDEX users_display_name_trgm_idx ON users USING GIN (display_name gin_trgm_ops)
    WHERE deleted_at IS NULL AND display_name IS NOT NULL;
```

**不索引 `content_md`**：Neon 免費層只有 0.5GB，而 `excerpt`（寫入時產生的 200 字摘要）對「找文章」的效果已經足夠。

⚠️ **down migration 不 `DROP EXTENSION pg_trgm`**——extension 是資料庫層級的共用資源。

### 12.5 Phase 5 新增兩張表與一個欄位

Migration：`db/migrations/00005_phase5_media.sql`。P5A（圖片）與 P5B（影片）分段實作，但 migration 一次做完——已套用的 migration 永不修改（決策 #51），分兩個檔案沒有好處。

| 表 / 欄位 | 用途 | 刪除策略 |
|---|---|---|
| `media` | 一筆已上傳且通過反查驗證的檔案 | 不刪（不自動清理孤兒，#125）；使用者被 hard delete 時 CASCADE |
| `post_media` | 文章擁有哪些媒體。PK `(post_id, media_id)`、`position` | 隨整組替換重建；文章 soft delete 時保留（#126） |
| `users.avatar_media_id` | 頭像的生命週期指標（6.1.2） | 指向的 media 被刪時 `ON DELETE SET NULL`（#131） |

**`media` 的欄位設計要點**：

- **`provider_asset_id` 與 `provider_public_id` 並存**（#113）：前者不可變、後者可被 rename，所以 **UNIQUE 與冪等都下在 `provider_asset_id`**（`media_provider_asset_key`）
- **`purpose` 有 CHECK，但不設 `kind` 欄位**（#129）：圖片或影片完全由 purpose 決定，是衍生資料
- **`status`**（`pending` / `ready` / `failed`）：P5 一律寫 `ready`，其餘為未來的非同步轉檔預留
- **不設 `attached_at`**（#130）：「有沒有被使用」可由 `post_media` 與 `users.avatar_media_id` 推導
- `url`、`thumbnail_url`、`format`、`bytes`、`width`、`height`、`duration` 全部由後端反查後寫入

```sql
CONSTRAINT media_provider_asset_key UNIQUE (provider_asset_id),

-- duration 與 purpose 的一致性（同決策 #85 的手法：用 CHECK 鎖住「哪些欄位在哪些情況下必須有值」）
CONSTRAINT media_duration_check CHECK (
    (purpose =  'post_video' AND duration IS NOT NULL)
 OR (purpose <> 'post_video' AND duration IS NULL)
)
```

⚠️ `media_duration_check` 與未來的 `status = 'pending'` 衝突（影片剛建立時長度未知）。**升級成非同步轉檔時必須同時放寬這個 CHECK**（#145）。

**索引**：

```sql
CREATE INDEX media_owner_idx       ON media (owner_id, created_at DESC, id DESC);
CREATE INDEX post_media_media_idx  ON post_media (media_id);            -- 孤兒查詢反查用
CREATE INDEX users_avatar_media_idx ON users (avatar_media_id)
    WHERE avatar_media_id IS NOT NULL;                                   -- 孤兒查詢反查用
```

**`avatar_media_id` 用 `SET NULL`，與決策 #102 看似相反**：那裡（`notifications.actor_id`）拒用 `SET NULL`，因為它會讓 `NULLS NOT DISTINCT` 的去重索引把不同通知誤判為同一筆；這裡 NULL 本身就是有意義的狀態（「不是本站上傳的頭像」）。


### 12.6 Migration 管理

【決策】使用 **goose**（開發期工具，非執行期依賴）。

| 檔案 | 角色 |
|---|---|
| `db/migrations/*.sql` | **變更歷史的唯一真相**。已套用的檔案永不修改 |
| `db/schema.sql` | 完整快照，供閱讀與新環境參考。與 migrations 衝突時**以 migrations 為準** |

⚠️ **部署順序**：**先對正式資料庫執行 migration，再 push 程式碼**。新增表格對舊程式碼無害（舊程式不知道它存在），反之則會出錯。

⚠️ `CREATE TABLE / INDEX IF NOT EXISTS` **只比對名稱，不比對定義**。baseline migration 的索引名稱必須與既有資料庫逐字相同，否則會靜默建立重複索引（不報錯、不警告）。

⚠️ **`schema.sql` 的驗證方法**（決策 #144）：分別以「goose 從零套用全部 migration」與「直接執行 schema.sql」建立兩個資料庫，比對兩邊的擴充、表、索引、約束清單。**「執行沒有報錯」不等於正確**——Phase 4 的物件曾整段漏在 schema.sql 裡，就是這樣沒被發現的。

⚠️ **新增環境變數時的部署順序**（Phase 5）：先在 Render 加環境變數 → 對 Neon 執行 migration → 再 push。順序反了，新程式碼的正式環境護欄會讓服務啟動失敗。

---

## 13. 後端實作順序（本章為專案路線圖的唯一權威版本）

1. **基礎建設（任務 E）**：分層（handler / service / repository）、統一 response helper、**嚴格 JSON 綁定 helper（DisallowUnknownFields）**、config（env：DB、Redis、cookie secure、FRONTEND_ORIGIN、Google 憑證）、**CSRF middleware（Origin + Content-Type 檢查）**、auth middleware 骨架（required / optional 兩種模式）、repository 層封裝 `deleted_at IS NULL`、healthz 併入新結構。
2. **Redis 基建**：docker-compose 加 `redis:7-alpine`（不掛 volume）、go-redis、`scp:` 前綴常數、session / oauth_state / pending_signup 底層函式、cookie helper（含 `oauth_state` cookie、dev/prod 差異）、fail-closed / 降級處理（4.12）。
3. **Google OAuth（任務 F）**：申請憑證 → login（含 state cookie）、callback（三重 state 驗證、email_verified 檢查）、signup（正規化管線、constraint 分支、失敗收斂）、logout、`GET /me`。
   驗收：新帳號走完「Google → 選 username → /me」；老帳號直接登入；登出後 /me 回 401；雙分頁同時 signup 其中一個被視同登入。
4. **個人檔案（任務 G）**：`PATCH /me`（唯讀欄位 400）、`GET /users/{username}`。
5. **Posts CRUD（任務 H 主體）**：`POST /posts`（transaction + slug 重試）、`GET /users/{username}/posts/{slug}`（optional auth）、`PATCH` / `DELETE /posts/{id}`。
6. **列表與標籤**：`GET /posts`、`GET /me/posts`、`GET /users/{username}/posts`、`?tag=`、`GET /tags`（含 tie-break 排序）。
7. **收尾**：分頁、排序、excerpt 全端點驗收（curl / Postman）。

**Phase 2–5** 的實作順序與驗收見各階段文件（P2-0～P2-4、P3-0～P3-4、P4-0～P4-3、P5-0～P5-4）。Phase 5 的順序：P5-0 合約、migration 與 Cloudinary 設定 → P5-1 後端（簽章、反查、配額）→ P5-2 上傳元件與頭像 → P5-3 文章圖片 → P5-4 影片附件。

---

## 附錄 B：政策備註（未來功能的預先決策）

### B-1 刪除帳號政策（Phase 1 無此功能，先定政策避免矛盾）

【決策】未來實作刪帳號時：soft delete `users`（設 `deleted_at`）+ **hard delete 該 user 的所有 `oauth_accounts` 列** + 撤銷其所有 session。

- 理由：oauth 綁定若保留，該 Google 身分將永久卡死（`oauth_accounts_provider_uid_key` 指向已刪 user，無法重新註冊）；hard delete 綁定後，同一 Google 帳號可重新註冊為全新 user，舊內容仍以 soft delete 保留。
- 【事實 / 已知缺口】「撤銷某 user 的所有 session」目前做不到——session value 只有 `user_id`，無法反查。需要 `scp:user_sessions:{user_id}` 反向索引（Set），屆時一併實作。見附錄 C。

### B-2 帳號連結（account linking）政策草案（未來多登入方式時）

【建議】當未來新增 email/password 或其他 provider：僅在 provider 回報 `email_verified=true` 且 email 與既有帳號相符時，才**考慮**自動連結；且連結當下應要求驗證既有憑證（輸入密碼）或寄確認信。

- 【事實 / 風險】無條件自動連結存在 **account pre-hijacking** 攻擊面：攻擊者先用受害者 email 佔位建帳號，受害者日後 OAuth 登入被自動連結，攻擊者的舊憑證仍可進入該帳號。
- Phase 1 現行行為（Google-only）：signup 撞 email → `409 EMAIL_TAKEN`，不自動連結（兩個不同 sub 撞 email 身分不明）。

---

## 附錄 C：已知缺口（有意識地不做，非遺漏）

| 缺口 | 影響 | 何時處理 |
|---|---|---|
| Rate limiting（auth 端點防暴力 / 濫用） | signup 可被高頻嘗試。**Phase 5 已有第一個限流**（上傳簽章每人每日 50 次，#137），auth 端點仍無 | **已對外**（OAuth 已發布）；列為下一階段的待辦，可沿用 #137 的 Redis 模式 |
| 多裝置 session 管理 / 全裝置登出 | 需 `user_sessions` 反向索引 | 刪帳號或安全功能時 |
| CSRF token 機制 | 跨站部署（SameSite=None）時必須 | 跨站部署時 |
| PKCE | code 攔截防護的額外一層 | 可選強化 |
| username 可用性即時檢查端點 | 純 UX | 前端開發時視需要 |
| **Phase 5** | | |
| 草稿裡的媒體可被拿到網址的人看見（27.5，#124） | 草稿的秘密性對媒體不成立 | 需 signed delivery URL |
| 不自動清理孤兒（27.6，#125） | 儲存空間緩慢累積 | 須先解決 #112 的不同步 |
| Cloudinary 端的孤兒（上傳後未回報，#141） | SQL 查不到；以配額限制上限 | 人工檢查 Media Library |
| 直接呼叫 API 時，`media_ids` 可包含內文沒用到的圖片（#161 只在編輯器生效） | 孤兒查詢少算 | — |
| 既有文章的摘要要到下次編輯才套用 26.4 | 舊文章卡片可能仍顯示圖片網址 | 需要時寫一次性批次重算 |
| 影片第一次播放需等待轉檔（#118） | 首播延遲數秒 | 改 `eager_async` + 輪詢（屆時須放寬 `media_duration_check`，#145） |
| OAuth 帶入的 Google 頭像為外部連結 | 可能失效（前端顯示字母頭像） | 視 HTTP 狀態碼決定是否登入時更新 |
| 每篇最多 1 部影片、無 HLS | — | — |

---

## 附錄 D：取捨與進階版本一覽（面試可講）

| 主題 | Phase 1 選擇 | 進階版本 | 為何先簡單 |
|---|---|---|---|
| 登入狀態 | Redis session、7 天絕對過期 | sliding TTL / 多裝置管理 | 絕對過期最可預測 |
| 註冊流程 | pending signup（選好 username 才建帳號） | 暫代 username + 完成度旗標 | DB 不留半成品、schema 零變更 |
| 並行 signup | 身分錨點重查收斂（5.3.1） | 分散式鎖 / 佇列 | DB constraint + 阻塞語意已足夠 |
| OAuth 防護 | state（cookie 綁定 + Redis 一次性） | + PKCE | confidential client 已足 |
| CSRF | Lax + Content-Type + Origin 多層 | CSRF token | 同站部署下已足，且明文記錄升級條件 |
| slug 唯一性 | DB constraint + 寫入重試 | 分散式 ID / 預留池 | constraint 就是權威 |
| 分頁 | offset | cursor | 資料量小、直觀 |
| username | 一次設定不可改 | 可改 + handle 歷史 | 避免連結失效 |
| 列表內容 | 寫入時產 excerpt | 全文快取 | 讀取零成本 |
| 媒體儲存（P5） | Cloudinary Free（#108） | 自建 R2 / S3 + 轉檔佇列 | 免費方案跑不動 ffmpeg，且沒有持久檔案系統 |
| 上傳路徑（P5） | 簽章直傳，檔案不經過 Go（#109） | 經後端代理上傳 | 0.1 CPU / 512MB 扛不住檔案流量 |
| 上傳驗證（P5） | 回報後以 Admin API 反查（#114） | webhook 通知 | webhook 會被 CSRF 的 Origin 檢查擋下（#132），且 Render 休眠 |
| 濫用防護（P5） | 單次簽章 + 每日 50 個（#135、#137） | 依用量動態配額 | 先補「簽章可重複使用」的洞，配額才有意義 |
| 影片（P5） | on-the-fly mp4，無 HLS（#118、#120） | eager + adaptive streaming | 每個畫質一份 derived asset，額度消耗成倍 |
| 圖片壓縮（P5） | 前端 Canvas，只是最佳化（#123） | 後端統一處理 | 省上傳時間與額度；限制本來就在後端 |

---

*文件結束。v5.0 為合約基準線；任何欄位 / 路徑 / 格式變更，請先改本文件並通知前後端兩邊。*