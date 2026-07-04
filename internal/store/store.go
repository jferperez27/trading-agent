// Package store owns persistence: opening the SQLite database, running
// migrations on startup, and reading/writing sessions, trades, and analyses.
package store

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
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

// newUID returns a random 16-byte hex string used as a session's unique id.
func newUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating uid: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// CreateSession inserts a new open session and returns its ID. The session's
// UID is generated here.
func (s *Store) CreateSession(p SessionParams) (int64, error) {
	uid, err := newUID()
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(timeLayout)
	res, err := s.db.Exec(`
		INSERT INTO sessions
		    (uid, name, market, started_at, ended_at, instrument, initial_balance, notes)
		VALUES (?, ?, ?, ?, NULL, ?, ?, ?)`,
		uid, p.Name, p.Market, now, p.Instrument, p.InitialBalance, p.Notes)
	if err != nil {
		return 0, fmt.Errorf("inserting session: %w", err)
	}
	return res.LastInsertId()
}

// CloseSession sets ended_at on an open session and generates + stores the
// close-time metadata JSON (duration, aggregate stats, final balance). It
// returns an error if the session does not exist or is already closed.
func (s *Store) CloseSession(id int64) error {
	sess, err := s.GetSession(id)
	if err != nil {
		return err
	}
	if sess.EndedAt != nil {
		return fmt.Errorf("session %d is already closed", id)
	}

	stats, err := s.SessionStats(id)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	meta := ClosedMeta{
		ClosedAt:        now.Format(timeLayout),
		DurationSeconds: int64(now.Sub(sess.StartedAt).Seconds()),
		TradeCount:      stats.TradeCount,
		Wins:            stats.Wins,
		Losses:          stats.Losses,
		TotalPnLPoints:  stats.TotalPnLPoints,
		TotalPnLCash:    stats.TotalPnLCash,
		WinRate:         stats.WinRate,
		AvgRMultiple:    stats.AvgRMultiple,
		FinalBalance:    stats.CurrentBalance,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling closed_meta for session %d: %w", id, err)
	}

	if _, err := s.db.Exec(`UPDATE sessions SET ended_at = ?, closed_meta = ? WHERE id = ?`,
		now.Format(timeLayout), string(metaJSON), id); err != nil {
		return fmt.Errorf("closing session %d: %w", id, err)
	}
	return nil
}

// SessionStats computes the live aggregates for a session from its trades.
func (s *Store) SessionStats(id int64) (Stats, error) {
	sess, err := s.GetSession(id)
	if err != nil {
		return Stats{}, err
	}
	trades, err := s.GetSessionTrades(id)
	if err != nil {
		return Stats{}, err
	}
	return ComputeStats(sess.InitialBalance, trades), nil
}

const sessionColumns = `id, uid, name, market, started_at, ended_at, instrument,
	initial_balance, notes, closed_meta`

// scanSession scans one session row (in sessionColumns order) from any
// row-scanner (sql.Row or sql.Rows).
func scanSession(scan func(...any) error) (Session, error) {
	var (
		sess       Session
		startedAt  string
		endedAt    sql.NullString
		closedMeta sql.NullString
	)
	err := scan(&sess.ID, &sess.UID, &sess.Name, &sess.Market, &startedAt, &endedAt,
		&sess.Instrument, &sess.InitialBalance, &sess.Notes, &closedMeta)
	if err != nil {
		return Session{}, err
	}
	sess.StartedAt = parseTime(startedAt)
	if endedAt.Valid {
		t := parseTime(endedAt.String)
		sess.EndedAt = &t
	}
	if closedMeta.Valid {
		sess.ClosedMeta = closedMeta.String
	}
	return sess, nil
}

// GetSession loads a single session by ID.
func (s *Store) GetSession(id int64) (Session, error) {
	row := s.db.QueryRow(`SELECT `+sessionColumns+` FROM sessions WHERE id = ?`, id)
	sess, err := scanSession(row.Scan)
	if err == sql.ErrNoRows {
		return Session{}, fmt.Errorf("session %d does not exist", id)
	}
	if err != nil {
		return Session{}, fmt.Errorf("loading session %d: %w", id, err)
	}
	return sess, nil
}

// ListSessions returns all sessions (newest first) with trade aggregates.
func (s *Store) ListSessions() ([]SessionListRow, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.uid, s.name, s.market, s.started_at, s.ended_at, s.instrument,
		       s.initial_balance, s.notes, s.closed_meta,
		       COUNT(t.id) AS trade_count,
		       COALESCE(SUM(t.pnl_cash), 0) AS total_pnl_cash
		FROM sessions s
		LEFT JOIN trades t ON t.session_id = s.id
		GROUP BY s.id
		ORDER BY s.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var out []SessionListRow
	for rows.Next() {
		var (
			r          SessionListRow
			startedAt  string
			endedAt    sql.NullString
			closedMeta sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.UID, &r.Name, &r.Market, &startedAt, &endedAt,
			&r.Instrument, &r.InitialBalance, &r.Notes, &closedMeta,
			&r.TradeCount, &r.TotalPnLCash); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		r.StartedAt = parseTime(startedAt)
		if endedAt.Valid {
			t := parseTime(endedAt.String)
			r.EndedAt = &t
		}
		if closedMeta.Valid {
			r.ClosedMeta = closedMeta.String
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

// InsertTrade computes pnl, r_multiple, and pnl_cash, inserts the trade, and
// returns its ID. A zero/negative Size defaults to 1 (pnl_cash == pnl points).
// The screenshot path is stored separately via SetTradeScreenshot once the
// trade ID is known (the filename embeds the trade ID).
func (s *Store) InsertTrade(t Trade) (int64, error) {
	if t.Size <= 0 {
		t.Size = 1
	}
	pnl, r := ComputeMetrics(t.Direction, t.EntryPrice, t.ExitPrice, t.StopLoss)
	pnlCash := pnl * t.Size
	now := time.Now().UTC().Format(timeLayout)
	res, err := s.db.Exec(`
		INSERT INTO trades
		    (session_id, entry_price, exit_price, stop_loss, size, direction, setup_type,
		     pnl, pnl_cash, r_multiple, screenshot_path, leak_tags, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?)`,
		t.SessionID, t.EntryPrice, t.ExitPrice, t.StopLoss, t.Size, t.Direction, t.SetupType,
		pnl, pnlCash, r, encodeLeakTags(t.LeakTags), t.Notes, now)
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
		SELECT id, session_id, entry_price, exit_price, stop_loss, size, direction, setup_type,
		       pnl, pnl_cash, r_multiple, screenshot_path, leak_tags, notes, created_at
		FROM trades WHERE id = ?`, id).
		Scan(&t.ID, &t.SessionID, &t.EntryPrice, &t.ExitPrice, &t.StopLoss, &t.Size, &t.Direction,
			&t.SetupType, &t.PnL, &t.PnLCash, &t.RMultiple, &screenshot, &leakTags, &t.Notes, &createdAt)
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
		SELECT id, session_id, entry_price, exit_price, stop_loss, size, direction, setup_type,
		       pnl, pnl_cash, r_multiple, screenshot_path, leak_tags, notes, created_at
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
			&t.Size, &t.Direction, &t.SetupType, &t.PnL, &t.PnLCash, &t.RMultiple,
			&screenshot, &leakTags, &t.Notes, &createdAt); err != nil {
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

func parseTime(s string) time.Time {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
