-- ============================================================
-- Phase 1 Schema
-- 資料表：users / oauth_accounts / posts / tags / post_tags
-- 目標環境：PostgreSQL 17（Docker）。不依賴 PostgreSQL 18 專屬功能。
--          gen_random_uuid() 為 PostgreSQL 13+ 內建，無需任何 extension。
--
-- 執行方式（擇一）：
--   1) 資料庫 GUI（DBeaver / TablePlus）開 SQL 編輯器貼上執行
--   2) psql -U app -d social_dev -f schema.sql
--
-- 應用層約定（本 schema 不使用任何 trigger，以下規則由 Go 程式負責）：
--   1) 所有 UPDATE 語句必須自行帶上 updated_at = now()。
--      資料庫的 DEFAULT now() 只在 INSERT 時生效。
--   2) 查詢 users、posts 時，一律加 WHERE deleted_at IS NULL，
--      建議封裝在 Go 的 repository 層統一處理，避免遺漏。
-- ============================================================


-- ============================================================
-- 1. users：使用者身分與個人檔案
-- ============================================================
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT        NOT NULL,             -- 公開 handle，URL 用：/@username
    email         TEXT        NOT NULL,             -- 帳號 email（首次 Google 登入時取得）
    display_name  TEXT,                             -- 顯示名稱，可含空白/emoji，不需唯一
    avatar_url    TEXT,
    bio           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ                       -- soft delete：NULL = 未刪除
);

-- email 唯一：以 lower(email) 比對，避免大小寫造成重複帳號。
-- partial index（WHERE deleted_at IS NULL）讓已刪除帳號的 email 可被重新註冊。
CREATE UNIQUE INDEX users_email_key
    ON users (lower(email))
    WHERE deleted_at IS NULL;

-- username 唯一：同樣以 lower(username) 比對（Bob 與 bob 視為同一個）。
CREATE UNIQUE INDEX users_username_key
    ON users (lower(username))
    WHERE deleted_at IS NULL;


-- ============================================================
-- 2. oauth_accounts：第三方登入綁定（Phase 1 只有 google）
-- ============================================================
CREATE TABLE oauth_accounts (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL,          -- 'google'；未來可加 'github' 等
    provider_user_id TEXT        NOT NULL,          -- Google 的 sub claim（穩定且唯一的使用者 id）
    email            TEXT,                          -- 該 provider 回報的 email（僅供參考）
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 同一個第三方帳號只能綁定一次；登入時以 (provider, provider_user_id) 找回 user。
    CONSTRAINT oauth_accounts_provider_uid_key UNIQUE (provider, provider_user_id)
);

-- 反向查詢：某個 user 綁了哪些第三方帳號
CREATE INDEX oauth_accounts_user_id_idx ON oauth_accounts (user_id);


-- ============================================================
-- 3. posts：文章（只存 Markdown 原文）
-- ============================================================
CREATE TABLE posts (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         TEXT        NOT NULL,
    slug          TEXT        NOT NULL,             -- URL 用：/@username/slug
    content_md    TEXT        NOT NULL,             -- Markdown 原文（唯一真實來源，不另存 HTML）
    excerpt       TEXT,                             -- 摘要（選填）
    status        TEXT        NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft', 'published')),
    published_at  TIMESTAMPTZ,                      -- 首次發布時間；draft 時為 NULL
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ                       -- soft delete：NULL = 未刪除
);

-- slug 在「每位作者底下」唯一（不同作者可以用相同 slug）。
-- 此 index 前綴為 author_id，因此「查某作者全部文章（含草稿）」也能使用它。
CREATE UNIQUE INDEX posts_author_slug_key
    ON posts (author_id, slug)
    WHERE deleted_at IS NULL;

-- 某位作者的「公開文章列表」：WHERE author_id = $1 AND status = 'published'
--                              ORDER BY published_at DESC
CREATE INDEX posts_author_published_idx
    ON posts (author_id, published_at DESC)
    WHERE status = 'published' AND deleted_at IS NULL;

-- 全站最新文章列表（首頁 / 未來 feed 的基礎）
CREATE INDEX posts_published_idx
    ON posts (published_at DESC)
    WHERE status = 'published' AND deleted_at IS NULL;


-- ============================================================
-- 4. tags：標籤主檔（全站共用）
-- ============================================================
CREATE TABLE tags (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,               -- 顯示名稱，如 'Web Development'
    slug        TEXT        NOT NULL,               -- URL 用，如 'web-development'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tags_slug_key ON tags (slug);


-- ============================================================
-- 5. post_tags：posts 與 tags 的多對多中介表（join table）
--    關聯型資料：直接 hard delete，不做 soft delete。
-- ============================================================
CREATE TABLE post_tags (
    post_id   UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id    UUID NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,

    -- 複合主鍵：防止同一篇文章重複掛同一個標籤；
    -- 同時就是「查某篇文章的所有標籤」的索引。
    PRIMARY KEY (post_id, tag_id)
);

-- 反向查詢：「某個標籤底下有哪些文章」
CREATE INDEX post_tags_tag_id_idx ON post_tags (tag_id);

-- ============================================================
-- 6. post_likes：文章按讚（Phase 2）
--    關聯型資料：hard delete，不做 soft delete。
-- ============================================================
CREATE TABLE post_likes (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id     UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 複合主鍵：DB 層保證同一人不能對同一篇文章按讚兩次，
    -- 讓 PUT /like 天然冪等。
    PRIMARY KEY (user_id, post_id)
);

-- 反向查詢：「這篇文章有幾個讚」
CREATE INDEX post_likes_post_id_idx ON post_likes (post_id);


-- ============================================================
-- 7. comments：單層留言（Phase 2）
--    內容型資料：soft delete，與 posts 一致。
-- ============================================================
CREATE TABLE comments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

-- 某篇文章的留言，依時間正序（索引方向與查詢方向一致）
CREATE INDEX comments_post_idx
    ON comments (post_id, created_at ASC, id ASC)
    WHERE deleted_at IS NULL;

CREATE INDEX comments_author_idx
    ON comments (author_id)
    WHERE deleted_at IS NULL;


-- ============================================================
-- 8. follows：使用者對使用者的自關聯多對多（Phase 2）
--    關聯型資料：hard delete。
-- ============================================================
CREATE TABLE follows (
    follower_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (follower_id, followee_id),

    -- DB 層防止追蹤自己；應用層也會檢查，但權威在這裡。
    CONSTRAINT follows_no_self CHECK (follower_id <> followee_id)
);

-- 反向查詢：「誰追蹤我」（粉絲列表、粉絲數）
CREATE INDEX follows_followee_idx ON follows (followee_id);

-- conversations：兩人之間的對話。
-- 唯一性由 DB 保證：兩個 user id 排序後存入，UNIQUE 讓 A↔B 與 B↔A 是同一列。
CREATE TABLE conversations (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_low_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_high_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT conversations_pair_key UNIQUE (user_low_id, user_high_id),

    -- 一個 CHECK 同時保證「順序正確」與「不能自己跟自己聊天」。
    -- 應用層仍會先擋下自我對話以給出可讀訊息（同決策 #28 的雙層原則）。
    CONSTRAINT conversations_ordered CHECK (user_low_id < user_high_id)
);

-- 「我的對話列表」查詢是 WHERE user_low_id = $me OR user_high_id = $me。
--
-- ⚠️ 刻意不建 user_low_id 的單欄索引：
--    conversations_pair_key 是 (user_low_id, user_high_id) 的複合索引，
--    PostgreSQL 可以只用它的「前綴欄位」來服務 user_low_id = $me，
--    再建一個單欄索引是完全重複的，只會拖慢寫入。
--    user_high_id 不是任何索引的前綴，所以它需要自己的索引。
CREATE INDEX conversations_user_high_idx ON conversations (user_high_id);


-- messages：訊息本體。Phase 3 不做編輯／刪除，故無 soft delete。
CREATE TABLE messages (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id   UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 前端產生的 UUID，用於冪等（網路重試不會產生兩則訊息）
    -- 與樂觀更新的比對（收到回音時替換本地 pending 訊息）。
    client_message_id UUID        NOT NULL,

    content           TEXT        NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 冪等的權威保證。撞到此約束時，於交易外重查既有訊息並回 200
    -- （同決策 #19 的收斂模式：不可在已 rollback 的交易內重查）。
    CONSTRAINT messages_client_id_key UNIQUE (sender_id, client_message_id)
);

-- 分頁與未讀數共用。方向與查詢一致（DESC）：
-- keyset 分頁的 before 模式是 ORDER BY created_at DESC, id DESC，
-- after 模式的 ASC 由 PostgreSQL 反向掃描同一個索引處理。
CREATE INDEX messages_conversation_idx
    ON messages (conversation_id, created_at DESC, id DESC);


-- conversation_reads：每人在每個對話的已讀位置。
-- ⚠️ 沒有這一列 = 從未讀過。不使用 NULL 表達同一狀態。
CREATE TABLE conversation_reads (
    conversation_id      UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id              UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 給未來的「未讀分隔線」UI 使用
    last_read_message_id UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,

    -- 未讀數查詢用：count(*) WHERE created_at > last_read_at。
    -- 與 last_read_message_id 在同一句 SQL 內同時寫入，結構上不可能漂移。
    last_read_at         TIMESTAMPTZ NOT NULL,

    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (conversation_id, user_id)
);