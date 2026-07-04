# tradectl

A personal, local-only terminal app for logging and reviewing
[FxReplay](https://fxreplay.com) backtest sessions. Start a session, log trades
against known "leak" categories with reference screenshots, and watch your
session stats — balance, P/L, win rate — update live as you trade.

Backtest-only by design: no live market data, no auth, no multi-user concerns.

## Requirements

- Go 1.26+

## Install (Global Installation - Recommended)

```bash
go install ./cmd/tradectl
```

This drops a self-contained binary (pure-Go SQLite, no CGO) into `$HOME/go/bin`.
Make sure that's on your `PATH` — add this to `~/.zshrc` (or your shell's rc) if
it isn't:

```bash
export PATH="$PATH:$HOME/go/bin"
```

Then `tradectl` works from any directory.

## Install (Local Installation)
```bash
go build -o tradectl ./cmd/tradectl
```

Can be invoked using `./tradectl`

## Usage

### The app

```bash
tradectl        # from any directory — launches the full-screen interactive app
```

You land on the **sessions menu** and stay in the app until you quit:

| Screen | Keys |
|---|---|
| Sessions menu | `↑/↓` move · `enter` open · `n` new session · `c` close · `:` command · `q` quit |
| Session dashboard | `a` add trade · `c` close session · `:` command · `esc` back |
| Forms | `enter` next · `space` toggle (multi-select) · `esc` cancel |

- **New session** asks for name, market (futures/stocks/forex/crypto/other),
  instrument (e.g. NQ), initial money, and optional notes — then drops you
  straight into that session's dashboard.
- **The dashboard** shows a live stats header — current balance, P/L ($ and
  points), trades won/lost, win rate, avg R — that updates as you log each
  trade. After a trade is saved the form resets, ready for the next one.
- **Trades** capture direction, entry/exit/stop, size (contracts × point
  value), setup (ORB/FVG/other), leak tags, notes, and an optional screenshot.
  `pnl`, `pnl_cash`, and `r_multiple` are computed for you.
- **Closing a session** generates and stores metadata (duration, trade count,
  wins/losses, win rate, total P/L, final balance) and shows the summary.
- **`:` command bar** — quick scripting without leaving the app:
  `new` · `open <id>` · `close <id>` · `help` · `quit`.

### Subcommands (shell scripting)

```bash
tradectl sessions new                 # prompts: name, market, instrument, initial money, notes
tradectl sessions list                # id, name, market, instrument, started, ended, trades, P/L
tradectl sessions close <id>          # closes + generates the session metadata
tradectl log                          # standalone trade-logging form
```

## Configuration

On first run, `tradectl` creates `~/.tradectl/config.yaml`:

```yaml
data_dir: ~/.tradectl/data    # SQLite DB + screenshots live here (~ is expanded)
```

The data dir is a fixed location under your home directory so the app sees the
same data no matter where you launch it from.

## Data

Everything is stored under `data_dir` (default `~/.tradectl/data`):

- `tradectl.db` — SQLite database (`sessions`, `trades`)
- `screenshots/` — copied trade screenshots, referenced by relative path in the DB

Inspect directly any time with `sqlite3 ~/.tradectl/data/tradectl.db`.

## Project layout

```
cmd/tradectl/          # thin main entrypoint (calls cli.Execute)
internal/
  cli/                 # cobra command tree: root launches the TUI + sessions/log
  config/              # ~/.tradectl/config.yaml loading
  store/               # SQLite persistence, domain types, stats, migrations/, screenshots
  tui/                 # full-screen BubbleTea app: menu, dashboard, forms, command bar
tests/                 # test suite (black-box, exercises exported APIs)
```

Each `internal/` package owns a single concern; `cli` is orchestration only.

## Development

```bash
go test ./...     # runs the suite in tests/ (metrics, stats, store round-trip, command parser)
go vet ./...
```
