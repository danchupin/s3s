package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/danchupin/s3s/internal/logging"
	"github.com/danchupin/s3s/internal/secret"
	yaml "go.yaml.in/yaml/v3"
)

// fp writes a formatted prompt/notice to the wizard's output, ignoring write
// errors (terminal output failures are not actionable mid-wizard).
func fp(out io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(out, format, a...)
}

// Marshal serializes a config to YAML. Secret fields hold ${ENV} references
// produced by the wizard, so no real credential is ever written to disk.
func Marshal(c *Config) ([]byte, error) {
	return yaml.Marshal(c)
}

// loadRaw reads and parses a config WITHOUT env resolution or validation, for
// editing. A missing file yields an empty Config (apiVersion seeded).
func loadRaw(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{APIVersion: "s3s/v1"}, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("%w: parse error", ErrInvalid)
	}
	if c.APIVersion == "" {
		c.APIVersion = "s3s/v1"
	}
	return &c, nil
}

// Upsert merges a cluster, user, and context into the config, replacing any
// existing entry with the same name. When setCurrent is true the context becomes
// current-context.
func (c *Config) Upsert(cl Cluster, u User, cx Context, setCurrent bool) {
	c.Clusters = upsertCluster(c.Clusters, cl)
	c.Users = upsertUser(c.Users, u)
	c.Contexts = upsertContext(c.Contexts, cx)
	if setCurrent || c.CurrentContext == "" {
		c.CurrentContext = cx.Name
	}
}

func upsertCluster(list []Cluster, v Cluster) []Cluster {
	for i := range list {
		if list[i].Name == v.Name {
			list[i] = v
			return list
		}
	}
	return append(list, v)
}

func upsertUser(list []User, v User) []User {
	for i := range list {
		if list[i].Name == v.Name {
			list[i] = v
			return list
		}
	}
	return append(list, v)
}

func upsertContext(list []Context, v Context) []Context {
	for i := range list {
		if list[i].Name == v.Name {
			list[i] = v
			return list
		}
	}
	return append(list, v)
}

// Save writes config bytes to path (0600), creating parent dirs.
func Save(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// EnvVarName derives the environment variable that will hold a context's secret,
// e.g. "prod-public" -> "S3S_PROD_PUBLIC_SECRET".
func EnvVarName(context string) string {
	var b strings.Builder
	b.WriteString("S3S_")
	for _, r := range strings.ToUpper(context) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	b.WriteString("_SECRET")
	return b.String()
}

// RunInit drives the interactive config generator: it prompts on out, reads
// answers from in, merges the result into the config at path (creating or
// extending it), and writes it back. Secrets are stored as ${ENV} references —
// the raw key is never written to disk (FR-005).
func RunInit(in io.Reader, out io.Writer, path string) error {
	sc := bufio.NewScanner(in)

	fp(out, "s3s config generator → %s\n\n", path)

	ctxName := ask(out, sc, "Context name", "")
	if ctxName == "" {
		return fmt.Errorf("%w: context name is required", ErrInvalid)
	}
	clusterName := ask(out, sc, "Cluster name", ctxName)
	endpoint := ask(out, sc, "Endpoint URL (e.g. http://127.0.0.1:9000)", "")
	region := ask(out, sc, "Region", DefaultRegion)
	pathStyle := askBool(out, sc, "Path-style addressing", true)
	tlsSkip := askBool(out, sc, "Skip TLS verification (https only)", false)
	anonymous := askBool(out, sc, "Anonymous access (public buckets)", false)

	cl := Cluster{
		Name:          clusterName,
		Endpoint:      endpoint,
		Region:        region,
		PathStyle:     pathStyle,
		TLSSkipVerify: tlsSkip,
	}

	userName := ctxName
	u := User{Name: userName}
	var envVar string
	if anonymous {
		u.Anonymous = true
	} else {
		// Credential source (005 US6): keychain stores the secret in the OS keystore;
		// cmd/awsProfile reference external sources; env keeps the ${ENV} reference.
		switch strings.ToLower(ask(out, sc, "Credential source [env/keychain/cmd/awsProfile]", "env")) {
		case "keychain":
			u.AccessKeyID = ask(out, sc, "Access key ID", "")
			u.Keychain = true
			sec, perr := secret.Prompt("Secret access key (no echo): ")
			if perr != nil {
				return perr
			}
			if serr := secret.StoreKeychain(userName, sec); serr != nil {
				return serr
			}
		case "cmd":
			u.AccessKeyID = ask(out, sc, "Access key ID", "")
			u.Command = ask(out, sc, "Command that prints the secret to stdout", "")
		case "awsprofile", "aws", "profile":
			u.AWSProfile = ask(out, sc, "AWS profile name", "default")
		default: // env — unchanged ${ENV} reference
			u.AccessKeyID = ask(out, sc, "Access key ID", "")
			envVar = EnvVarName(ctxName)
			u.SecretAccessKey = logging.Secret("${" + envVar + "}")
		}
	}

	cx := Context{Name: ctxName, Cluster: clusterName, User: userName}
	setCurrent := askBool(out, sc, "Make this the current context", true)

	cfg, err := loadRaw(path)
	if err != nil {
		return err
	}
	cfg.Upsert(cl, u, cx, setCurrent)
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := Save(path, data); err != nil {
		return err
	}

	fp(out, "\nWrote %s\n", path)
	if envVar != "" {
		fp(out, "\nSet the secret in your environment (it is NOT stored in the file):\n")
		fp(out, "  export %s=<your-secret-access-key>\n", envVar)
	}
	fp(out, "\nRun:  s3s --context %s\n", ctxName)
	return nil
}

// ask prints a prompt (with optional default) and returns the trimmed answer,
// falling back to def on empty input.
func ask(out io.Writer, sc *bufio.Scanner, label, def string) string {
	if def != "" {
		fp(out, "%s [%s]: ", label, def)
	} else {
		fp(out, "%s: ", label)
	}
	if !sc.Scan() {
		return def
	}
	v := strings.TrimSpace(sc.Text())
	if v == "" {
		return def
	}
	return v
}

// askBool prompts for a yes/no answer with a default.
func askBool(out io.Writer, sc *bufio.Scanner, label string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	fp(out, "%s [%s]: ", label, hint)
	if !sc.Scan() {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(sc.Text())) {
	case "":
		return def
	case "y", "yes", "true", "1":
		return true
	default:
		return false
	}
}
