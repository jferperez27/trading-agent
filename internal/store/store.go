// Package store owns persistence: opening the SQLite database, running
// migrations on startup, and reading/writing sessions, trades, and analyses.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, keeps the binary CGO-free
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DBFileName is the SQLite database filename under the data directory.
const DBFileName = "tradectl.db"

// ScreenshotsDir is the screenshots subdirectory under the data directory.
const ScreenshotsDir = "screenshots"

const timeLayout = time.RFC3339

// Store wraps the database connection and knows the data directory so it can
// resolve relative screenshot paths.
type Store struct {
	db      *sql.DB
	dataDir string
}

// Open ensures the data directory exists, opens (creating if needed) the
// SQLite database at dataDir/tradectl.db, and runs any pending migrations.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir %s: %w", dataDir, err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, ScreenshotsDir), 0o755); err != nil {
		return nil, fmt.Errorf("creating screenshots dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, DBFileName)
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	s := &Store{db: db, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// DataDir returns the configured data directory.
func (s *Store) DataDir() string { return s.dataDir }

// migrate applies any embedded migration files not yet recorded in
// schema_migrations, in lexical filename order, each in its own transaction.
func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE name = ?`, name).Scan(&exists); err != nil {
			return fmt.Errorf("checking migration %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("applying migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`,
			name, time.Now().UTC().Format(timeLayout)); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// --- Sessions ---

// CreateSession inserts a new open session and returns its ID.
func (s *Store) CreateSession(instrument, notes string) (int64, error) {
	now := time.Now().UTC().Format(timeLayout)
	res, err := s.db.Exec(
		`INSERT INTO sessions (started_at, ended_at, instrument, notes) VALUES (?, NULL, ?, ?)`,
		now, instrument, notes)
	if err != nil {
		return 0, fmt.Errorf("inserting session: %w", err)
	}
	return res.LastInsertId()
}

// CloseSession sets ended_at on an open session. It returns an error if the
// session does not exist or is already closed.
func (s *Store) CloseSession(id int64) error {
	var endedAt sql.NullString
	err := s.db.QueryRow(`SELECT ended_at FROM sessions WHERE id = ?`, id).Scan(&endedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("session %d does not exist", id)
	}
	if err != nil {
		return fmt.Errorf("looking up session %d: %w", id, err)
	}
	if endedAt.Valid {
		return fmt.Errorf("session %d is already closed", id)
	}
	now := time.Now().UTC().Format(timeLayout)
	if _, err := s.db.Exec(`UPDATE sessions SET ended_at = ? WHERE id = ?`, now, id); err != nil {
		return fmt.Errorf("closing session %d: %w", id, err)
	}
	return nil
}

// GetSession loads a single session by ID.
func (s *Store) GetSession(id int64) (Session, error) {
	var (
		sess      Session
		startedAt string
		endedAt   sql.NullString
	)
	err := s.db.QueryRow(
		`SELECT id, started_at, ended_at, instrument, notes FROM sessions WHERE id = ?`, id).
		Scan(&sess.ID, &startedAt, &endedAt, &sess.Instrument, &sess.Notes)
	if err == sql.ErrNoRows {
		return Session{}, fmt.Errorf("session %d does not exist", id)
	}
	if err != nil {
		return Session{}, fmt.Errorf("loading session %d: %w", id, err)
	}
	sess.StartedAt = parseTime(startedAt)
	if endedAt.Valid {
		t := parseTime(endedAt.String)
		sess.EndedAt = &t
	}
	return sess, nil
}

// ListSessions returns all sessions (newest first) with their trade counts.
func (s *Store) ListSessions() ([]SessionListRow, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.started_at, s.ended_at, s.instrument, s.notes,
		       (SELECT COUNT(1) FROM trades t WHERE t.session_id = s.id) AS trade_count
		FROM sessions s
		ORDER BY s.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var out []SessionListRow
	for rows.Next() {
		var (
			r         SessionListRow
			startedAt string
			endedAt   sql.NullString
		)
		if err := rows.Scan(&r.ID, &startedAt, &endedAt, &r.Instrument, &r.Notes, &r.TradeCount); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		r.StartedAt = parseTime(startedAt)
		if endedAt.Valid {
			t := parseTime(endedAt.String)
			r.EndedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestOpenSessionID returns the ID of the most recently started session that
// is still open, or 0 if none. The boolean reports whether one was found.
func (s *Store) LatestOpenSessionID() (int64, bool, error) {
	var id int64
	err := s.db.QueryRow(
		`SELECT id FROM sessions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("finding latest open session: %w", err)
	}
	return id, true, nil
}

// --- Trades ---

// InsertTrade computes pnl/r_multiple, inserts the trade, and returns its ID.
// The screenshot path is stored separately via SetTradeScreenshot once the
// trade ID is known (the filename embeds the trade ID).
func (s *Store) InsertTrade(t Trade) (int64, error) {
	pnl, r := ComputeMetrics(t.Direction, t.EntryPrice, t.ExitPrice, t.StopLoss)
	now := time.Now().UTC().Format(timeLayout)
	res, err := s.db.Exec(`
		INSERT INTO trades
		    (session_id, entry_price, exit_price, stop_loss, direction, setup_type,
		     pnl, r_multiple, screenshot_path, leak_tags, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?)`,
		t.SessionID, t.EntryPrice, t.ExitPrice, t.StopLoss, t.Direction, t.SetupType,
		pnl, r, encodeLeakTags(t.LeakTags), t.Notes, now)
	if err != nil {
		return 0, fmt.Errorf("inserting trade: %w", err)
	}
	return res.LastInsertId()
}

// SetTradeScreenshot stores the relative screenshot path for a trade.
func (s *Store) SetTradeScreenshot(tradeID int64, relPath string) error {
	if _, err := s.db.Exec(`UPDATE trades SET screenshot_path = ? WHERE id = ?`, relPath, tradeID); err != nil {
		return fmt.Errorf("setting screenshot for trade %d: %w", tradeID, err)
	}
	return nil
}

// SessionExists reports whether a session with the given ID exists.
func (s *Store) SessionExists(id int64) (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM sessions WHERE id = ?`, id).Scan(&n); err != nil {
		return false, fmt.Errorf("checking session %d: %w", id, err)
	}
	return n > 0, nil
}

// GetTrade loads a single trade by ID.
func (s *Store) GetTrade(id int64) (Trade, error) {
	var (
		t          Trade
		screenshot sql.NullString
		leakTags   string
		createdAt  string
	)
	err := s.db.QueryRow(`
		SELECT id, session_id, entry_price, exit_price, stop_loss, direction, setup_type,
		       pnl, r_multiple, screenshot_path, leak_tags, notes, created_at
		FROM trades WHERE id = ?`, id).
		Scan(&t.ID, &t.SessionID, &t.EntryPrice, &t.ExitPrice, &t.StopLoss, &t.Direction,
			&t.SetupType, &t.PnL, &t.RMultiple, &screenshot, &leakTags, &t.Notes, &createdAt)
	if err == sql.ErrNoRows {
		return Trade{}, fmt.Errorf("trade %d does not exist", id)
	}
	if err != nil {
		return Trade{}, fmt.Errorf("loading trade %d: %w", id, err)
	}
	if screenshot.Valid {
		t.ScreenshotPath = screenshot.String
	}
	t.LeakTags = decodeLeakTags(leakTags)
	t.CreatedAt = parseTime(createdAt)
	return t, nil
}

// GetSessionTrades loads all trades for a session, oldest first.
func (s *Store) GetSessionTrades(sessionID int64) ([]Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, entry_price, exit_price, stop_loss, direction, setup_type,
		       pnl, r_multiple, screenshot_path, leak_tags, notes, created_at
		FROM trades WHERE session_id = ? ORDER BY id ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("loading trades for session %d: %w", sessionID, err)
	}
	defer rows.Close()

	var out []Trade
	for rows.Next() {
		var (
			t          Trade
			screenshot sql.NullString
			leakTags   string
			createdAt  string
		)
		if err := rows.Scan(&t.ID, &t.SessionID, &t.EntryPrice, &t.ExitPrice, &t.StopLoss,
			&t.Direction, &t.SetupType, &t.PnL, &t.RMultiple, &screenshot, &leakTags,
			&t.Notes, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning trade: %w", err)
		}
		if screenshot.Valid {
			t.ScreenshotPath = screenshot.String
		}
		t.LeakTags = decodeLeakTags(leakTags)
		t.CreatedAt = parseTime(createdAt)
		out = append(out, t)
	}
	return out, rows.Err()
}

// --- Analyses ---

// InsertAnalysis records a Claude analysis call and returns its ID. An empty
// Summary is stored as SQL NULL.
func (s *Store) InsertAnalysis(a Analysis) (int64, error) {
	now := time.Now().UTC().Format(timeLayout)
	var summary any
	if a.Summary != "" {
		summary = a.Summary
	}
	res, err := s.db.Exec(`
		INSERT INTO analyses
		    (scope, target_id, model_used, input_tokens, output_tokens, cost_usd,
		     result_text, summary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Scope, a.TargetID, a.ModelUsed, a.InputTokens, a.OutputTokens, a.CostUSD,
		a.ResultText, summary, now)
	if err != nil {
		return 0, fmt.Errorf("inserting analysis: %w", err)
	}
	return res.LastInsertId()
}

func parseTime(s string) time.Time {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
