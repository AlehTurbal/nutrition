-- Phase 5: chat assistant. Multiple threads per user; each thread holds the
-- visible user/assistant dialog. User scoping flows through chat_threads.user_id;
-- the tool-use exchange and proposals are not persisted.

CREATE TABLE chat_threads (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_threads_user ON chat_threads (user_id, created_at DESC);

CREATE TABLE chat_messages (
    id         BIGSERIAL PRIMARY KEY,
    thread_id  BIGINT NOT NULL REFERENCES chat_threads(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_messages_thread ON chat_messages (thread_id, created_at);
