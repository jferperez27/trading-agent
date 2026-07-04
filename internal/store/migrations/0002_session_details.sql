-- Session details + trade sizing.
-- sessions: name, market category, starting balance, unique id, and the
-- close-time metadata JSON (populated when the session is closed).
-- trades: size multiplier (contracts x point value) and cash pnl (pnl x size).

ALTER TABLE sessions ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN market TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN initial_balance REAL NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN uid TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN closed_meta TEXT;

ALTER TABLE trades ADD COLUMN size REAL NOT NULL DEFAULT 1;
ALTER TABLE trades ADD COLUMN pnl_cash REAL NOT NULL DEFAULT 0;

-- Backfill: pre-0002 trades get cash pnl at the default size of 1.
UPDATE trades SET pnl_cash = pnl * size;
