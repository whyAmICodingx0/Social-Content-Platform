-- +goose Up
-- ============================================================
-- Phase 2：按讚、留言、關注
-- 本檔在既有資料庫會實際執行（這三張表尚不存在）
-- ============================================================

-- post_likes：文章按讚。關聯型資料 → hard delete
CREATE TABLE post_likes (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id     UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 複合主鍵：DB 層保證同一人不能對同一篇文章按讚兩次，
    -- 讓 PUT 天然冪等（決策 #44）
    PRIMARY KEY (user_id, post_id)
);

-- 反向查詢：「這篇文章有幾個讚」。
-- 主鍵前綴是 user_id，無法服務以 post_id 為條件的查詢
CREATE INDEX post_likes_post_id_idx ON post_likes (post_id);


-- comments：單層留言。內容型資料 → soft delete
CREATE TABLE comments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
    -- 未來若支援巢狀回覆：新增 parent_id UUID REFERENCES comments(id)
);

-- 主要查詢：某篇文章的留言，依時間正序（合約 5.3）。
-- 索引方向與查詢方向一致（ASC），純粹為語意清楚——
-- PostgreSQL 能反向掃描 B-tree，各欄同向時 DESC/ASC 效能無差別。
-- created_at + id 對應排序 tie-break（決策 #27）
CREATE INDEX comments_post_idx
    ON comments (post_id, created_at ASC, id ASC)
    WHERE deleted_at IS NULL;

CREATE INDEX comments_author_idx
    ON comments (author_id)
    WHERE deleted_at IS NULL;


-- follows：使用者對使用者的自關聯多對多。關聯型資料 → hard delete
CREATE TABLE follows (
    follower_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (follower_id, followee_id),

    -- DB 層防止追蹤自己。應用層也會檢查（為了給可讀的錯誤訊息），
    -- 但權威在這裡——與 slug 唯一性同樣的原則（決策 #28）
    CONSTRAINT follows_no_self CHECK (follower_id <> followee_id)
);

-- 反向查詢：「誰追蹤我」（粉絲列表、粉絲數）
CREATE INDEX follows_followee_idx ON follows (followee_id);

-- +goose Down
DROP TABLE IF EXISTS follows;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS post_likes;