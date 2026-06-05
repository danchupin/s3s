package storage

import "context"

// readOnlyGuard wraps a backend and refuses every mutation, returning ErrReadOnly
// without contacting the wrapped client. Read methods pass straight through via the
// embedded Storage. This is the single runtime enforcement point for read-only
// (FR-003, FR-012): UI code cannot bypass it because it holds the guard, not the
// raw client.
type readOnlyGuard struct {
	Storage
}

// CreateFolder refuses the mutation; the wrapped backend is never called.
func (readOnlyGuard) CreateFolder(context.Context, string, string) error {
	return ErrReadOnly
}

var _ Mutator = readOnlyGuard{}

// Guard returns b unchanged when writable, or a read-only wrapper that refuses all
// mutations otherwise. The resolver calls this at construction time so the UI holds
// an already-correct backend (writable or read-only) for the active context.
func Guard(b Storage, writable bool) Storage {
	if writable {
		return b
	}
	return readOnlyGuard{Storage: b}
}
