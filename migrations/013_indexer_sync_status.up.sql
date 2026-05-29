-- migrations/013_indexer_sync_status.up.sql
CREATE TABLE IF NOT EXISTS indexer_sync_status (
    id                     SMALLINT PRIMARY KEY CHECK (id = 1),
    db_height              BIGINT       NOT NULL,
    znnd_frontier_height   BIGINT       NOT NULL,
    znnd_target_height     BIGINT       NOT NULL,
    drift_momentums        BIGINT       NOT NULL,
    node_lag_momentums     BIGINT       NOT NULL,
    state                  TEXT         NOT NULL,
    consecutive_bad_checks INTEGER      NOT NULL DEFAULT 0,
    active_node_url        TEXT         NOT NULL,
    active_node_label      TEXT         NOT NULL,
    chain_identifier       TEXT         NOT NULL,
    failed_over_at         BIGINT,
    last_progress_at       BIGINT       NOT NULL,
    checked_at             BIGINT       NOT NULL
);
