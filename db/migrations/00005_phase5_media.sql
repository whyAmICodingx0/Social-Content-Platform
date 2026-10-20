-- +goose Up
-- ============================================================
-- Phase 5：媒體上傳（Cloudinary）
-- ============================================================

CREATE TABLE media (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Cloudinary 的兩個識別碼。
    -- asset_id 不可變；public_id 可被 rename ——
    -- 所以唯一性與冪等都下在 asset_id（決策 #113）。
    provider_asset_id  TEXT        NOT NULL,
    provider_public_id TEXT        NOT NULL,

    -- 用途。⚠️ 刻意不設 kind 欄位 —— 它完全由 purpose 決定，
    -- 是衍生資料（同決策 #55 的判斷）。
    purpose            TEXT        NOT NULL
                       CHECK (purpose IN ('avatar', 'post_image', 'post_video')),

    -- P5 一律直接寫 'ready'（同步反查成功才建立）。
    -- 'pending' / 'failed' 為未來的非同步路徑預留（決策 #118）。
    status             TEXT        NOT NULL
                       CHECK (status IN ('pending', 'ready', 'failed')),

    -- ⚠️ 以下全部由後端 Admin API 反查後寫入，不採信前端回報（決策 #114）
    url                TEXT        NOT NULL,
    thumbnail_url      TEXT,
    format             TEXT        NOT NULL,
    bytes              BIGINT      NOT NULL,
    width              INT,
    height             INT,
    duration           REAL,

    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT media_provider_asset_key UNIQUE (provider_asset_id),

    CONSTRAINT media_duration_check CHECK (
        (purpose =  'post_video' AND duration IS NOT NULL)
     OR (purpose <> 'post_video' AND duration IS NULL)
    )
);

CREATE INDEX media_owner_idx ON media (owner_id, created_at DESC, id DESC);


-- post_media：文章擁有哪些媒體。
--
-- ⚠️ 對「圖片」而言，這張表的唯一用途是生命週期追蹤（孤兒查詢），
-- 完全不影響渲染 —— 圖片顯示與否由 content_md 的 Markdown 決定（決策 #112）。
-- 對「影片」而言它同時是渲染來源（附件區塊）。
CREATE TABLE post_media (
    post_id  UUID NOT NULL REFERENCES posts(id)  ON DELETE CASCADE,
    media_id UUID NOT NULL REFERENCES media(id)  ON DELETE CASCADE,
    position INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (post_id, media_id)
);

CREATE INDEX post_media_media_idx ON post_media (media_id);


-- users.avatar_media_id：生命週期指標（決策 #111）。
ALTER TABLE users
    ADD COLUMN avatar_media_id UUID REFERENCES media(id) ON DELETE SET NULL;

CREATE INDEX users_avatar_media_idx ON users (avatar_media_id)
    WHERE avatar_media_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_avatar_media_idx;
ALTER TABLE users DROP COLUMN IF EXISTS avatar_media_id;
DROP TABLE IF EXISTS post_media;
DROP TABLE IF EXISTS media;