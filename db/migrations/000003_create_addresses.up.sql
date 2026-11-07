CREATE TABLE addresses
(
    id          CHAR(36)     NOT NULL,
    contact_id  CHAR(36)     NOT NULL,
    street      VARCHAR(255) NULL,
    city        VARCHAR(255) NULL,
    province    VARCHAR(255) NULL,
    postal_code VARCHAR(10)  NULL,
    country     VARCHAR(100) NULL,
    version     INT          NOT NULL DEFAULT 1,
    created_at  BIGINT       NOT NULL,
    updated_at  BIGINT       NOT NULL,
    deleted_at  BIGINT       NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_addresses_contact (contact_id, deleted_at),
    CONSTRAINT fk_addresses_contact FOREIGN KEY (contact_id) REFERENCES contacts (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
