// Package migrations embeds the SQL schema files and applies them.
//
// Every *.sql file is applied on each startup in lexical order, so each file
// must be idempotent (CREATE ... IF NOT EXISTS, guarded DO blocks, etc.).
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.sql
var files embed.FS

// advisoryLockKey serializes concurrent Apply calls (e.g. several replicas
// starting at once). The value is arbitrary but must stay stable.
const advisoryLockKey = 7_386_104_221

// Names returns the embedded migration file names in the order they are applied.
func Names() ([]string, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// Apply executes every embedded migration against db in lexical order.
func Apply(ctx context.Context, db *sql.DB) error {
	names, err := Names()
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, advisoryLockKey) //nolint:errcheck // lock is released with the session anyway

	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := conn.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}
