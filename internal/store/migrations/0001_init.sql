-- Sprint 1 schema: sessions, trades, analyses.
-- See Documentation/00-MASTER.md for the authoritative column spec.

CREATE TABLE sessions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at  DATETIME NOT NULL,
    ended_at    DATETIME,            -- nullable until the session is closed
    instrument  TEXT NOT NULL,
    notes       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE trades (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id      INTEGER NOT NULL REFERENCES sessions(id),
    entry_price     REAL NOT NULL,
    exit_price      REAL NOT NULL,
    stop_loss       REAL NOT NULL,
    direction       TEXT NOT NULL,   -- 'long' / 'short'
    setup_type      TEXT NOT NULL,   -- 'ORB' / 'FVG' / 'other'
    pnl             REAL NOT NULL,
    r_multiple      REAL NOT NULL,
    screenshot_path TEXT,            -- nullable, relative to data_dir
    leak_tags       TEXT NOT NULL DEFAULT '', -- comma-separated enum values
    notes           TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL
);

CREATE INDEX idx_trades_session_id ON trades(session_id);

CREATE TABLE analyses (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    scope         TEXT NOT NULL,     -- 'trade' / 'session'
    target_id     INTEGER NOT NULL,  -- trades.id or sessions.id depending on scope
    model_used    TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    cost_usd      REAL NOT NULL,
    result_text   TEXT NOT NULL,
    summary       TEXT,              -- compact structured JSON summary (nullable)
    created_at    DATETIME NOT NULL
);

CREATE INDEX idx_analyses_scope_target ON analyses(scope, target_id);
