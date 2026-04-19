// Package store defines the persistence interface and shared types for gochess.
package store

import (
	"context"
	"time"
)

// Player is the persistent representation of a user account.
type Player struct {
	ID           string
	Username     string // empty for guests
	PasswordHash string // empty for guests
	DisplayName  string
	Rating       int
	IsGuest      bool
	CreatedAt    time.Time
}

// Game is a completed game record.
type Game struct {
	ID          string
	WhiteID     string
	BlackID     string
	PGN         string
	Result      string
	TimeControl string
	Rated       bool
	StartedAt   time.Time
	FinishedAt  time.Time
}

// Session maps a bearer token to a player.
type Session struct {
	Token     string
	PlayerID  string
	ExpiresAt time.Time
}

// Store is the persistence interface. All methods accept a context so callers
// can cancel or time-out DB operations.
type Store interface {
	// Player operations
	SavePlayer(ctx context.Context, p *Player) error
	GetPlayer(ctx context.Context, id string) (*Player, error)
	GetPlayerByUsername(ctx context.Context, username string) (*Player, error)
	UpdateRating(ctx context.Context, playerID string, newRating int) error

	// Game operations
	SaveGame(ctx context.Context, g *Game) error
	GetGame(ctx context.Context, id string) (*Game, error)
	ListGamesByPlayer(ctx context.Context, playerID string, limit, offset int) ([]*Game, error)

	// Session operations
	SaveSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, token string) (*Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteExpiredSessions(ctx context.Context) error

	// Health
	Ping(ctx context.Context) error

	// Lifecycle
	Close()
}
