package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fulstaph/gochess/store"
)

func openTestDB(t *testing.T) *store.Postgres {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration tests")
	}
	db, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestPlayerRoundtrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	p := &store.Player{
		ID:          "test-player-1",
		DisplayName: "TestUser",
		Rating:      1250,
		IsGuest:     true,
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}
	if err := db.SavePlayer(ctx, p); err != nil {
		t.Fatalf("SavePlayer: %v", err)
	}

	got, err := db.GetPlayer(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPlayer: %v", err)
	}
	if got.DisplayName != p.DisplayName {
		t.Errorf("DisplayName: got %q want %q", got.DisplayName, p.DisplayName)
	}
	if got.Rating != p.Rating {
		t.Errorf("Rating: got %d want %d", got.Rating, p.Rating)
	}
}

func TestUpdateRating(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	p := &store.Player{ID: "test-rating-1", DisplayName: "Rater", Rating: 1200, IsGuest: true, CreatedAt: time.Now()}
	if err := db.SavePlayer(ctx, p); err != nil {
		t.Fatalf("SavePlayer: %v", err)
	}
	if err := db.UpdateRating(ctx, p.ID, 1230); err != nil {
		t.Fatalf("UpdateRating: %v", err)
	}
	got, err := db.GetPlayer(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPlayer: %v", err)
	}
	if got.Rating != 1230 {
		t.Errorf("Rating: got %d want 1230", got.Rating)
	}
}

func TestSessionRoundtrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	p := &store.Player{ID: "test-session-1", DisplayName: "Sess", Rating: 1200, IsGuest: true, CreatedAt: time.Now()}
	if err := db.SavePlayer(ctx, p); err != nil {
		t.Fatalf("SavePlayer: %v", err)
	}

	s := &store.Session{
		Token:     "test-token-abc123",
		PlayerID:  p.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := db.SaveSession(ctx, s); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	got, err := db.GetSession(ctx, s.Token)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.PlayerID != p.ID {
		t.Errorf("PlayerID: got %q want %q", got.PlayerID, p.ID)
	}

	if err := db.DeleteSession(ctx, s.Token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := db.GetSession(ctx, s.Token); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestGameRoundtrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	wp := &store.Player{ID: "game-white-1", DisplayName: "White", Rating: 1200, IsGuest: true, CreatedAt: time.Now()}
	bp := &store.Player{ID: "game-black-1", DisplayName: "Black", Rating: 1200, IsGuest: true, CreatedAt: time.Now()}
	for _, p := range []*store.Player{wp, bp} {
		if err := db.SavePlayer(ctx, p); err != nil {
			t.Fatalf("SavePlayer: %v", err)
		}
	}

	g := &store.Game{
		ID:          "test-game-1",
		WhiteID:     wp.ID,
		BlackID:     bp.ID,
		PGN:         "1. e4 e5 2. Nf3",
		Result:      "White wins",
		TimeControl: "blitz5",
		Rated:       true,
		StartedAt:   time.Now().Add(-5 * time.Minute),
		FinishedAt:  time.Now(),
	}
	if err := db.SaveGame(ctx, g); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}

	got, err := db.GetGame(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if got.PGN != g.PGN {
		t.Errorf("PGN mismatch")
	}

	list, err := db.ListGamesByPlayer(ctx, wp.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListGamesByPlayer: %v", err)
	}
	if len(list) == 0 {
		t.Error("expected at least one game in list")
	}
}
