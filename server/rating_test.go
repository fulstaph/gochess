package server

import (
	"sync"
	"testing"
)

func TestRater_InitialRating(t *testing.T) {
	r := newRater()
	if got := r.Rating("unknown-player"); got != 1200 {
		t.Fatalf("expected 1200, got %d", got)
	}
}

func TestRater_UpdateGame_Win(t *testing.T) {
	r := newRater()
	r.UpdateGame("white", "black", false)

	wRating := r.Rating("white")
	bRating := r.Rating("black")

	if wRating <= 1200 {
		t.Fatalf("expected winner rating > 1200, got %d", wRating)
	}
	if bRating >= 1200 {
		t.Fatalf("expected loser rating < 1200, got %d", bRating)
	}
	// Ratings should change by the same amount (K-factor symmetry for equal starting ratings).
	delta := wRating - 1200
	if bRating != 1200-delta {
		t.Fatalf("expected symmetric change: white delta=%d, black rating=%d", delta, bRating)
	}
}

func TestRater_UpdateGame_Draw(t *testing.T) {
	// Give one player a higher rating first.
	r := newRater()
	r.UpdateGame("strong", "weak", false) // strong wins → higher rating

	strongBefore := r.Rating("strong")
	weakBefore := r.Rating("weak")

	r.UpdateGame("strong", "weak", true) // draw

	strongAfter := r.Rating("strong")
	weakAfter := r.Rating("weak")

	// Stronger player should lose rating in a draw, weaker should gain.
	if strongAfter >= strongBefore {
		t.Fatalf("expected strong player to lose rating in draw: before=%d after=%d", strongBefore, strongAfter)
	}
	if weakAfter <= weakBefore {
		t.Fatalf("expected weak player to gain rating in draw: before=%d after=%d", weakBefore, weakAfter)
	}
}

func TestRater_EmptyID_NoOp(t *testing.T) {
	r := newRater()
	r.UpdateGame("", "black", false)
	r.UpdateGame("white", "", false)

	// Neither should have been created.
	if r.Rating("white") != 1200 {
		t.Fatal("expected no change for empty-ID game")
	}
	if r.Rating("black") != 1200 {
		t.Fatal("expected no change for empty-ID game")
	}
}

func TestRater_ConcurrentAccess(t *testing.T) {
	r := newRater()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = r.Rating("player")
			r.UpdateGame("a", "b", idx%2 == 0)
		}(i)
	}
	wg.Wait()

	// Just verify no panics or races occurred.
	_ = r.Rating("a")
	_ = r.Rating("b")
}
