-- Transactional outbox: event ditulis di transaksi yang sama dengan data
-- bisnis, lalu dikirim ke Kafka (atau handler lokal) oleh relay di worker.
-- status: pending -> published, atau failed setelah max_attempts.
CREATE TABLE outbox_events
(
    id           CHAR(36)     NOT NULL,
    topic        VARCHAR(100) NOT NULL,
    msg_key      VARCHAR(100) NOT NULL,
    aggregate_id CHAR(36)     NOT NULL,
    event_type   VARCHAR(100) NOT NULL,
    payload      JSON         NOT NULL,
    headers      JSON         NULL,
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    attempts     INT          NOT NULL DEFAULT 0,
    last_error   TEXT         NULL,
    created_at   BIGINT       NOT NULL,
    published_at BIGINT       NULL,
    PRIMARY KEY (id),
    KEY idx_outbox_status_created (status, created_at),
    KEY idx_outbox_aggregate (aggregate_id, status)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
