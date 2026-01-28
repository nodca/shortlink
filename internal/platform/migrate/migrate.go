package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	Dir string
}

type Result struct {
	Dir          string
	AppliedFiles []string
	SkippedFiles []string
}

func Up(ctx context.Context, db *pgxpool.Pool, opts Options) (*Result, error) {
	dir, err := resolveMigrationsDir(opts.Dir)
	if err != nil {
		return nil, err
	}

	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}

	entries, err := listSQLFiles(dir)
	if err != nil {
		return nil, err
	}

	res := &Result{Dir: dir}
	for _, name := range entries {
		applied, err := isApplied(ctx, db, name)
		if err != nil {
			return nil, err
		}
		if applied {
			res.SkippedFiles = append(res.SkippedFiles, name)
			continue
		}
		if err := applyFile(ctx, db, dir, name); err != nil {
			return nil, err
		}
		res.AppliedFiles = append(res.AppliedFiles, name)
	}

	return res, nil
}

func ensureTable(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`)
	return err
}

func listSQLFiles(dir string) ([]string, error) {
	entries := make([]string, 0, 32)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			entries = append(entries, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	return entries, nil
}

func isApplied(ctx context.Context, db *pgxpool.Pool, version string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists)
	return exists, err
}

func applyFile(ctx context.Context, db *pgxpool.Pool, dir string, filename string) error {
	path := filepath.Join(dir, filename)
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", filename, err)
	}

	// Execute as a single batch. Most files are idempotent via IF NOT EXISTS.
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %s: %w", filename, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1,$2)`, filename, time.Now()); err != nil {
		return fmt.Errorf("record migration %s: %w", filename, err)
	}

	return tx.Commit(ctx)
}

func resolveMigrationsDir(opt string) (string, error) {
	if strings.TrimSpace(opt) != "" {
		return filepath.Clean(opt), nil
	}

	// prefer CWD/migrations
	if dir, err := filepath.Abs("migrations"); err == nil {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir, nil
		}
	}

	// fallback to executable dir/migrations
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve migrations dir: %w", err)
	}
	exeDir := filepath.Dir(exe)
	dir := filepath.Join(exeDir, "migrations")
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("migrations dir not found (tried %s)", dir)
	}
	return dir, nil
}

