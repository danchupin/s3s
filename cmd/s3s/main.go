// Command s3s is a read-only, keyboard-driven TUI for browsing S3-compatible
// object storage (Ceph RGW, MinIO). It loads a kubectl-style config, resolves the
// active context, builds a read-only storage client, and runs the Bubble Tea UI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/logging"
	"github.com/danchupin/s3s/internal/storage"
	"github.com/danchupin/s3s/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "s3s:", err)
		os.Exit(1)
	}
}

func run() error {
	var ctxFlag, cfgPath string
	flag.StringVar(&ctxFlag, "context", "", "active context name (overrides $S3S_CONTEXT and current-context)")
	flag.StringVar(&cfgPath, "config", "", "path to config file (default: XDG ~/.config/s3s/config.yaml)")
	flag.Parse()

	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return fmt.Errorf("no config found at %s\n  create one — see specs/.../quickstart.md for the format", cfgPath)
		}
		return err
	}

	active := config.ActiveContextName(ctxFlag, os.Getenv(config.EnvContext), cfg.CurrentContext)
	if active == "" {
		return errors.New("no context selected: set current-context in config or pass --context")
	}

	// File logging only — the TUI owns the terminal (Constitution V).
	if logger, closer, lerr := logging.New(defaultLogPath(), slog.LevelInfo); lerr == nil {
		defer func() { _ = closer.Close() }()
		slog.SetDefault(logger)
		slog.Info("s3s start", "context", active, "config", cfgPath)
	}

	cc, err := cfg.ClientConfig(active)
	if err != nil {
		return err
	}
	store, err := storage.New(cc)
	if err != nil {
		return err
	}

	switchFn := func(name string) (storage.Storage, error) {
		c, cerr := cfg.ClientConfig(name)
		if cerr != nil {
			return nil, cerr
		}
		return storage.New(c)
	}

	model := ui.New(store, active, cfg.ContextNames(), switchFn)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// defaultLogPath resolves a writable log file path (XDG state dir, then temp).
func defaultLogPath() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "s3s", "s3s.log")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "s3s", "s3s.log")
	}
	return filepath.Join(os.TempDir(), "s3s.log")
}
