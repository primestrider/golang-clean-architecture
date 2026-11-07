-- Refresh token disimpan hanya dalam bentuk hash SHA-256 (token_hash).
-- family_id mengelompokkan token hasil rotasi dari satu login. Jika token
-- lama dipakai ulang, seluruh keluarga dicabut (deteksi pencurian token).
CREATE TABLE refresh_tokens
(
    id             CHAR(36)     NOT NULL,
    user_id        CHAR(36)     NOT NULL,
    family_id      CHAR(36)     NOT NULL,
    token_hash     CHAR(64)     NOT NULL,
    expires_at     BIGINT       NOT NULL,
    revoked_at     BIGINT       NULL,
    replaced_by_id CHAR(36)     NULL,
    user_agent     VARCHAR(255) NULL,
    ip             VARCHAR(45)  NULL,
    created_at     BIGINT       NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_refresh_tokens_hash (token_hash),
    KEY idx_refresh_tokens_user (user_id),
    KEY idx_refresh_tokens_family (family_id),
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
