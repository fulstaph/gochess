package chess

import "testing"

func TestToFENInitialPosition(t *testing.T) {
	state := InitialState()
	got := ToFEN(state)
	want := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if got != want {
		t.Fatalf("ToFEN initial:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestFENRoundtripInitialPosition(t *testing.T) {
	state := InitialState()
	fen := ToFEN(state)
	parsed, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	if ToFEN(parsed) != fen {
		t.Fatalf("FEN roundtrip failed:\noriginal: %s\nreparsed: %s", fen, ToFEN(parsed))
	}
}

func TestFENAfterE4(t *testing.T) {
	state := InitialState()
	mv, err := ParseMove("e2e4", state)
	if err != nil {
		t.Fatalf("ParseMove: %v", err)
	}
	state = ApplyMove(state, mv)

	got := ToFEN(state)
	want := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	if got != want {
		t.Fatalf("ToFEN after e4:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestFENRoundtripEnPassant(t *testing.T) {
	// After 1. e4 e5 2. e5d5 ... d7d5, en passant on d6 is possible.
	state := InitialState()
	for _, m := range []string{"e2e4", "e7e5", "d2d4", "d7d5"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	fen := ToFEN(state)
	parsed, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	if ToFEN(parsed) != fen {
		t.Fatalf("FEN roundtrip with en passant failed:\noriginal: %s\nreparsed: %s", fen, ToFEN(parsed))
	}
}

func TestFENRoundtripCastlingRights(t *testing.T) {
	// After white castles kingside, only queenside and black rights remain.
	state := InitialState()
	for _, m := range []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1e2", "g8f6", "o-o"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	fen := ToFEN(state)
	parsed, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	if ToFEN(parsed) != fen {
		t.Fatalf("FEN roundtrip castling rights failed:\noriginal: %s\nreparsed: %s", fen, ToFEN(parsed))
	}
	// Moving the king loses both white castling rights.
	if parsed.castling[0] {
		t.Fatalf("expected white kingside castling to be lost after O-O")
	}
	if parsed.castling[1] {
		t.Fatalf("expected white queenside castling to also be lost after king moves")
	}
	// Black castling rights are unaffected.
	if !parsed.castling[2] || !parsed.castling[3] {
		t.Fatalf("expected black castling rights to still be intact")
	}
}

func TestParseFENNoCastling(t *testing.T) {
	fen := "8/8/8/8/8/8/8/4K2k w - - 0 1"
	state, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	for i, right := range state.castling {
		if right {
			t.Fatalf("expected no castling rights, got castling[%d]=true", i)
		}
	}
	if state.enPassantR != -1 || state.enPassantC != -1 {
		t.Fatalf("expected no en passant square")
	}
}

func TestParseFENHalfmoveAndFullmove(t *testing.T) {
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 42 15"
	state, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	if state.halfmove != 42 {
		t.Fatalf("expected halfmove=42, got %d", state.halfmove)
	}
	if state.fullmoveNum != 15 {
		t.Fatalf("expected fullmoveNum=15, got %d", state.fullmoveNum)
	}
}

func TestParseFENErrors(t *testing.T) {
	cases := []string{
		"",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
		"rnbqkbnr/pppppppp/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1",
	}
	for _, fen := range cases {
		if _, err := ParseFEN(fen); err == nil {
			t.Errorf("expected error for FEN %q, got nil", fen)
		}
	}
}

func TestFENBlackToMove(t *testing.T) {
	fen := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	state, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}
	if state.turn != Black {
		t.Fatalf("expected black to move")
	}
	if ToFEN(state) != fen {
		t.Fatalf("FEN roundtrip failed:\ngot:  %s\nwant: %s", ToFEN(state), fen)
	}
}
