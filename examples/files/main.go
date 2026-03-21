// Command minergate is the MinerGate Dashboard GUI application.
//
// It connects to one or more Antminer-compatible ASIC miners via their
// HTTP/CGI interface (same endpoints used by the Bitmain dashboard.html),
// displays live statistics, and renders an interactive hashrate history chart
// using go-echarts served from an in-process HTTP server.
//
// Usage:
//
//	minergate [-config /path/to/config.json]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	"minergate/internal/config"
	"minergate/internal/gui"
)

const appID = "com.minergate.dashboard"

func main() {
	// ── CLI flags ────────────────────────────────────────────────────────────
	configPath := flag.String(
		"config", "",
		"Path to config.json (optional; overrides the default ~/.config/minergate/config.json)",
	)
	flag.Parse()

	// ── Load configuration ───────────────────────────────────────────────────
	var (
		cfg *config.Config
		err error
	)
	if *configPath != "" {
		cfg, err = loadConfigFromPath(*configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		log.Fatalf("minergate: config error: %v", err)
	}

	// Ensure the data directory (for CSV files) exists.
	if err := cfg.EnsureDataDir(); err != nil {
		log.Printf("minergate: warning: cannot create data dir %s: %v", cfg.DataDir, err)
	}

	// ── Fyne application ─────────────────────────────────────────────────────
	a := app.NewWithID(appID)
	a.Settings().SetTheme(resolveTheme(cfg.Theme))

	win := a.NewWindow("MinerGate Dashboard")
	win.SetMaster()

	// ── Dashboard controller ─────────────────────────────────────────────────
	dash := gui.NewDashboard(a, win, cfg)

	// Ensure clean shutdown on window close.
	win.SetOnClosed(func() {
		dash.Stop()
	})

	win.ShowAndRun()
}

// loadConfigFromPath reads a JSON config file from an explicit filesystem path.
// Unknown fields are silently ignored; missing fields keep their default values.
func loadConfigFromPath(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	cfg := config.Default() // start with safe defaults
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return cfg, nil
}

// resolveTheme maps a theme name string to a Fyne built-in theme.
// "light" → light theme; anything else (including "dark") → dark theme.
func resolveTheme(name string) fyne.Theme {
	if name == "light" {
		return theme.LightTheme()
	}
	return theme.DarkTheme()
}
