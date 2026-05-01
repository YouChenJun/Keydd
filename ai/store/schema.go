package store

// SQLite 建表语句
const sqliteSchema = `
CREATE TABLE IF NOT EXISTS traffic_analysis (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    host             TEXT NOT NULL,
    path             TEXT NOT NULL,
    method           TEXT NOT NULL,
    query_param_keys TEXT DEFAULT '',
    body_schema_hash TEXT DEFAULT '',
    content_type     TEXT DEFAULT '',
    sample_request   TEXT DEFAULT '',
    sample_response  TEXT DEFAULT '',
    sig_key          TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    session_id           TEXT DEFAULT '',
    business_name        TEXT DEFAULT '',
    business_description TEXT DEFAULT '',
    function_name        TEXT DEFAULT '',
    sensitivity          TEXT DEFAULT '',
    auth_mechanism       TEXT DEFAULT '',
    analysis_context     TEXT DEFAULT '',
    penetration_priority INTEGER DEFAULT 0,
    risk_level           TEXT DEFAULT '',
    final_summary        TEXT DEFAULT '',
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    analyzed_at      DATETIME DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_traffic_analysis_host ON traffic_analysis(host);
CREATE INDEX IF NOT EXISTS idx_traffic_analysis_status ON traffic_analysis(status);

CREATE TABLE IF NOT EXISTS ai_statistics (
    id                      INTEGER PRIMARY KEY,
    total_requests          BIGINT DEFAULT 0,
    success_count           BIGINT DEFAULT 0,
    failure_count           BIGINT DEFAULT 0,
    timeout_count           BIGINT DEFAULT 0,
    llm_error_count         BIGINT DEFAULT 0,
    parse_error_count       BIGINT DEFAULT 0,
    rate_limited_count      BIGINT DEFAULT 0,
    total_prompt_tokens     BIGINT DEFAULT 0,
    total_completion_tokens BIGINT DEFAULT 0,
    total_tokens            BIGINT DEFAULT 0,
    prompt_cached_tokens    BIGINT DEFAULT 0,
    turn_count              BIGINT DEFAULT 0,
    updated_at               DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO ai_statistics (id) VALUES (1);
`

// PostgreSQL 建表语句
const postgresSchema = `
CREATE TABLE IF NOT EXISTS traffic_analysis (
    id               SERIAL PRIMARY KEY,
    host             TEXT NOT NULL,
    path             TEXT NOT NULL,
    method           TEXT NOT NULL,
    query_param_keys TEXT DEFAULT '',
    body_schema_hash TEXT DEFAULT '',
    content_type     TEXT DEFAULT '',
    sample_request   TEXT DEFAULT '',
    sample_response  TEXT DEFAULT '',
    sig_key          TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    session_id           TEXT DEFAULT '',
    business_name        TEXT DEFAULT '',
    business_description TEXT DEFAULT '',
    function_name        TEXT DEFAULT '',
    sensitivity          TEXT DEFAULT '',
    auth_mechanism       TEXT DEFAULT '',
    analysis_context     TEXT DEFAULT '',
    penetration_priority INTEGER DEFAULT 0,
    risk_level           TEXT DEFAULT '',
    final_summary        TEXT DEFAULT '',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    analyzed_at      TIMESTAMP DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_traffic_analysis_host ON traffic_analysis(host);
CREATE INDEX IF NOT EXISTS idx_traffic_analysis_status ON traffic_analysis(status);

CREATE TABLE IF NOT EXISTS ai_statistics (
    id                      INTEGER PRIMARY KEY,
    total_requests          BIGINT DEFAULT 0,
    success_count           BIGINT DEFAULT 0,
    failure_count           BIGINT DEFAULT 0,
    timeout_count           BIGINT DEFAULT 0,
    llm_error_count         BIGINT DEFAULT 0,
    parse_error_count       BIGINT DEFAULT 0,
    rate_limited_count      BIGINT DEFAULT 0,
    total_prompt_tokens     BIGINT DEFAULT 0,
    total_completion_tokens BIGINT DEFAULT 0,
    total_tokens            BIGINT DEFAULT 0,
    prompt_cached_tokens    BIGINT DEFAULT 0,
    turn_count              BIGINT DEFAULT 0,
    updated_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO ai_statistics (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
`
