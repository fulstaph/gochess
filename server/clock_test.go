package server

import (
	"sync"
	"testing"
	"time"

	"github.com/fulstaph/gochess/chess"
)

func TestClock_Snapshot_Initial(t *testing.T) {
	tc := TimeControl{Initial: 5 * time.Minute, Increment: 3 * time.Second}
	c := NewClock(tc, func(int) {})

	wMs, bMs := c.Snapshot()
	if wMs != 300000 || bMs != 300000 {
		t.Fatalf("expected 300000ms each, got white=%d black=%d", wMs, bMs)
	}
}

func TestClock_Start_And_Snapshot(t *testing.T) {
	tc := TimeControl{Initial: 10 * time.Second}
	c := NewClock(tc, func(int) {})

	c.Start(chess.White)
	time.Sleep(50 * time.Millisecond)

	wMs, bMs := c.Snapshot()
	// White's clock should have decreased, black's should be full.
	if wMs >= 10000 {
		t.Fatalf("expected white time to decrease, got %d ms", wMs)
	}
	if bMs != 10000 {
		t.Fatalf("expected black time at 10000 ms, got %d ms", bMs)
	}
}

func TestClock_Punch_SwitchesSides(t *testing.T) {
	tc := TimeControl{Initial: 10 * time.Second, Increment: 1 * time.Second}
	c := NewClock(tc, func(int) {})

	c.Start(chess.White)
	time.Sleep(50 * time.Millisecond)
	c.Punch(chess.White)

	// After punch, white should have ~10s + 1s increment - elapsed.
	// Black's clock should now be ticking.
	time.Sleep(50 * time.Millisecond)
	wMs, bMs := c.Snapshot()

	// White got increment, so should be > 10000 (minus small elapsed before punch).
	if wMs < 10000 {
		t.Fatalf("expected white >= 10000ms after increment, got %d", wMs)
	}
	// Black should be slightly less than 10000 (has been ticking for ~50ms).
	if bMs >= 10000 {
		t.Fatalf("expected black < 10000ms, got %d", bMs)
	}
}

func TestClock_FlagLoss(t *testing.T) {
	tc := TimeControl{Initial: 50 * time.Millisecond}

	var mu sync.Mutex
	var flaggedLoser int
	flagged := make(chan struct{})

	c := NewClock(tc, func(loser int) {
		mu.Lock()
		flaggedLoser = loser
		mu.Unlock()
		close(flagged)
	})

	c.Start(chess.White)

	select {
	case <-flagged:
		mu.Lock()
		if flaggedLoser != chess.White {
			t.Fatalf("expected White to be flagged, got %d", flaggedLoser)
		}
		mu.Unlock()
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for flag")
	}
}

func TestClock_Stop_PreventsFlag(t *testing.T) {
	tc := TimeControl{Initial: 50 * time.Millisecond}

	flagged := make(chan struct{}, 1)
	c := NewClock(tc, func(int) {
		flagged <- struct{}{}
	})

	c.Start(chess.White)
	c.Stop()

	select {
	case <-flagged:
		t.Fatal("flag should not fire after Stop()")
	case <-time.After(200 * time.Millisecond):
		// OK — no flag fired.
	}
}

func TestParseTimeControl(t *testing.T) {
	tests := []struct {
		name    string
		wantNil bool
	}{
		{"bullet1", false},
		{"bullet2", false},
		{"blitz3", false},
		{"blitz5", false},
		{"rapid10", false},
		{"rapid15", false},
		{"none", true},
		{"unknown", true},
		{"", true},
	}
	for _, tt := range tests {
		tc := ParseTimeControl(tt.name)
		if tt.wantNil && tc != nil {
			t.Errorf("ParseTimeControl(%q) = non-nil, want nil", tt.name)
		}
		if !tt.wantNil && tc == nil {
			t.Errorf("ParseTimeControl(%q) = nil, want non-nil", tt.name)
		}
	}
}
