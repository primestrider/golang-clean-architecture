-- Catatan event yang sudah diproses tiap consumer, untuk idempotency:
-- event yang terkirim ulang (at-least-once) tidak diproses dua kali.
CREATE TABLE processed_events
(
    consumer_name VARCHAR(100) NOT NULL,
    event_id      CHAR(36)     NOT NULL,
    processed_at  BIGINT       NOT NULL,
    PRIMARY KEY (consumer_name, event_id),
    KEY idx_processed_events_at (processed_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
