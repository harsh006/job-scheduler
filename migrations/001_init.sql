CREATE TABLE IF NOT EXISTS jobs (
    id               VARCHAR(36)   NOT NULL PRIMARY KEY,
    name             VARCHAR(255)  NOT NULL,
    cron_expr        VARCHAR(100)  NOT NULL,
    url              VARCHAR(2048) NOT NULL,
    http_method      VARCHAR(10)   NOT NULL DEFAULT 'POST',
    payload          TEXT,
    headers          JSON,
    status           ENUM('active','paused','deleted') NOT NULL DEFAULT 'active',
    max_retries      INT           NOT NULL DEFAULT 3,
    retry_delay_secs INT           NOT NULL DEFAULT 5,
    next_run_at      DATETIME(3)   NOT NULL,
    created_at       DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at       DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
                                    ON UPDATE CURRENT_TIMESTAMP(3),
    INDEX idx_jobs_status_next_run (status, next_run_at)
);

CREATE TABLE IF NOT EXISTS runs (
    id            VARCHAR(36) NOT NULL PRIMARY KEY,
    job_id        VARCHAR(36) NOT NULL,
    status        ENUM('running','succeeded','failed') NOT NULL,
    attempt       INT         NOT NULL DEFAULT 1,
    started_at    DATETIME(3) NOT NULL,
    finished_at   DATETIME(3),
    duration_ms   BIGINT,
    response_code INT,
    error_message TEXT,
    FOREIGN KEY (job_id) REFERENCES jobs(id),
    INDEX idx_runs_job_started (job_id, started_at DESC),
    INDEX idx_runs_started     (started_at DESC)
);
