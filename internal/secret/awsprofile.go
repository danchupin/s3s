package secret

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// awsCredentialsPath resolves the shared credentials file, honoring
// AWS_SHARED_CREDENTIALS_FILE (005 R11).
func awsCredentialsPath() string {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".aws", "credentials")
	}
	return filepath.Join(home, ".aws", "credentials")
}

// awsProfile parses the named profile's static credentials from the shared
// credentials INI file. SSO / role assumption / credential_process are out of scope:
// a profile without static keys is a clear error (005 R11).
func awsProfile(profile string) (accessKey, secret, token string, err error) {
	path := awsCredentialsPath()
	f, err := os.Open(path) //nolint:gosec // path is the user's own aws config
	if err != nil {
		return "", "", "", fmt.Errorf("secret: open aws credentials %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	inSection := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.TrimSpace(line[1:len(line)-1]) == profile
			continue
		}
		if !inSection {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "aws_access_key_id":
			accessKey = strings.TrimSpace(v)
		case "aws_secret_access_key":
			secret = strings.TrimSpace(v)
		case "aws_session_token":
			token = strings.TrimSpace(v)
		}
	}
	if accessKey == "" || secret == "" {
		return "", "", "", fmt.Errorf("secret: profile %q has no static credentials in %s", profile, path)
	}
	return accessKey, secret, token, nil
}
