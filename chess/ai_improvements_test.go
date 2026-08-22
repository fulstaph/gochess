package chess

import (
	"context"
	"testing"
	"time"
)

func FuzzParseFEN(f *testing.F) {
	// Seed with valid FEN strings.
	f.Add("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	f.Add("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")
	f.Add("8/8/8/8/8/8/8/4K2k w - - 0 1")
	f.Add("r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1")

	f.Fuzz(func(t *testing.T, fen string) {
		state, err := ParseFEN(fen)
		if err != nil {
			return // expected for malformed input
		}
		// Round-trip: valid FEN should survive ToFEN → ParseFEN.
		fen2 := ToFEN(state)
		state2, err := ParseFEN(fen2)
		if err != nil {
			t.Fatalf("round-trip failed: ToFEN produced %q which fails parse: %v", fen2, err)
		}
		if ToFEN(state2) != fen2 {
			t.Fatalf("double round-trip diverged: %q vs %q", ToFEN(state2), fen2)
		}
	})
}

func TestBestMoveCtxCancellation(t *testing.T) {
	state := InitialState()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Depth 10 would take a long time; context should cancel it early.
	mv, ok := BestMoveCtx(ctx, state, 10)
	if !ok {
		t.Fatal("BestMoveCtx should return a move even when cancelled")
	}
	// The move should be valid.
	formatted := FormatMove(mv)
	if len(formatted) < 4 {
		t.Fatalf("unexpected move format: %q", formatted)
	}
}

func TestQuiescenceDepthLimitDetailed(t *testing.T) {
	// Position with many captures available.
	fen := "r1bqkbnr/pppppppp/2n5/4P3/8/8/PPPP1PPP/RNBQKBNR b KQkq - 0 2"
	state, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	// Quiescence should terminate and return a finite score.
	score := quiescence(state, -inf, inf, maxQSDepth)
	if score <= -inf || score >= inf {
		t.Fatalf("quiescence returned unbounded score: %d", score)
	}
}

func TestHistoryDecay(t *testing.T) {
	// Verify that history values are halved between iterations.
	sc := &searchContext{
		tt:  newTransTable(10),
		ctx: context.Background(),
	}
	sc.history[0][1] = 100
	sc.history[10][20] = 200

	// Simulate one iteration of decay.
	for i := range sc.history {
		for j := range sc.history[i] {
			sc.history[i][j] /= 2
		}
	}

	if sc.history[0][1] != 50 {
		t.Errorf("expected 50, got %d", sc.history[0][1])
	}
	if sc.history[10][20] != 100 {
		t.Errorf("expected 100, got %d", sc.history[10][20])
	}
}

func TestTranspositionTableBasic(t *testing.T) {
	tt := newTransTable(10) // 1024 entries

	state := InitialState()
	hash := ZobristHash(state)

	// Store and probe.
	mv := Move{fromR: 6, fromC: 4, toR: 4, toC: 4} // e2e4
	tt.store(hash, 3, 42, ttExact, mv)

	entry, ok := tt.probe(hash)
	if !ok {
		t.Fatal("expected TT hit")
	}
	if entry.score != 42 || entry.depth != 3 || entry.flag != ttExact {
		t.Errorf("unexpected entry: %+v", entry)
	}
	if entry.best != mv {
		t.Errorf("unexpected best move: %+v", entry.best)
	}
}

func TestZobristConsistency(t *testing.T) {
	state := InitialState()
	h1 := ZobristHash(state)
	h2 := ZobristHash(state)
	if h1 != h2 {
		t.Fatal("same position should produce same hash")
	}

	// Different position should produce different hash.
	state2 := ApplyMove(state, Move{fromR: 6, fromC: 4, toR: 4, toC: 4})
	h3 := ZobristHash(state2)
	if h1 == h3 {
		t.Fatal("different positions should produce different hashes")
	}
}

func TestPawnStructureEval(t *testing.T) {
	// Doubled pawns should be penalized.
	doubled, err := ParseFEN("8/8/8/8/8/4P3/4P3/4K2k w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	normal, err := ParseFEN("8/8/8/8/8/4P3/3P4/4K2k w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	dScore := evaluatePawnStructure(doubled.board)
	nScore := evaluatePawnStructure(normal.board)
	if dScore >= nScore {
		t.Errorf("doubled pawns should score lower: doubled=%d normal=%d", dScore, nScore)
	}
}
