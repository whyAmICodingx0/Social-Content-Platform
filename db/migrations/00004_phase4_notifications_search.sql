-- +goose Up
-- ============================================================
-- Phase 4：通知系統 + 搜尋
-- ============================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ------------------------------------------------------------
-- notifications
-- ------------------------------------------------------------
CREATE TABLE notifications (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- 收到通知的人
    recipient_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 觸發者。NOT NULL：決策 #20 已定案 users 只做 soft delete，
    -- 所以「actor 消失」的情況不存在，nullable 是多餘的複雜度。
    -- 更重要的是：SET NULL 搭配下方的 NULLS NOT DISTINCT 去重索引，
    -- 會讓多筆不同通知被誤判為同一筆。
    -- ON DELETE CASCADE 與專案內其他 8 個 users 外鍵一致。
    actor_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    type         TEXT        NOT NULL CHECK (type IN ('like', 'comment', 'follow')),

    -- 相關實體（polymorphic，不設 FK —— 型別依 type 而異）：
    --   like    → post_id
    --   comment → comment_id（不是 post_id！同一人可在同一篇文章留多則留言，
    --             每則都該通知，用 comment_id 才不會被去重索引擋掉）
    --   follow  → NULL
    entity_id    UUID,

    -- ⚠️ 不設 is_read 欄位。NULL = 未讀、非 NULL = 已讀。
    -- 兩個欄位表達同一件事就是兩份真相，遲早不一致。
    -- 這不違反決策 #71 —— #71 禁止的是「沒有那一列」與「NULL」表達同一狀態，
    -- 此處列必然存在，NULL 是唯一的表示法。
    read_at      TIMESTAMPTZ,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- 不通知自己。應用層也會先擋（給可讀訊息），DB 是最終防線。
    -- 同決策 #66 / #28 的雙層原則。
    CONSTRAINT notifications_no_self CHECK (recipient_id <> actor_id),

    -- ⚠️ 這個 CHECK 與下方的 NULLS NOT DISTINCT 互鎖，缺一不可：
    -- 它保證「只有 follow 的 entity_id 是 NULL」，
    -- 因此 NULLS NOT DISTINCT 不會誤把兩筆不同的 like 當成同一筆。
    CONSTRAINT notifications_entity_check CHECK (
        (type IN ('like', 'comment') AND entity_id IS NOT NULL)
     OR (type = 'follow'            AND entity_id IS NULL)
    )
);

-- 通知列表：某人的通知依時間新到舊（tie-break 用 id，決策 #27）
CREATE INDEX notifications_recipient_idx
    ON notifications (recipient_id, created_at DESC, id DESC);

-- 未讀數。
-- ⚠️ 這不是 notifications_recipient_idx 的冗餘：後者無法過濾 read_at，
-- 用它算未讀數要掃過該使用者的全部通知；partial index 只含未讀列，小得多。
-- （對照 P3-0 刪掉的 conversations_user_low_idx —— 那個才是真正的前綴冗餘。）
CREATE INDEX notifications_unread_idx
    ON notifications (recipient_id)
    WHERE read_at IS NULL;

-- 去重（決策 #85）。
-- NULLS NOT DISTINCT（PG 15+）讓 follow 類型（entity_id 為 NULL）也能去重 ——
-- 沒有它，PostgreSQL 預設把 NULL 視為互不相同，追蹤通知會完全失去去重能力。
CREATE UNIQUE INDEX notifications_dedup_key
    ON notifications (recipient_id, actor_id, type, entity_id)
    NULLS NOT DISTINCT;

-- ------------------------------------------------------------
-- 搜尋索引（決策 #94：只索引 title 與 excerpt，不索引 content_md）
-- ------------------------------------------------------------
-- 不索引 content_md 的理由：Neon 免費層只有 0.5GB，
-- 而 excerpt（寫入時產生的 200 字摘要）對「找文章」的效果已經足夠。

CREATE INDEX posts_title_trgm_idx
    ON posts USING GIN (title gin_trgm_ops)
    WHERE status = 'published' AND deleted_at IS NULL;

-- excerpt 是 nullable（schema 定義 + 寫入用 NULLIF），
-- 故 partial index 加上 excerpt IS NOT NULL 以縮小索引
CREATE INDEX posts_excerpt_trgm_idx
    ON posts USING GIN (excerpt gin_trgm_ops)
    WHERE status = 'published' AND deleted_at IS NULL AND excerpt IS NOT NULL;

-- 使用者搜尋（決策 #101）
CREATE INDEX users_username_trgm_idx
    ON users USING GIN (username gin_trgm_ops)
    WHERE deleted_at IS NULL;

CREATE INDEX users_display_name_trgm_idx
    ON users USING GIN (display_name gin_trgm_ops)
    WHERE deleted_at IS NULL AND display_name IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_display_name_trgm_idx;
DROP INDEX IF EXISTS users_username_trgm_idx;
DROP INDEX IF EXISTS posts_excerpt_trgm_idx;
DROP INDEX IF EXISTS posts_title_trgm_idx;
DROP TABLE IF EXISTS notifications;

-- ⚠️ 刻意不 DROP EXTENSION pg_trgm ——
-- extension 是資料庫層級的共用資源，可能有其他使用者。