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

// Test runs a live reachability check against the draft off the event loop (FR-025a). The
// secret is revealed only to build the throwaway client. For a pinned (scoped) connection it
// probes the first declared bucket via a bounded ListLevel instead of ListBuckets — what the
// connection will actually use, and what bucket-scoped creds can reach (010 US4/FR-008).
func (s connSeam) Test(ctx context.Context, d ui.ConnDraft) error {
	cc := storage.ClientConfig{
		Endpoint:    d.Endpoint,
		Region:      d.Region,
		PathStyle:   d.PathStyle,
		AccessKeyID: d.AccessKeyID,
		SecretKey:   d.Secret.Reveal(),
	}
	st, err := storage.New(cc)
	if err != nil {
		return err
	}
	if len(d.Buckets) > 0 {
		_, err = st.ListLevel(ctx, storage.LevelQuery{Bucket: d.Buckets[0], MaxKeys: 1})
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
		PathStyle:   d.PathStyle,
		ReadOnly:    d.ReadOnly,
		Buckets:     d.Buckets,
	}, d.Secret.Reveal())
}

// Delete removes the named connection (config triple + keychain secret) and returns the
// updated context-name list (007 US5).
func (s connSeam) Delete(_ context.Context, name string) ([]string, error) {
	return s.cfg.RemoveConnection(name)
}

// AddBucket pins a bucket name to the named connection's cluster and returns the updated
// pinned list (010 US2). A config mutation only — no keychain, no S3 write.
func (s connSeam) AddBucket(_ context.Context, ctxName, bucket string) ([]string, error) {
	return s.cfg.AppendBucket(ctxName, bucket)
}
