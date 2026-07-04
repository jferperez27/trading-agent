# tradectl

A personal, local-only CLI for analyzing and documenting
[FxReplay](https://fxreplay.com) backtest sessions. Log trades against known
"leak" categories, attach reference screenshots, and get honest, Claude-powered
critique of individual trades and full sessions — with every API call's
token/cost tracked.

Backtest-only by design: no live market data, no auth, no multi-user concerns.

> **Status:** Sprint 1 (core data layer, CLI logging, Claude integration) is
> complete. Sprints 2 (cross-session insight + read-only API) and 3 (Next.js
> dashboard) are specced in `Documentation/` but not yet built.

## Requirements

- Go 1.26+
- An Anthropic API key (only needed for `analyze`)

## Install

```bash
go build -o tradectl ./cmd/tradectl
```

The binary is self-contained (pure-Go SQLite, no CGO).

## Configuration

On first run, `tradectl` creates `~/.tradectl/config.yaml` with defaults:

```yaml
anthropic_api_key_env: ANTHROPIC_API_KEY   # env var holding the key (never stored here)
default_model_trade: claude-haiku-4-5-20251001
default_model_session: claude-sonnet-4-6
longitudinal_context_count: 3              # used in Sprint 2
monthly_cost_alert_threshold_usd: 10.00    # used in Sprint 3
data_dir: ./data                           # SQLite DB + screenshots live here
```

Export your key before running `analyze`:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

## Usage

```bash
# Sessions
tradectl sessions new                 # prompts for instrument (e.g. NQ) and optional notes
tradectl sessions list                # id, instrument, started, ended, trade count
tradectl sessions close <id>

# Log a trade (interactive form)
tradectl log
#   → session (defaults to the latest open one), direction, entry/exit/stop,
#     setup (ORB/FVG/other), leak tags (multi-select), notes, optional screenshot.
#   pnl and r_multiple are computed for you.

# Analyze (logs cost/tokens to the analyses table)
tradectl analyze --trade <id>                 # quick per-trade leak check (Haiku)
tradectl analyze --session <id>               # full session critique (Sonnet)
tradectl analyze --session <id> --model claude-haiku-4-5-20251001   # override the model
```

## Data

Everything is stored under `data_dir` (default `./data`):

- `tradectl.db` — SQLite database (`sessions`, `trades`, `analyses`)
- `screenshots/` — copied trade screenshots, referenced by relative path in the DB

Inspect directly any time with `sqlite3 data/tradectl.db`.

## Project layout

```
cmd/tradectl/          # thin main entrypoint (calls cli.Execute)
internal/
  cli/                 # cobra command tree: cli.go (root) + sessions/log/analyze
  config/              # ~/.tradectl/config.yaml loading + API-key resolution
  store/               # SQLite persistence, domain types, migrations/, screenshots
  cost/                # per-model rate card + cost computation
  claude/              # Anthropic SDK wrapper + <verdict> parsing
  analysis/            # structured session summary (stats from trades + verdict)
  tui/                 # BubbleTea interactive log form
Documentation/         # cross-sprint spec (00-MASTER) + per-sprint scopes
```

Each `internal/` package owns a single concern; `cli` is orchestration only. See
`CLAUDE.md` for the architecture and data-flow detail.

## Development

```bash
go test ./...     # unit tests (metrics, cost/rate-card, store round-trip, summary, verdict)
go vet ./...
```

See `CLAUDE.md` for architecture and `Documentation/` for the full spec.
