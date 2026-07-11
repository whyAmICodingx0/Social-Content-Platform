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
