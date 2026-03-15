package chess

import "strings"

// PositionKey returns a repetition key that includes board, side to move,
// castling rights, and en passant square.
func PositionKey(state GameState) string {
	var b strings.Builder
	b.Grow(90)

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := state.board[r][c]
			if p == 0 {
				b.WriteByte('.')
			} else {
				b.WriteRune(p)
			}
		}
		if r != 7 {
			b.WriteByte('/')
		}
	}

	b.WriteByte(' ')
	if state.turn == White {
		b.WriteByte('w')
	} else {
		b.WriteByte('b')
	}

	b.WriteByte(' ')
	rights := castlingRights(state)
	if rights == "" {
		b.WriteByte('-')
	} else {
		b.WriteString(rights)
	}

	b.WriteByte(' ')
	if state.enPassantR == -1 || state.enPassantC == -1 || !enPassantCapturable(state) {
		b.WriteByte('-')
	} else {
		b.WriteString(formatSquare(state.enPassantR, state.enPassantC))
	}

	return b.String()
}

// IsFiftyMoveDraw returns true when the fifty-move rule applies.
func IsFiftyMoveDraw(state GameState) bool {
	return state.halfmove >= 100
}

// IsSeventyFiveMoveDraw returns true when the automatic 75-move rule applies.
func IsSeventyFiveMoveDraw(state GameState) bool {
	return state.halfmove >= 150
}

// enPassantCapturable reports whether the recorded en passant target square
// can actually be captured by the side to move. If not, the square must not
// participate in repetition hashing per FIDE rules.
func enPassantCapturable(state GameState) bool {
	if state.enPassantR == -1 || state.enPassantC == -1 {
		return false
	}

	// White captures upward (row-1), Black downward (row+1). The pawn that
	// could capture must be on the adjacent file and one rank away from the
	// target square.
	targetR := state.enPassantR
	targetC := state.enPassantC

	if state.turn == White {
		pawnRow := targetR + 1
		if pawnRow >= 0 && pawnRow < 8 {
			if (targetC > 0 && state.board[pawnRow][targetC-1] == 'P') ||
				(targetC < 7 && state.board[pawnRow][targetC+1] == 'P') {
				return true
			}
		}
	} else {
		pawnRow := targetR - 1
		if pawnRow >= 0 && pawnRow < 8 {
			if (targetC > 0 && state.board[pawnRow][targetC-1] == 'p') ||
				(targetC < 7 && state.board[pawnRow][targetC+1] == 'p') {
				return true
			}
		}
	}

	return false
}

func castlingRights(state GameState) string {
	var b strings.Builder
	if state.castling[0] {
		b.WriteByte('K')
	}
	if state.castling[1] {
		b.WriteByte('Q')
	}
	if state.castling[2] {
		b.WriteByte('k')
	}
	if state.castling[3] {
		b.WriteByte('q')
	}
	return b.String()
}
