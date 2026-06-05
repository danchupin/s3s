package storage

import (
	"context"
	"io"
)

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

// Every new mutating method (003) MUST be refused here too — this is the single
// runtime enforcement point. Omitting one would let a write slip past a read-only
// context (FR-012, SC-008). A guard test asserts each returns ErrReadOnly.
func (readOnlyGuard) RemoveObject(context.Context, string, string) error { return ErrReadOnly }

func (readOnlyGuard) UploadFile(context.Context, string, string, io.Reader, int64) error {
	return ErrReadOnly
}

func (readOnlyGuard) CopyKey(context.Context, string, string, string) error { return ErrReadOnly }

func (readOnlyGuard) MoveObject(context.Context, string, string, string) error { return ErrReadOnly }

func (readOnlyGuard) DeleteRecursive(context.Context, string, string, func(DeleteSummary)) (DeleteSummary, error) {
	return DeleteSummary{}, ErrReadOnly
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
