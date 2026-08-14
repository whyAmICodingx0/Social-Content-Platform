-- +goose Up
-- ============================================================
-- Phase 1 初始 schema（baseline）
--
-- 所有語句皆為 IF NOT EXISTS：
--   · 空資料庫   → 完整建立五張表與索引
--   · 既有資料庫 → 全部跳過，僅寫入 goose 版本紀錄
-- 這讓「新環境從零建置」與「既有環境納管」共用同一個檔案。
--
-- ⚠️⚠️ 警告：正式環境永遠不要 goose down 到此版本 ⚠️⚠️
-- 下方的 Down 會 DROP Phase 1 全部五張表，連同所有使用者與文章資料。
-- 這個 Down 只用於「在本機重建乾淨資料庫」。
--
-- ⚠️ 索引名稱必須與既有資料庫逐字相同（見教學段落 a）。
--    IF NOT EXISTS 只比對名稱不比對定義，名稱不符會靜默建立重複索引。
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT        NOT NULL,
    email         TEXT        NOT NULL,
    display_name  TEXT,
    avatar_url    TEXT,
    bio           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_key
    ON users (lower(email))
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_username_key
    ON users (lower(username))
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL,
    provider_user_id TEXT        NOT NULL,
    email            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT oauth_accounts_provider_uid_key UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS oauth_accounts_user_id_idx ON oauth_accounts (user_id);

CREATE TABLE IF NOT EXISTS posts (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         TEXT        NOT NULL,
    slug          TEXT        NOT NULL,
    content_md    TEXT        NOT NULL,
    excerpt       TEXT,
    status        TEXT        NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft', 'published')),
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS posts_author_slug_key
    ON posts (author_id, slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS posts_author_published_idx
    ON posts (author_id, published_at DESC)
    WHERE status = 'published' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS posts_published_idx
    ON posts (published_at DESC)
    WHERE status = 'published' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tags (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS tags_slug_key ON tags (slug);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id   UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id    UUID NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

CREATE INDEX IF NOT EXISTS post_tags_tag_id_idx ON post_tags (tag_id);

-- +goose Down
-- ⚠️ 僅供本機重建使用，正式環境絕不執行
DROP TABLE IF EXISTS post_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS users;