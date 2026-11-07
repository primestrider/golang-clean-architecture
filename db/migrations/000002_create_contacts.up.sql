CREATE TABLE contacts
(
    id         CHAR(36)     NOT NULL,
    user_id    CHAR(36)     NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name  VARCHAR(100) NULL,
    email      VARCHAR(200) NULL,
    phone      VARCHAR(20)  NULL,
    version    INT          NOT NULL DEFAULT 1,
    created_at BIGINT       NOT NULL,
    updated_at BIGINT       NOT NULL,
    deleted_at BIGINT       NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_contacts_user (user_id, deleted_at),
    CONSTRAINT fk_contacts_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
