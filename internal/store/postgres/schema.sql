-- links is the source of truth. click_daily is the async aggregate.
-- Raw client IPs are never stored.

CREATE TABLE IF NOT EXISTS links (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    created_by TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    click_count BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS click_daily (
    link_id TEXT NOT NULL REFERENCES links (id),
    day DATE NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (link_id, day)
);
