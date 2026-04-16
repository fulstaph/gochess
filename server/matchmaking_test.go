package server

import "testing"

func TestMatchmaker_NoMatchAlone(t *testing.T) {
	mm := newMatchmaker()
	p := testPlayer("p1")

	white, black := mm.Enqueue(p, "blitz5", 1200)
	if white != nil || black != nil {
		t.Fatal("expected no match with single player")
	}
}

func TestMatchmaker_MatchBySameTC(t *testing.T) {
	mm := newMatchmaker()
	p1 := testPlayer("p1")
	p2 := testPlayer("p2")

	mm.Enqueue(p1, "blitz5", 1200)
	white, black := mm.Enqueue(p2, "blitz5", 1200)

	if white == nil || black == nil {
		t.Fatal("expected match with two players same TC")
	}
	if white.player.ID != "p1" || black.player.ID != "p2" {
		t.Fatalf("unexpected pairing: white=%s black=%s", white.player.ID, black.player.ID)
	}
}

func TestMatchmaker_NoMatchDifferentTC(t *testing.T) {
	mm := newMatchmaker()
	p1 := testPlayer("p1")
	p2 := testPlayer("p2")

	mm.Enqueue(p1, "blitz5", 1200)
	white, black := mm.Enqueue(p2, "rapid10", 1200)

	if white != nil || black != nil {
		t.Fatal("expected no match with different TCs")
	}
}

func TestMatchmaker_NoMatchOutsideRatingWindow(t *testing.T) {
	mm := newMatchmaker()
	p1 := testPlayer("p1")
	p2 := testPlayer("p2")

	mm.Enqueue(p1, "blitz5", 1000)
	white, black := mm.Enqueue(p2, "blitz5", 1300) // 300 > 200 window

	if white != nil || black != nil {
		t.Fatal("expected no match outside rating window")
	}
}

func TestMatchmaker_Dequeue(t *testing.T) {
	mm := newMatchmaker()
	p1 := testPlayer("p1")
	p2 := testPlayer("p2")

	mm.Enqueue(p1, "blitz5", 1200)
	removed := mm.Dequeue("p1")
	if !removed {
		t.Fatal("expected Dequeue to return true")
	}

	// p2 should not match since p1 was removed.
	white, black := mm.Enqueue(p2, "blitz5", 1200)
	if white != nil || black != nil {
		t.Fatal("expected no match after dequeue")
	}
}

func TestMatchmaker_DuplicateRemoval(t *testing.T) {
	mm := newMatchmaker()
	p1 := testPlayer("p1")
	p2 := testPlayer("p2")

	mm.Enqueue(p1, "blitz5", 1200)
	mm.Enqueue(p1, "blitz5", 1200) // re-enqueue removes stale entry

	// Only one entry for p1, so p2 should match.
	white, black := mm.Enqueue(p2, "blitz5", 1200)
	if white == nil || black == nil {
		t.Fatal("expected match after re-enqueue")
	}
}
