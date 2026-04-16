package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type migration struct {
	Version int
	SQL     string
}

// migrations is the ordered list of schema migrations.
// Each migration is applied exactly once. Never modify an existing migration;
// append a new one instead.
var migrations = []migration{
	{
		Version: 1,
		SQL: `
CREATE TABLE IF NOT EXISTS players (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE,          -- NULL for guests
    password_hash TEXT,                 -- NULL for guests
    display_name  TEXT NOT NULL,
    rating        INT  NOT NULL DEFAULT 1200,
    is_guest      BOOL NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    player_id  TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_player_id ON sessions(player_id);
CREATE INDEX IF NOT EXISTS sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS games (
    id           TEXT PRIMARY KEY,
    white_id     TEXT REFERENCES players(id),
    black_id     TEXT REFERENCES players(id),
    pgn          TEXT NOT NULL DEFAULT '',
    result       TEXT NOT NULL DEFAULT '',
    time_control TEXT NOT NULL DEFAULT '',
    rated        BOOL NOT NULL DEFAULT FALSE,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS games_white_id ON games(white_id);
CREATE INDEX IF NOT EXISTS games_black_id ON games(black_id);
`,
	},
}

const schemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

// runMigrations applies all pending migrations in order.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var maxVersion int
	err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxVersion)
	if err != nil {
		return fmt.Errorf("query max version: %w", err)
	}

	for _, m := range migrations {
		if m.Version <= maxVersion {
			continue
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", m.Version, err)
		}
		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %d: %w", m.Version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.Version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}
	return nil
}
