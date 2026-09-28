// Package repository implements persistence for rooms, participants, rounds
// and votes on top of PostgreSQL (database/sql + lib/pq).
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

var (
	// ErrNotFound is returned when the requested row does not exist. Malformed
	// identifiers (e.g. a non-UUID id) are reported as not found as well.
	ErrNotFound = errors.New("repository: not found")
	// ErrDuplicate is returned when an insert violates a unique constraint.
	ErrDuplicate = errors.New("repository: duplicate")
)

// PostgreSQL error codes we translate.
const (
	pgInvalidTextRepresentation = "22P02"
	pgUniqueViolation           = "23505"
)

// mapErr translates driver errors into the package's sentinel errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if pqErr, ok := errors.AsType[*pq.Error](err); ok {
		switch pqErr.Code {
		case pgInvalidTextRepresentation:
			return ErrNotFound
		case pgUniqueViolation:
			return ErrDuplicate
		}
	}
	return err
}

// withTx runs fn inside a transaction, committing on success and rolling back otherwise.
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
