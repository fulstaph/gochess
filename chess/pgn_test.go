package chess

import (
	"strings"
	"testing"
)

func TestFormatSANPawnMove(t *testing.T) {
	state := InitialState()
	mv, err := ParseMove("e2e4", state)
	if err != nil {
		t.Fatalf("ParseMove: %v", err)
	}
	got := FormatSAN(mv, state)
	if got != "e4" {
		t.Fatalf("expected \"e4\", got %q", got)
	}
}

func TestFormatSANKnightMove(t *testing.T) {
	state := InitialState()
	mv, err := ParseMove("g1f3", state)
	if err != nil {
		t.Fatalf("ParseMove: %v", err)
	}
	got := FormatSAN(mv, state)
	if got != "Nf3" {
		t.Fatalf("expected \"Nf3\", got %q", got)
	}
}

func TestFormatSANCastle(t *testing.T) {
	state := InitialState()
	for _, m := range []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1e2", "g8f6"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseMove("o-o", state)
	if err != nil {
		t.Fatalf("ParseMove O-O: %v", err)
	}
	got := FormatSAN(mv, state)
	if got != "O-O" {
		t.Fatalf("expected \"O-O\", got %q", got)
	}
}

func TestFormatSANCapture(t *testing.T) {
	state := InitialState()
	for _, m := range []string{"e2e4", "d7d5"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseMove("e4d5", state)
	if err != nil {
		t.Fatalf("ParseMove e4d5: %v", err)
	}
	got := FormatSAN(mv, state)
	if got != "exd5" {
		t.Fatalf("expected \"exd5\", got %q", got)
	}
}

func TestFormatSANPromotion(t *testing.T) {
	state := GameState{
		turn:       White,
		enPassantR: -1,
		enPassantC: -1,
	}
	state.board[7][4] = 'K'
	state.board[0][0] = 'k'
	state.board[1][4] = 'P'

	mv, err := ParseMove("e7e8q", state)
	if err != nil {
		t.Fatalf("ParseMove e7e8q: %v", err)
	}
	got := FormatSAN(mv, state)
	// The promoted queen gives check to the black king on a1.
	if got != "e8=Q+" {
		t.Fatalf("expected \"e8=Q+\", got %q", got)
	}
}

func TestFormatSANCheck(t *testing.T) {
	// Fool's mate setup: after f3, e5, g4, Qh4 is checkmate.
	state := InitialState()
	for _, m := range []string{"f2f3", "e7e5", "g2g4"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseMove("d8h4", state)
	if err != nil {
		t.Fatalf("ParseMove d8h4: %v", err)
	}
	got := FormatSAN(mv, state)
	if got != "Qh4#" {
		t.Fatalf("expected \"Qh4#\", got %q", got)
	}
}

func TestFormatSANDisambiguation(t *testing.T) {
	// Place two rooks on a4 and h4 (white), kings in corners.
	// Both rooks can reach d4 — requires file disambiguation.
	state := GameState{
		turn:        White,
		enPassantR:  -1,
		enPassantC:  -1,
		fullmoveNum: 1,
	}
	// Kings on e1/e8 (not on any rank/file used by the rooks).
	state.board[7][4] = 'K' // e1
	state.board[0][4] = 'k' // e8
	state.board[4][0] = 'R' // a4
	state.board[4][7] = 'R' // h4

	// Ra4 to d4 (fromC=0 -> col a, toC=3 -> col d)
	mv := Move{fromR: 4, fromC: 0, toR: 4, toC: 3}
	got := FormatSAN(mv, state)
	if got != "Rad4" {
		t.Fatalf("expected \"Rad4\", got %q", got)
	}

	// Rh4 to d4
	mv2 := Move{fromR: 4, fromC: 7, toR: 4, toC: 3}
	got2 := FormatSAN(mv2, state)
	if got2 != "Rhd4" {
		t.Fatalf("expected \"Rhd4\", got %q", got2)
	}
}

func TestFormatPGNFoolsMate(t *testing.T) {
	state := InitialState()
	moves := []string{"f2f3", "e7e5", "g2g4", "d8h4"}
	mvList := make([]Move, 0, len(moves))
	for _, m := range moves {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		mvList = append(mvList, mv)
		state = ApplyMove(state, mv)
	}

	pgn := FormatPGN(mvList, InitialState(), nil, "0-1")

	if !strings.Contains(pgn, "[Result \"0-1\"]") {
		t.Fatalf("PGN missing Result tag:\n%s", pgn)
	}
	if !strings.Contains(pgn, "1. f3 e5 2. g4 Qh4#") {
		t.Fatalf("PGN moves incorrect:\n%s", pgn)
	}
	if !strings.Contains(pgn, "0-1") {
		t.Fatalf("PGN missing result token:\n%s", pgn)
	}
}

func TestFormatPGNHeaders(t *testing.T) {
	pgn := FormatPGN(nil, InitialState(), map[string]string{
		"White": "Alice",
		"Black": "Bob",
		"Event": "Test Match",
	}, "1/2-1/2")

	if !strings.Contains(pgn, `[White "Alice"]`) {
		t.Fatalf("PGN missing White header:\n%s", pgn)
	}
	if !strings.Contains(pgn, `[Black "Bob"]`) {
		t.Fatalf("PGN missing Black header:\n%s", pgn)
	}
	if !strings.Contains(pgn, `[Event "Test Match"]`) {
		t.Fatalf("PGN missing Event header:\n%s", pgn)
	}
}

// ---- ParseSAN tests ----

func TestParseSAN_PawnMove(t *testing.T) {
	state := InitialState()
	mv, err := ParseSAN("e4", state)
	if err != nil {
		t.Fatalf("ParseSAN e4: %v", err)
	}
	if mv.fromC != 4 || mv.toR != 4 || mv.toC != 4 {
		t.Fatalf("expected e2→e4, got from=(%d,%d) to=(%d,%d)", mv.fromR, mv.fromC, mv.toR, mv.toC)
	}
}

func TestParseSAN_PawnCapture(t *testing.T) {
	state := InitialState()
	for _, m := range []string{"e2e4", "d7d5"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseSAN("exd5", state)
	if err != nil {
		t.Fatalf("ParseSAN exd5: %v", err)
	}
	if mv.toR != 3 || mv.toC != 3 {
		t.Fatalf("expected capture on d5, got to=(%d,%d)", mv.toR, mv.toC)
	}
}

func TestParseSAN_Promotion(t *testing.T) {
	state := GameState{turn: White, enPassantR: -1, enPassantC: -1}
	state.board[7][4] = 'K'
	state.board[0][0] = 'k'
	state.board[1][4] = 'P'

	mv, err := ParseSAN("e8=Q", state)
	if err != nil {
		t.Fatalf("ParseSAN e8=Q: %v", err)
	}
	if pieceType(mv.promotion) != 'q' {
		t.Fatalf("expected queen promotion, got %c", mv.promotion)
	}

	mv2, err := ParseSAN("e8=R", state)
	if err != nil {
		t.Fatalf("ParseSAN e8=R: %v", err)
	}
	if pieceType(mv2.promotion) != 'r' {
		t.Fatalf("expected rook promotion, got %c", mv2.promotion)
	}
}

func TestParseSAN_KnightMove(t *testing.T) {
	state := InitialState()
	mv, err := ParseSAN("Nf3", state)
	if err != nil {
		t.Fatalf("ParseSAN Nf3: %v", err)
	}
	if mv.toR != 5 || mv.toC != 5 {
		t.Fatalf("expected Nf3 to (5,5), got to=(%d,%d)", mv.toR, mv.toC)
	}
}

func TestParseSAN_Disambiguation(t *testing.T) {
	state := GameState{turn: White, enPassantR: -1, enPassantC: -1, fullmoveNum: 1}
	state.board[7][4] = 'K'
	state.board[0][4] = 'k'
	state.board[4][0] = 'R' // Ra4
	state.board[4][7] = 'R' // Rh4

	mv, err := ParseSAN("Rad4", state)
	if err != nil {
		t.Fatalf("ParseSAN Rad4: %v", err)
	}
	if mv.fromC != 0 {
		t.Fatalf("expected rook from file a (col 0), got col %d", mv.fromC)
	}

	mv2, err := ParseSAN("Rhd4", state)
	if err != nil {
		t.Fatalf("ParseSAN Rhd4: %v", err)
	}
	if mv2.fromC != 7 {
		t.Fatalf("expected rook from file h (col 7), got col %d", mv2.fromC)
	}
}

func TestParseSAN_Castling(t *testing.T) {
	state := InitialState()
	for _, m := range []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1e2", "g8f6"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseSAN("O-O", state)
	if err != nil {
		t.Fatalf("ParseSAN O-O: %v", err)
	}
	if !mv.isCastle || mv.toC != 6 {
		t.Fatalf("expected kingside castle, got isCastle=%v toC=%d", mv.isCastle, mv.toC)
	}
}

func TestParseSAN_CheckAnnotation(t *testing.T) {
	// After f3, e5, g4 — Black can play Qh4# (with check annotation).
	state := InitialState()
	for _, m := range []string{"f2f3", "e7e5", "g2g4"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseSAN("Qh4#", state)
	if err != nil {
		t.Fatalf("ParseSAN Qh4#: %v", err)
	}
	if mv.toR != 4 || mv.toC != 7 {
		t.Fatalf("expected Qh4, got to=(%d,%d)", mv.toR, mv.toC)
	}
}

// ---- ParsePGN tests ----

func TestParsePGN_FoolsMate(t *testing.T) {
	pgn := `[Event "Test"]
[Result "0-1"]

1. f3 e5 2. g4 Qh4# 0-1
`
	moves, initial, headers, err := ParsePGN(pgn)
	if err != nil {
		t.Fatalf("ParsePGN: %v", err)
	}
	if len(moves) != 4 {
		t.Fatalf("expected 4 moves, got %d", len(moves))
	}
	if headers["Event"] != "Test" {
		t.Fatalf("expected Event=Test, got %q", headers["Event"])
	}
	if initial.turn != White {
		t.Fatal("expected initial state with White to move")
	}

	// Apply all moves and verify checkmate.
	state := initial
	for _, mv := range moves {
		state = ApplyMove(state, mv)
	}
	legal := GenerateLegalMoves(state)
	if len(legal) != 0 || !IsInCheck(state, state.turn) {
		t.Fatal("expected checkmate position after fool's mate")
	}
}

func TestParsePGN_WithFEN(t *testing.T) {
	pgn := `[FEN "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1"]

1. e4 Kd7 *
`
	moves, initial, _, err := ParsePGN(pgn)
	if err != nil {
		t.Fatalf("ParsePGN: %v", err)
	}
	if len(moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(moves))
	}
	if initial.board[6][4] != 'P' {
		t.Fatal("expected pawn on e2 from FEN")
	}
}

func TestParsePGN_WithComments(t *testing.T) {
	pgn := `[Result "*"]

1. e4 {A strong move} e5 2. Nf3 *
`
	moves, _, _, err := ParsePGN(pgn)
	if err != nil {
		t.Fatalf("ParsePGN: %v", err)
	}
	if len(moves) != 3 {
		t.Fatalf("expected 3 moves, got %d", len(moves))
	}
}

func TestParsePGN_RoundTrip(t *testing.T) {
	// Format a game, then parse it back and verify.
	state := InitialState()
	inputMoves := []string{"e2e4", "e7e5", "g1f3", "b8c6"}
	mvList := make([]Move, 0, len(inputMoves))
	for _, m := range inputMoves {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		mvList = append(mvList, mv)
		state = ApplyMove(state, mv)
	}

	pgn := FormatPGN(mvList, InitialState(), nil, "*")
	parsedMoves, _, _, err := ParsePGN(pgn)
	if err != nil {
		t.Fatalf("ParsePGN round-trip: %v", err)
	}
	if len(parsedMoves) != len(mvList) {
		t.Fatalf("round-trip: expected %d moves, got %d", len(mvList), len(parsedMoves))
	}
	for i := range mvList {
		if parsedMoves[i] != mvList[i] {
			t.Fatalf("round-trip: move %d mismatch: got %v, want %v", i, parsedMoves[i], mvList[i])
		}
	}
}

func TestFormatSANEnPassant(t *testing.T) {
	state := InitialState()
	for _, m := range []string{"e2e4", "a7a5", "e4e5", "d7d5"} {
		mv, err := ParseMove(m, state)
		if err != nil {
			t.Fatalf("ParseMove %s: %v", m, err)
		}
		state = ApplyMove(state, mv)
	}
	mv, err := ParseMove("e5d6", state)
	if err != nil {
		t.Fatalf("ParseMove e5d6: %v", err)
	}
	got := FormatSAN(mv, state)
	if got != "exd6" {
		t.Fatalf("expected \"exd6\", got %q", got)
	}
}
