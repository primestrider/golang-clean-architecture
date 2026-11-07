-- Jejak audit: siapa (actor_id) melakukan apa (action) pada data mana.
-- changes berisi field yang berubah (before/after), tanpa field sensitif.
CREATE TABLE audit_logs
(
    id          CHAR(36)     NOT NULL,
    actor_id    CHAR(36)     NULL,
    action      VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50)  NOT NULL,
    entity_id   CHAR(36)     NOT NULL,
    request_id  VARCHAR(64)  NULL,
    changes     JSON         NULL,
    created_at  BIGINT       NOT NULL,
    PRIMARY KEY (id),
    KEY idx_audit_logs_entity (entity_type, entity_id),
    KEY idx_audit_logs_actor (actor_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
