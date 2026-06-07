// Command s3s is a read-only, keyboard-driven TUI for browsing S3-compatible
// object storage (Ceph RGW, MinIO). It loads a kubectl-style config, resolves the
// active context, builds a read-only storage client, and runs the Bubble Tea UI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/logging"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/secret"
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
	// Subcommand: `s3s cred <set|rotate|rm> <context>` — keystore secret management.
	if args := os.Args[1:]; len(args) >= 1 && args[0] == "cred" {
		return runCred(args[1:])
	}

	var ctxFlag, cfgPath string
	var showVersion, writeEnabled bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&writeEnabled, "write", false, "start in write mode (default: read-only); toggle at runtime with the write key; a context marked readonly stays protected")
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
	firstRun := false
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return err
		}
		// No config yet (009): start the TUI in the add-connection form instead of erroring.
		// An empty config bound to cfgPath lets the in-app form write the first connection.
		cfg = config.Empty(cfgPath)
		firstRun = true
	}
	// A config that exists but declares no contexts is the same "no connections yet" state.
	if len(cfg.ContextNames()) == 0 {
		firstRun = true
	}

	active := ""
	if !firstRun {
		// No flag / $S3S_CONTEXT / current-context selects a context. Rather than erroring
		// before the TUI starts, leave active empty: the app opens the connection manager so
		// the user picks one in the UI (it already receives the full context list).
		active = config.ActiveContextName(ctxFlag, os.Getenv(config.EnvContext), cfg.CurrentContext)
	}

	// Insecure-permissions warning (005 FR-040) — printed pre-TUI to stderr.
	if config.InsecurePerms(cfgPath) {
		fmt.Fprintf(os.Stderr, "s3s: warning — %s is group/world-readable; run: chmod 600 %s\n", cfgPath, cfgPath)
	}

	// File logging only — the TUI owns the terminal (Constitution V).
	if logger, closer, lerr := logging.New(defaultLogPath(), slog.LevelInfo); lerr == nil {
		defer func() { _ = closer.Close() }()
		slog.SetDefault(logger)
		slog.Info("s3s start", "context", active, "config", cfgPath)
	}

	// backendFrom assembles a ui.Backend from a resolved ClientConfig. The UI guards
	// the backend dynamically (runtime read-only↔write toggle, 005 US5), so it gets the
	// RAW store plus the context's hard lock. Writable carries the *initial* arm intent
	// (--write); ReadOnly carries the absolute readonly:true lock (FR-028/FR-031).
	backendFrom := func(name string, cc storage.ClientConfig) (ui.Backend, error) {
		cl, u, rerr := cfg.Resolve(name)
		if rerr != nil {
			return ui.Backend{}, rerr
		}
		st, serr := storage.New(cc)
		if serr != nil {
			return ui.Backend{}, serr
		}
		policy, perr := cfg.WriteModeFor(name, writeEnabled)
		if perr != nil {
			return ui.Backend{}, perr
		}
		userLabel := u.Name
		if u.Anonymous {
			userLabel += " (anonymous)"
		}
		return ui.Backend{
			Store: st, Cluster: cl.Name, User: userLabel,
			Endpoint: cl.Endpoint, Region: cl.Region,
			Writable: writeEnabled, ReadOnly: policy.ReadOnly,
			DownloadDir: cfg.DownloadDir,
		}, nil
	}

	// resolve resolves a context's secret NON-interactively (used on in-TUI context
	// switch, where the TUI owns the terminal — no prompting).
	resolve := func(name string) (ui.Backend, error) {
		cc, cerr := cfg.ClientConfig(context.Background(), name)
		if cerr != nil {
			return ui.Backend{}, cerr
		}
		return backendFrom(name, cc)
	}

	// The active context resolves at startup, where an interactive secure prompt IS
	// allowed (before the TUI starts — Constitution V, 005 R12). On a resolution
	// failure (e.g. an empty keystore) fall back to the prompt and offer to save. On
	// first run there is no active context — the add-connection form opens instead (009).
	var initial ui.Backend
	if !firstRun && active != "" {
		initial, err = resolve(active)
		if err != nil {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return err
			}
			fmt.Fprintf(os.Stderr, "s3s: %v\n", err)
			sec, perr := secret.Prompt(fmt.Sprintf("secret for context %q: ", active))
			if perr != nil {
				return perr
			}
			cc, cerr := cfg.ClientConfigWithSecret(active, sec)
			if cerr != nil {
				return cerr
			}
			if initial, err = backendFrom(active, cc); err != nil {
				return err
			}
			offerSaveToKeychain(cfg, active, sec)
		}
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
	// connSeam lets the UI add a cluster connection from inside the app (006 US4): it
	// tests reachability and persists the triple + keychain secret, mutating the live cfg
	// so the new context is switchable without a restart.
	model := ui.New(initial, active, cfg.ContextNames(), resolve, connSeam{cfg: cfg}, imgProto)
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
