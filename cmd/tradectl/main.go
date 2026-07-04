// Command tradectl is a personal FxReplay backtest session analyzer and
// documentor: it logs trades, stores reference screenshots, and runs
// Claude-powered analysis on individual trades and full sessions.
//
// This is a thin entrypoint; all command wiring lives in internal/cli.
package main

import "tradectl/internal/cli"

func main() {
	cli.Execute()
}
