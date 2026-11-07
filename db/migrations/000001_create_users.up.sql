-- Tabel user. id berupa UUID; email dipakai untuk login.
-- deleted_at = 0 berarti aktif (soft delete). Dengan nilai 0, bukan NULL,
-- unique index (email, deleted_at) tetap berlaku untuk user aktif, sementara
-- email user yang sudah dihapus boleh dipakai mendaftar lagi.
CREATE TABLE users
(
    id         CHAR(36)     NOT NULL,
    email      VARCHAR(255) NOT NULL,
    name       VARCHAR(100) NOT NULL,
    password   VARCHAR(100) NOT NULL,
    role       VARCHAR(20)  NOT NULL DEFAULT 'user',
    avatar_key VARCHAR(255) NULL,
    version    INT          NOT NULL DEFAULT 1,
    created_at BIGINT       NOT NULL,
    updated_at BIGINT       NOT NULL,
    deleted_at BIGINT       NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_email (email, deleted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
