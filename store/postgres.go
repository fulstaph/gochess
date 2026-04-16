package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Postgres implements Store against a PostgreSQL database.
type Postgres struct {
	pool *pgxpool.Pool
}

// Open connects to the database at dsn and runs schema migrations.
func Open(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (db *Postgres) Close() {
	db.pool.Close()
}

// ---- Player ----

func (db *Postgres) SavePlayer(ctx context.Context, p *Player) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO players (id, username, password_hash, display_name, rating, is_guest, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			username      = EXCLUDED.username,
			password_hash = EXCLUDED.password_hash,
			display_name  = EXCLUDED.display_name,
			rating        = EXCLUDED.rating,
			is_guest      = EXCLUDED.is_guest`,
		p.ID, nullStr(p.Username), nullStr(p.PasswordHash),
		p.DisplayName, p.Rating, p.IsGuest, p.CreatedAt,
	)
	return err
}

func (db *Postgres) GetPlayer(ctx context.Context, id string) (*Player, error) {
	row := db.pool.QueryRow(ctx, `
		SELECT id, COALESCE(username,''), COALESCE(password_hash,''),
		       display_name, rating, is_guest, created_at
		FROM players WHERE id = $1`, id)
	return scanPlayer(row)
}

func (db *Postgres) GetPlayerByUsername(ctx context.Context, username string) (*Player, error) {
	row := db.pool.QueryRow(ctx, `
		SELECT id, COALESCE(username,''), COALESCE(password_hash,''),
		       display_name, rating, is_guest, created_at
		FROM players WHERE username = $1`, username)
	return scanPlayer(row)
}

func (db *Postgres) UpdateRating(ctx context.Context, playerID string, newRating int) error {
	_, err := db.pool.Exec(ctx, `UPDATE players SET rating = $1 WHERE id = $2`, newRating, playerID)
	return err
}

func scanPlayer(row pgx.Row) (*Player, error) {
	var p Player
	err := row.Scan(&p.ID, &p.Username, &p.PasswordHash, &p.DisplayName, &p.Rating, &p.IsGuest, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

// ---- Game ----

func (db *Postgres) SaveGame(ctx context.Context, g *Game) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO games (id, white_id, black_id, pgn, result, time_control, rated, started_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			pgn         = EXCLUDED.pgn,
			result      = EXCLUDED.result,
			finished_at = EXCLUDED.finished_at`,
		g.ID, nullStr(g.WhiteID), nullStr(g.BlackID),
		g.PGN, g.Result, g.TimeControl, g.Rated, g.StartedAt, g.FinishedAt,
	)
	return err
}

func (db *Postgres) GetGame(ctx context.Context, id string) (*Game, error) {
	row := db.pool.QueryRow(ctx, `
		SELECT id, COALESCE(white_id,''), COALESCE(black_id,''),
		       pgn, result, time_control, rated, started_at, finished_at
		FROM games WHERE id = $1`, id)
	return scanGame(row)
}

func (db *Postgres) ListGamesByPlayer(ctx context.Context, playerID string, limit, offset int) ([]*Game, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, COALESCE(white_id,''), COALESCE(black_id,''),
		       pgn, result, time_control, rated, started_at, finished_at
		FROM games
		WHERE white_id = $1 OR black_id = $1
		ORDER BY finished_at DESC
		LIMIT $2 OFFSET $3`, playerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func scanGame(row pgx.Row) (*Game, error) {
	var g Game
	err := row.Scan(&g.ID, &g.WhiteID, &g.BlackID, &g.PGN, &g.Result, &g.TimeControl, &g.Rated, &g.StartedAt, &g.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &g, err
}

// ---- Session ----

func (db *Postgres) SaveSession(ctx context.Context, s *Session) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO sessions (token, player_id, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token) DO UPDATE SET expires_at = EXCLUDED.expires_at`,
		s.Token, s.PlayerID, s.ExpiresAt,
	)
	return err
}

func (db *Postgres) GetSession(ctx context.Context, token string) (*Session, error) {
	var s Session
	err := db.pool.QueryRow(ctx, `
		SELECT token, player_id, expires_at FROM sessions
		WHERE token = $1 AND expires_at > NOW()`, token).
		Scan(&s.Token, &s.PlayerID, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (db *Postgres) DeleteSession(ctx context.Context, token string) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func (db *Postgres) DeleteExpiredSessions(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	return err
}

// ---- helpers ----

// nullStr returns nil for empty strings so Postgres stores NULL rather than "".
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// PlayerStats holds aggregated win/loss/draw counts for a player profile.
type PlayerStats struct {
	Wins   int
	Losses int
	Draws  int
}

// GetPlayerStats returns win/loss/draw counts for a player.
func (db *Postgres) GetPlayerStats(ctx context.Context, playerID string) (PlayerStats, error) {
	var st PlayerStats
	rows, err := db.pool.Query(ctx, `
		SELECT result, white_id FROM games
		WHERE (white_id = $1 OR black_id = $1) AND rated = TRUE`, playerID)
	if err != nil {
		return st, err
	}
	defer rows.Close()

	for rows.Next() {
		var result, whiteID string
		if err := rows.Scan(&result, &whiteID); err != nil {
			return st, err
		}
		switch {
		case len(result) >= 4 && result[:4] == "Draw":
			st.Draws++
		case len(result) >= 5 && result[:5] == "White":
			if whiteID == playerID {
				st.Wins++
			} else {
				st.Losses++
			}
		case len(result) >= 5 && result[:5] == "Black":
			if whiteID == playerID {
				st.Losses++
			} else {
				st.Wins++
			}
		}
	}
	return st, rows.Err()
}

// SessionTTL is how long a guest session stays valid without activity.
const SessionTTL = 30 * 24 * time.Hour
