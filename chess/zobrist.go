package chess

import "math/rand/v2"

// Zobrist hashing for fast position identification and transposition table lookups.

var (
	zobristPiece       [12][64]uint64 // piece index × square
	zobristBlackToMove uint64
	zobristCastling    [4]uint64
	zobristEnPassant   [8]uint64 // file only (0-7)
)

func init() {
	// Use a fixed seed so hashes are deterministic across runs.
	rng := rand.New(rand.NewPCG(0x3141592653589793, 0x2718281828459045))
	for p := 0; p < 12; p++ {
		for sq := 0; sq < 64; sq++ {
			zobristPiece[p][sq] = rng.Uint64()
		}
	}
	zobristBlackToMove = rng.Uint64()
	for i := 0; i < 4; i++ {
		zobristCastling[i] = rng.Uint64()
	}
	for i := 0; i < 8; i++ {
		zobristEnPassant[i] = rng.Uint64()
	}
}

// pieceIndex maps a piece rune to a Zobrist table index (0-11).
func pieceIndex(p rune) int {
	switch p {
	case 'P':
		return 0
	case 'N':
		return 1
	case 'B':
		return 2
	case 'R':
		return 3
	case 'Q':
		return 4
	case 'K':
		return 5
	case 'p':
		return 6
	case 'n':
		return 7
	case 'b':
		return 8
	case 'r':
		return 9
	case 'q':
		return 10
	case 'k':
		return 11
	}
	return -1
}

// ZobristHash computes the full Zobrist hash of a position from scratch.
func ZobristHash(state GameState) uint64 {
	var h uint64
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := state.board[r][c]
			if p != 0 {
				h ^= zobristPiece[pieceIndex(p)][r*8+c]
			}
		}
	}
	if state.turn == Black {
		h ^= zobristBlackToMove
	}
	for i := 0; i < 4; i++ {
		if state.castling[i] {
			h ^= zobristCastling[i]
		}
	}
	if state.enPassantR != -1 && state.enPassantC != -1 && enPassantCapturable(state) {
		h ^= zobristEnPassant[state.enPassantC]
	}
	return h
}
