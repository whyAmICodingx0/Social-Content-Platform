-- +goose Up
-- ============================================================
-- Phase 3：1 對 1 私訊
-- ============================================================

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

-- +goose Down
DROP TABLE IF EXISTS conversation_reads;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;