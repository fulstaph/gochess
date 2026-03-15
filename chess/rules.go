package chess

import "strings"

func ApplyMove(state GameState, mv Move) GameState {
	next := state
	piece := next.board[mv.fromR][mv.fromC]
	target := next.board[mv.toR][mv.toC]
	next.board[mv.fromR][mv.fromC] = 0

	if mv.isEnPassant {
		if piece == 'P' {
			next.board[mv.toR+1][mv.toC] = 0
			target = 'p'
		} else {
			next.board[mv.toR-1][mv.toC] = 0
			target = 'P'
		}
	}

	if mv.promotion != 0 {
		next.board[mv.toR][mv.toC] = mv.promotion
	} else {
		next.board[mv.toR][mv.toC] = piece
	}

	if mv.isCastle {
		if piece == 'K' && mv.toC == 6 {
			next.board[7][5] = 'R'
			next.board[7][7] = 0
		} else if piece == 'K' && mv.toC == 2 {
			next.board[7][3] = 'R'
			next.board[7][0] = 0
		} else if piece == 'k' && mv.toC == 6 {
			next.board[0][5] = 'r'
			next.board[0][7] = 0
		} else if piece == 'k' && mv.toC == 2 {
			next.board[0][3] = 'r'
			next.board[0][0] = 0
		}
	}

	if piece == 'K' {
		next.castling[0] = false
		next.castling[1] = false
	}
	if piece == 'k' {
		next.castling[2] = false
		next.castling[3] = false
	}
	if piece == 'R' {
		if mv.fromR == 7 && mv.fromC == 0 {
			next.castling[1] = false
		} else if mv.fromR == 7 && mv.fromC == 7 {
			next.castling[0] = false
		}
	}
	if piece == 'r' {
		if mv.fromR == 0 && mv.fromC == 0 {
			next.castling[3] = false
		} else if mv.fromR == 0 && mv.fromC == 7 {
			next.castling[2] = false
		}
	}
	if target == 'R' {
		if mv.toR == 7 && mv.toC == 0 {
			next.castling[1] = false
		} else if mv.toR == 7 && mv.toC == 7 {
			next.castling[0] = false
		}
	}
	if target == 'r' {
		if mv.toR == 0 && mv.toC == 0 {
			next.castling[3] = false
		} else if mv.toR == 0 && mv.toC == 7 {
			next.castling[2] = false
		}
	}

	next.enPassantR = -1
	next.enPassantC = -1
	if piece == 'P' && mv.fromR == 6 && mv.toR == 4 {
		next.enPassantR = 5
		next.enPassantC = mv.fromC
	} else if piece == 'p' && mv.fromR == 1 && mv.toR == 3 {
		next.enPassantR = 2
		next.enPassantC = mv.fromC
	}

	if piece == 'P' || piece == 'p' || target != 0 || mv.isEnPassant {
		next.halfmove = 0
	} else {
		next.halfmove++
	}

	if state.turn == Black {
		next.fullmoveNum++
	}
	next.turn = -state.turn
	return next
}

func IsInCheck(state GameState, color int) bool {
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := state.board[r][c]
			if p == 0 {
				continue
			}
			if color == White && p == 'K' {
				return IsSquareAttacked(state, r, c, Black)
			}
			if color == Black && p == 'k' {
				return IsSquareAttacked(state, r, c, White)
			}
		}
	}
	return false
}

func IsSquareAttacked(state GameState, r, c int, byColor int) bool {
	board := state.board

	if byColor == White {
		if inBounds(r+1, c-1) && board[r+1][c-1] == 'P' {
			return true
		}
		if inBounds(r+1, c+1) && board[r+1][c+1] == 'P' {
			return true
		}
	} else {
		if inBounds(r-1, c-1) && board[r-1][c-1] == 'p' {
			return true
		}
		if inBounds(r-1, c+1) && board[r-1][c+1] == 'p' {
			return true
		}
	}

	knight := 'N'
	if byColor == Black {
		knight = 'n'
	}
	for _, d := range [][2]int{{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2}, {1, -2}, {1, 2}, {2, -1}, {2, 1}} {
		rr := r + d[0]
		cc := c + d[1]
		if inBounds(rr, cc) && board[rr][cc] == knight {
			return true
		}
	}

	if isLineAttacked(state, r, c, byColor, [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}, "bq") {
		return true
	}
	if isLineAttacked(state, r, c, byColor, [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}, "rq") {
		return true
	}

	king := 'K'
	if byColor == Black {
		king = 'k'
	}
	for _, d := range [][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}} {
		rr := r + d[0]
		cc := c + d[1]
		if inBounds(rr, cc) && board[rr][cc] == king {
			return true
		}
	}

	return false
}

func isLineAttacked(state GameState, r, c int, byColor int, dirs [][2]int, pieces string) bool {
	board := state.board
	for _, d := range dirs {
		rr := r + d[0]
		cc := c + d[1]
		for inBounds(rr, cc) {
			p := board[rr][cc]
			if p != 0 {
				if colorOf(p) == byColor && strings.ContainsRune(pieces, pieceType(p)) {
					return true
				}
				break
			}
			rr += d[0]
			cc += d[1]
		}
	}
	return false
}
