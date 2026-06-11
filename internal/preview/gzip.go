package preview

import (
	"bytes"
	"compress/gzip"
	"io"
)

// Compressed carries the gzip-decode metadata of a transparently decompressed payload
// (017 US5/FR-026): the compressed input size and whether the OUTPUT hit the Limit cap.
type Compressed struct {
	From      int64 // compressed bytes the preview fetched
	Truncated bool  // decompressed output capped at Limit
}

// isGzip reports the gzip magic (1f 8b) — the primary detection signal; name/header
// hints only corroborate it (research D13).
func isGzip(data []byte) bool {
	return len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b
}

// gunzipCapped decompresses at most limit output bytes (io.LimitReader on OUTPUT —
// compression-bomb-safe by construction). truncated reports a hit cap.
func gunzipCapped(data []byte, limit int64) (out []byte, truncated bool, err error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = zr.Close() }()
	out, err = io.ReadAll(io.LimitReader(zr, limit))
	if err != nil {
		return nil, false, err
	}
	if int64(len(out)) == limit {
		// Probe one more byte to distinguish "exactly limit" from "capped".
		var one [1]byte
		if n, _ := zr.Read(one[:]); n > 0 {
			truncated = true
		}
	}
	return out, truncated, nil
}
