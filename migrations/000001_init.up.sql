CREATE TABLE IF NOT EXISTS chats (
    id BIGINT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS links (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    last_update TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chat_links (
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    link_id BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, link_id)
);

CREATE TABLE IF NOT EXISTS link_tags (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    link_id BIGINT NOT NULL,
    tag TEXT NOT NULL,
    UNIQUE (chat_id, link_id, tag),
    FOREIGN KEY (chat_id, link_id) REFERENCES chat_links(chat_id, link_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS link_filters (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    link_id BIGINT NOT NULL,
    filter TEXT NOT NULL,
    UNIQUE (chat_id, link_id, filter),
    FOREIGN KEY (chat_id, link_id) REFERENCES chat_links(chat_id, link_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chat_links_chat_id ON chat_links(chat_id);
CREATE INDEX IF NOT EXISTS idx_chat_links_link_id ON chat_links(link_id);
CREATE INDEX IF NOT EXISTS idx_links_last_update ON links(last_update);
CREATE INDEX IF NOT EXISTS idx_link_tags_chat_link ON link_tags(chat_id, link_id);
CREATE INDEX IF NOT EXISTS idx_link_filters_chat_link ON link_filters(chat_id, link_id);
