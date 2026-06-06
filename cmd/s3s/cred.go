package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/secret"
)

// runCred handles `s3s cred <set|rotate|rm> <context>` — manage a context's secret in
// the OS keystore ONLY (never the config file, 005 FR-037). Writing to the keystore is
// not an S3 operation, so it does not breach the read-only guarantee.
func runCred(args []string) error {
	fs := flag.NewFlagSet("cred", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "path to config file (default: XDG ~/.config/s3s/config.yaml)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 2 {
		return errors.New("usage: s3s cred <set|rotate|rm> <context>")
	}
	action, ctxName := rest[0], rest[1]

	path := *cfgPath
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	account, err := cfg.KeychainAccount(ctxName)
	if err != nil {
		return err
	}

	switch action {
	case "set", "rotate":
		sec, perr := secret.Prompt(fmt.Sprintf("secret for context %q (no echo): ", ctxName))
		if perr != nil {
			return perr
		}
		if serr := secret.StoreKeychain(account, sec); serr != nil {
			return serr
		}
		fmt.Printf("stored secret for context %q in the OS keystore\n", ctxName)
	case "rm":
		if serr := secret.RemoveKeychain(account); serr != nil {
			return serr
		}
		fmt.Printf("removed secret for context %q from the OS keystore\n", ctxName)
	default:
		return fmt.Errorf("unknown cred action %q (use set|rotate|rm)", action)
	}
	return nil
}

// offerSaveToKeychain prompts (post a successful interactive secret prompt) to persist
// the secret into the OS keystore for next time (005 FR-038). Best-effort; failures are
// reported but non-fatal.
func offerSaveToKeychain(cfg *config.Config, ctxName, sec string) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	account, err := cfg.KeychainAccount(ctxName)
	if err != nil {
		return
	}
	fmt.Fprint(os.Stderr, "save this secret to the OS keystore for next time? [y/N]: ")
	var ans string
	_, _ = fmt.Fscanln(os.Stdin, &ans)
	if ans == "y" || ans == "Y" {
		if serr := secret.StoreKeychain(account, sec); serr != nil {
			fmt.Fprintf(os.Stderr, "could not save to keystore: %v\n", serr)
			return
		}
		fmt.Fprintln(os.Stderr, "saved.")
	}
}
