CREATE TABLE IF NOT EXISTS tour_embed_tokens (
    id VARCHAR(50) PRIMARY KEY,
    tour_id VARCHAR(50) NOT NULL,
    token VARCHAR(80) NOT NULL,
    name VARCHAR(100),
    allowed_domains TEXT,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    created_by VARCHAR(50),
    updated_by VARCHAR(50)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tour_embed_tokens_token ON tour_embed_tokens (token);
CREATE INDEX IF NOT EXISTS idx_tour_embed_tokens_tour_id ON tour_embed_tokens (tour_id);
