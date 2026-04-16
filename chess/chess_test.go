package chess

import "testing"

func applyMoveInput(t *testing.T, state GameState, input string) GameState {
	t.Helper()
	mv, err := ParseMove(input, state)
	if err != nil {
		t.Fatalf("parse %s: %v", input, err)
	}
	return ApplyMove(state, mv)
}

func TestInitialStatePieces(t *testing.T) {
	state := InitialState()
	board := state.Board()

	if board[7][4] != 'K' || board[0][4] != 'k' {
		t.Fatalf("expected kings on e1/e8, got %c and %c", board[7][4], board[0][4])
	}
	if board[6][0] != 'P' || board[1][7] != 'p' {
		t.Fatalf("expected pawns on a2/h7, got %c and %c", board[6][0], board[1][7])
	}
}

func TestPawnAdvanceAndTurn(t *testing.T) {
	state := InitialState()
	state = applyMoveInput(t, state, "e2e4")
	board := state.Board()

	if board[6][4] != 0 || board[4][4] != 'P' {
		t.Fatalf("expected pawn on e4 after move")
	}
	if state.Turn() != Black {
		t.Fatalf("expected black to move")
	}
}

func TestIllegalMoveFromEmptySquare(t *testing.T) {
	state := InitialState()
	if _, err := ParseMove("e3e4", state); err == nil {
		t.Fatalf("expected illegal move from empty square")
	}
}

func TestCastlingKingside(t *testing.T) {
	state := InitialState()
	state = applyMoveInput(t, state, "e2e4")
	state = applyMoveInput(t, state, "e7e5")
	state = applyMoveInput(t, state, "g1f3")
	state = applyMoveInput(t, state, "b8c6")
	state = applyMoveInput(t, state, "f1e2")
	state = applyMoveInput(t, state, "g8f6")

	state = applyMoveInput(t, state, "o-o")
	board := state.Board()
	if board[7][6] != 'K' || board[7][5] != 'R' || board[7][7] != 0 {
		t.Fatalf("expected castled king on g1 and rook on f1")
	}
}

func TestCastlingQueenside(t *testing.T) {
	state := InitialState()
	// Clear b1, c1, d1 so white can castle queenside.
	state = applyMoveInput(t, state, "d2d4")
	state = applyMoveInput(t, state, "e7e5")
	state = applyMoveInput(t, state, "c1f4")
	state = applyMoveInput(t, state, "g8f6")
	state = applyMoveInput(t, state, "b1c3")
	state = applyMoveInput(t, state, "d7d6")
	state = applyMoveInput(t, state, "d1d3")
	state = applyMoveInput(t, state, "b8c6")

	state = applyMoveInput(t, state, "o-o-o")
	board := state.Board()
	if board[7][2] != 'K' || board[7][3] != 'R' || board[7][0] != 0 {
		t.Fatalf("expected castled king on c1 and rook on d1")
	}
}

func TestEnPassantCapture(t *testing.T) {
	state := InitialState()
	state = applyMoveInput(t, state, "e2e4")
	state = applyMoveInput(t, state, "a7a5")
	state = applyMoveInput(t, state, "e4e5")
	state = applyMoveInput(t, state, "d7d5")
	state = applyMoveInput(t, state, "e5d6")

	board := state.Board()
	if board[2][3] != 'P' || board[3][3] != 0 {
		t.Fatalf("expected en passant capture to place pawn on d6 and clear d5")
	}
}

func TestPromotionDefaultsToQueen(t *testing.T) {
	state := GameState{
		turn:       White,
		enPassantR: -1,
		enPassantC: -1,
	}
	state.board[7][4] = 'K'
	state.board[0][0] = 'k'
	state.board[1][4] = 'P'

	mv, err := ParseMove("e7e8", state)
	if err != nil {
		t.Fatalf("unexpected error parsing promotion: %v", err)
	}
	if mv.promotion != 'Q' {
		t.Fatalf("expected default promotion to queen, got %c", mv.promotion)
	}

	state = ApplyMove(state, mv)
	if state.board[0][4] != 'Q' {
		t.Fatalf("expected promoted queen on e8")
	}
}

func TestFoolsMateCheckmate(t *testing.T) {
	state := InitialState()
	state = applyMoveInput(t, state, "f2f3")
	state = applyMoveInput(t, state, "e7e5")
	state = applyMoveInput(t, state, "g2g4")
	state = applyMoveInput(t, state, "d8h4")

	if !IsInCheck(state, White) {
		t.Fatalf("expected white to be in check")
	}
	if len(GenerateLegalMoves(state)) != 0 {
		t.Fatalf("expected no legal moves for checkmate")
	}
}

func TestBestMoveFindsMateInOne(t *testing.T) {
	state := InitialState()
	state = applyMoveInput(t, state, "f2f3")
	state = applyMoveInput(t, state, "e7e5")
	state = applyMoveInput(t, state, "g2g4")

	best, ok := BestMove(state, 1)
	if !ok {
		t.Fatalf("expected AI to find a move")
	}

	expected, err := ParseMove("d8h4", state)
	if err != nil {
		t.Fatalf("expected Qh4 to be legal: %v", err)
	}

	if best != expected {
		t.Fatalf("expected %s, got %s", FormatMove(expected), FormatMove(best))
	}
}

func TestPositionKeyDiffersOnEnPassant(t *testing.T) {
	state := InitialState()
	key1 := PositionKey(state)

	state.enPassantR = 5
	state.enPassantC = 4
	key2 := PositionKey(state)

	if key1 == key2 {
		t.Fatalf("expected en passant to change position key")
	}
}

func TestFiftyMoveDraw(t *testing.T) {
	state := InitialState()
	state.halfmove = 99
	if IsFiftyMoveDraw(state) {
		t.Fatalf("expected no draw at halfmove 99")
	}
	state.halfmove = 100
	if !IsFiftyMoveDraw(state) {
		t.Fatalf("expected draw at halfmove 100")
	}
}

func TestSeventyFiveMoveDraw(t *testing.T) {
	state := InitialState()
	state.halfmove = 149
	if IsSeventyFiveMoveDraw(state) {
		t.Fatalf("expected no draw at halfmove 149")
	}
	state.halfmove = 150
	if !IsSeventyFiveMoveDraw(state) {
		t.Fatalf("expected draw at halfmove 150")
	}
}

func TestPositionKeyIgnoresUncapturableEnPassant(t *testing.T) {
	state := InitialState()
	key1 := PositionKey(state)

	// Place an en-passant square that cannot be captured (no adjacent white pawns).
	state.enPassantR = 2
	state.enPassantC = 4
	state.turn = White

	key2 := PositionKey(state)
	if key1 != key2 {
		t.Fatalf("expected uncapturable en passant square to be ignored in position key")
	}
}

func TestQuiescenceDepthLimit(t *testing.T) {
	// Position with many captures available; quiescence must terminate within maxQSDepth.
	// We use a simple position where pieces can capture each other repeatedly.
	// The test just verifies BestMove returns without hanging or panicking.
	state := InitialState()
	// Apply a few moves to get pieces in the center where captures can occur.
	for _, mv := range []string{"e2e4", "d7d5", "e4d5", "d8d5", "d2d4", "d5e4"} {
		state = applyMoveInput(t, state, mv)
	}
	// The position now has captures available. BestMove must return normally.
	_, ok := BestMove(state, 3)
	if !ok {
		t.Fatal("expected a legal move to exist")
	}
}

func TestQuiescenceAtDepthZeroReturnsStaticEval(t *testing.T) {
	state := InitialState()
	score := quiescence(state, -inf, inf, 0)
	staticEval := evaluate(state)
	if score != staticEval {
		t.Fatalf("quiescence at depth 0 should return static eval %d, got %d", staticEval, score)
	}
}
