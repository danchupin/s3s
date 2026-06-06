package secret

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAWSProfile: parse static keys from a fixture credentials file; missing profile
// and a static-less profile both error (005 R11).
func TestAWSProfile(t *testing.T) {
	cred := filepath.Join(t.TempDir(), "credentials")
	body := "[prod]\naws_access_key_id = AK\naws_secret_access_key = SK\naws_session_token = TK\n\n[nokeys]\nregion = us-east-1\n"
	if err := os.WriteFile(cred, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", cred)

	ak, sk, tok, err := awsProfile("prod")
	if err != nil {
		t.Fatalf("prod: %v", err)
	}
	if ak != "AK" || sk != "SK" || tok != "TK" {
		t.Errorf("prod = %q/%q/%q, want AK/SK/TK", ak, sk, tok)
	}
	if _, _, _, err := awsProfile("missing"); err == nil {
		t.Error("missing profile should error")
	}
	if _, _, _, err := awsProfile("nokeys"); err == nil {
		t.Error("static-less profile should error")
	}
}
