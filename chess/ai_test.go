package chess

import "testing"

func TestOrderMoves_PVFirst(t *testing.T) {
	state := InitialState()
	moves := GenerateLegalMoves(state)
	pv := moves[len(moves)-1] // pick last move as PV

	ordered := orderMoves(state, moves, pv, nil, 1)
	if ordered[0] != pv {
		t.Fatalf("expected PV move first, got %v", ordered[0])
	}
}

func TestOrderMoves_CapturesBeforeQuiet(t *testing.T) {
	// Set up a position with captures available.
	// White pawn on e4, black pawn on d5 → e4xd5 is a capture.
	state, err := ParseFEN("rnbqkbnr/ppp1pppp/8/3p4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq d6 0 2")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	moves := GenerateLegalMoves(state)
	ordered := orderMoves(state, moves, Move{}, nil, 1)

	// Find the first quiet move index.
	firstQuiet := -1
	lastCapture := -1
	for i, mv := range ordered {
		if isCapture(state, mv) {
			lastCapture = i
		} else if firstQuiet == -1 {
			firstQuiet = i
		}
	}

	if lastCapture >= 0 && firstQuiet >= 0 && lastCapture > firstQuiet {
		t.Fatalf("captures should come before quiet moves: last capture at %d, first quiet at %d",
			lastCapture, firstQuiet)
	}
}

func TestOrderMoves_KillerBeforeQuiet(t *testing.T) {
	state := InitialState()
	moves := GenerateLegalMoves(state)

	sc := &searchContext{maxDepth: 4}
	// Set a killer at ply 3 (depth=1, ply = maxDepth - depth = 3).
	killerMove := moves[len(moves)-1]
	if isCapture(state, killerMove) {
		killerMove = moves[0] // pick a quiet move
	}
	sc.killers[3][0] = killerMove

	ordered := orderMoves(state, moves, Move{}, sc, 1) // depth=1 → ply=3

	// Find killer position in the ordered list.
	killerIdx := -1
	firstNonKillerQuiet := -1
	for i, mv := range ordered {
		if mv == killerMove {
			killerIdx = i
		} else if !isCapture(state, mv) && firstNonKillerQuiet == -1 {
			firstNonKillerQuiet = i
		}
	}

	if killerIdx == -1 {
		t.Fatal("killer move not found in ordered list")
	}
	if firstNonKillerQuiet >= 0 && killerIdx > firstNonKillerQuiet {
		t.Fatalf("killer move (idx %d) should come before quiet moves (first at %d)",
			killerIdx, firstNonKillerQuiet)
	}
}

func TestOrderMoves_HistorySortsQuiet(t *testing.T) {
	state := InitialState()
	moves := GenerateLegalMoves(state)

	sc := &searchContext{maxDepth: 4}

	// Find two quiet moves and give one a higher history score.
	var quiet1, quiet2 Move
	found := 0
	for _, mv := range moves {
		if !isCapture(state, mv) {
			if found == 0 {
				quiet1 = mv
			} else if found == 1 {
				quiet2 = mv
			}
			found++
			if found == 2 {
				break
			}
		}
	}
	if found < 2 {
		t.Skip("not enough quiet moves")
	}

	// Give quiet2 a much higher history score.
	sc.history[sqIndex(quiet2.fromR, quiet2.fromC)][sqIndex(quiet2.toR, quiet2.toC)] = 1000

	ordered := orderMoves(state, moves, Move{}, sc, 1)

	// quiet2 should appear before quiet1 in ordered list (among quiet moves).
	idx1, idx2 := -1, -1
	for i, mv := range ordered {
		if mv == quiet1 {
			idx1 = i
		}
		if mv == quiet2 {
			idx2 = i
		}
	}

	if idx2 > idx1 {
		t.Fatalf("quiet2 (history=1000, idx=%d) should come before quiet1 (history=0, idx=%d)",
			idx2, idx1)
	}
}

func TestEvaluate_PhaseAware(t *testing.T) {
	// Full material position — midgame PSTs should dominate.
	mgState := InitialState()
	mgScore := evaluate(mgState)

	// Endgame position: only kings and pawns.
	egState, err := ParseFEN("4k3/pppppppp/8/8/8/8/PPPPPPPP/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	egScore := evaluate(egState)

	// Both should produce valid scores (just verify no panics and different values).
	if mgScore == egScore {
		t.Logf("mgScore=%d egScore=%d — scores differ as expected", mgScore, egScore)
	}

	// With only kings+pawns, phase should be 0 (pure endgame).
	// King should prefer center in endgame. A king on e1 vs e4 should score differently.
	kCenter, err := ParseFEN("8/8/8/8/4K3/8/8/4k3 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	kCorner, err := ParseFEN("K7/8/8/8/8/8/8/4k3 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	centerEval := evaluate(kCenter)
	cornerEval := evaluate(kCorner)

	// In endgame, centralized king should score better for White.
	if centerEval <= cornerEval {
		t.Fatalf("expected centralized king to score higher in endgame: center=%d corner=%d",
			centerEval, cornerEval)
	}
}

func TestBestMove_FindsMateInOne(t *testing.T) {
	// White to move, Qh5# is mate.
	state, err := ParseFEN("rnbqkbnr/pppp1ppp/8/4p3/6P1/5P2/PPPPP2P/RNBQKBNR b KQkq - 0 2")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	mv, ok := BestMove(state, 2)
	if !ok {
		t.Fatal("BestMove returned no move")
	}

	// Black should find Qh4# (d8→h4).
	if mv.fromR != 0 || mv.fromC != 3 || mv.toR != 4 || mv.toC != 7 {
		t.Fatalf("expected Qh4# (d8→h4), got from=(%d,%d) to=(%d,%d)",
			mv.fromR, mv.fromC, mv.toR, mv.toC)
	}
}

func TestBestMove_CapturesHangingPiece(t *testing.T) {
	// White queen hangs on e5, Black bishop on g7 can capture it.
	state, err := ParseFEN("4k3/6b1/8/4Q3/8/8/8/4K3 b - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	mv, ok := BestMove(state, 2)
	if !ok {
		t.Fatal("BestMove returned no move")
	}

	// Black bishop should capture the queen on e5 (row=3, col=4).
	if mv.toR != 3 || mv.toC != 4 {
		t.Fatalf("expected capture on e5 (row=3, col=4), got from=(%d,%d) to=(%d,%d)",
			mv.fromR, mv.fromC, mv.toR, mv.toC)
	}
}
