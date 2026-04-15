// Command rewrap re-encrypts every secret column under the current KEK.
//
// Safe to run repeatedly: rows already tagged with the current KEK version are
// skipped. Rows encrypted with an older KEK (or the pre-keyring legacy format)
// are decrypted and re-sealed with the current KEK's envelope, without altering
// the plaintext.
//
// Usage:
//
//	ENCRYPTION_KEYS=v1:... ,v2:... ENCRYPTION_KEY_CURRENT=v2 \
//	    DATABASE_URL=postgres://... go run ./cmd/rewrap [--dry-run]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/store"
)

// target describes one (table, id column, encrypted column) tuple to rewrap.
type target struct {
	table  string
	idCol  string
	colEnc string
}

var targets = []target{
	{"git_credentials", "id", "token_encrypted"},
	{"applications", "id", "git_credentials_encrypted"},
	{"databases", "id", "credentials_encrypted"},
	{"domains", "id", "ssl_credentials_encrypted"},
	{"env_vars", "id", "value_encrypted"},
	{"project_env_vars", "id", "value_encrypted"},
}

func main() {
	dryRun := flag.Bool("dry-run", false, "report what would be rewrapped without writing")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	pool, err := store.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	ctx := context.Background()
	slog.Info("rewrap starting",
		"current_kek", cfg.Keyring.CurrentVersion(),
		"available_keks", cfg.Keyring.Versions(),
		"dry_run", *dryRun,
	)

	var total, rewrapped, skipped, failed int
	for _, t := range targets {
		r, err := rewrapTable(ctx, pool, cfg.Keyring, t, *dryRun)
		if err != nil {
			slog.Error("table failed", "table", t.table, "error", err)
			failed++
			continue
		}
		total += r.scanned
		rewrapped += r.rewrapped
		skipped += r.skipped
		slog.Info("table done",
			"table", t.table,
			"scanned", r.scanned,
			"rewrapped", r.rewrapped,
			"skipped_current", r.skipped,
		)
	}

	slog.Info("rewrap complete",
		"total_scanned", total,
		"rewrapped", rewrapped,
		"skipped_current", skipped,
		"failed_tables", failed,
	)
	if failed > 0 {
		os.Exit(1)
	}
}

type result struct {
	scanned, rewrapped, skipped int
}

func rewrapTable(ctx context.Context, pool *pgxpool.Pool, kr *crypto.Keyring, t target, dryRun bool) (result, error) {
	var res result

	selectSQL := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s IS NOT NULL", t.idCol, t.colEnc, t.table, t.colEnc)
	rows, err := pool.Query(ctx, selectSQL)
	if err != nil {
		return res, fmt.Errorf("select: %w", err)
	}

	type row struct {
		id  string
		enc []byte
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.enc); err != nil {
			rows.Close()
			return res, fmt.Errorf("scan: %w", err)
		}
		batch = append(batch, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("iterate: %w", err)
	}

	updateSQL := fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s = $2", t.table, t.colEnc, t.idCol)
	for _, r := range batch {
		res.scanned++
		if len(r.enc) == 0 {
			res.skipped++
			continue
		}
		if kr.IsCurrent(r.enc) {
			res.skipped++
			continue
		}
		upgraded, err := kr.Rewrap(r.enc)
		if err != nil {
			return res, fmt.Errorf("rewrap row %s: %w", r.id, err)
		}
		if dryRun {
			res.rewrapped++
			continue
		}
		if _, err := pool.Exec(ctx, updateSQL, upgraded, r.id); err != nil {
			return res, fmt.Errorf("update row %s: %w", r.id, err)
		}
		res.rewrapped++
	}
	return res, nil
}
