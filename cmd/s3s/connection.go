package main

import (
	"context"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/storage"
	"github.com/danchupin/s3s/internal/ui"
)

// connSeam implements ui.Connector (006 US4): it tests reachability and persists a new
// connection, keeping all S3/config logic out of the UI (Constitution I). It holds the
// live *config.Config so a saved connection is immediately switchable in-session (FR-025).
type connSeam struct {
	cfg *config.Config
}

// Test runs a live reachability check against the draft (storage.New + ListBuckets) off
// the event loop (FR-025a). The secret is revealed only to build the throwaway client.
func (s connSeam) Test(ctx context.Context, d ui.ConnDraft) error {
	cc := storage.ClientConfig{
		Endpoint:    d.Endpoint,
		Region:      d.Region,
		PathStyle:   true,
		AccessKeyID: d.AccessKeyID,
		SecretKey:   d.Secret.Reveal(),
	}
	st, err := storage.New(cc)
	if err != nil {
		return err
	}
	_, err = st.ListBuckets(ctx)
	return err
}

// Save persists the connection: the secret to the keychain, the triple to config (FR-022/
// FR-022a/FR-023/FR-026). Returns the updated context-name list (FR-025).
func (s connSeam) Save(_ context.Context, d ui.ConnDraft) ([]string, error) {
	return s.cfg.AddConnection(config.NewConnection{
		Name:        d.Name,
		Endpoint:    d.Endpoint,
		Region:      d.Region,
		AccessKeyID: d.AccessKeyID,
		ReadOnly:    d.ReadOnly,
	}, d.Secret.Reveal())
}
