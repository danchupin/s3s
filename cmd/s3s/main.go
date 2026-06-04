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
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
	"github.com/danchupin/s3s/internal/ui"
)

// version is the build version, overridable via
// -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	ui.Version = version
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "s3s:", err)
		os.Exit(1)
	}
}

func run() error {
	// Subcommand: `s3s config init` — interactive config generator.
	if args := os.Args[1:]; len(args) >= 2 && args[0] == "config" && args[1] == "init" {
		return runConfigInit(args[2:])
	}

	var ctxFlag, cfgPath string
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&ctxFlag, "context", "", "active context name (overrides $S3S_CONTEXT and current-context)")
	flag.StringVar(&cfgPath, "config", "", "path to config file (default: XDG ~/.config/s3s/config.yaml)")
	flag.Parse()

	if showVersion {
		fmt.Println("s3s", version)
		return nil
	}

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

	// resolve builds the storage client + header metadata for a context.
	resolve := func(name string) (ui.Backend, error) {
		cl, u, rerr := cfg.Resolve(name)
		if rerr != nil {
			return ui.Backend{}, rerr
		}
		cc, cerr := cfg.ClientConfig(name)
		if cerr != nil {
			return ui.Backend{}, cerr
		}
		st, serr := storage.New(cc)
		if serr != nil {
			return ui.Backend{}, serr
		}
		userLabel := u.Name
		if u.Anonymous {
			userLabel += " (anonymous)"
		}
		return ui.Backend{
			Store:    st,
			Cluster:  cl.Name,
			User:     userLabel,
			Endpoint: cl.Endpoint,
			Region:   cl.Region,
		}, nil
	}

	initial, err := resolve(active)
	if err != nil {
		return err
	}

	// Image rendering. Default to ANSI half-block: Bubble Tea v2's cell renderer
	// only understands cell content (incl. truecolor), so terminal graphics
	// protocols (kitty/iTerm2) are stripped and never reach the terminal. Half-
	// block is lower-resolution but actually renders. The protocol paths are an
	// explicit, experimental opt-in for setups where they survive.
	imgProto := preview.ProtoNone
	switch os.Getenv("S3S_IMAGE_PROTOCOL") {
	case "kitty":
		imgProto = preview.ProtoKitty
	case "iterm2":
		imgProto = preview.ProtoITerm2
	case "auto":
		imgProto = preview.DetectProtocol(os.Getenv)
	}
	model := ui.New(initial, active, cfg.ContextNames(), resolve, imgProto)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// runConfigInit handles `s3s config init [--config <path>]` — the interactive
// config generator. Writing a local config file is not an S3 operation, so this
// does not breach the read-only guarantee (FR-019).
func runConfigInit(args []string) error {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "path to write the config file (default: XDG ~/.config/s3s/config.yaml)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := *cfgPath
	if path == "" {
		path = config.DefaultPath()
	}
	return config.RunInit(os.Stdin, os.Stdout, path)
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
