package chess

func GenerateLegalMoves(state GameState) []Move {
	pseudo := generatePseudoMoves(state)
	legal := make([]Move, 0, len(pseudo))
	color := state.turn
	for _, mv := range pseudo {
		next := ApplyMove(state, mv)
		if !IsInCheck(next, color) {
			legal = append(legal, mv)
		}
	}
	return legal
}

func generatePseudoMoves(state GameState) []Move {
	moves := make([]Move, 0, 64)
	board := state.board
	color := state.turn

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			piece := board[r][c]
			if piece == 0 || colorOf(piece) != color {
				continue
			}
			switch pieceType(piece) {
			case 'p':
				addPawnMoves(&moves, state, r, c, piece)
			case 'n':
				addKnightMoves(&moves, state, r, c, piece)
			case 'b':
				addSlidingMoves(&moves, state, r, c, piece, [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}})
			case 'r':
				addSlidingMoves(&moves, state, r, c, piece, [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}})
			case 'q':
				addSlidingMoves(&moves, state, r, c, piece, [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}, {-1, 0}, {1, 0}, {0, -1}, {0, 1}})
			case 'k':
				addKingMoves(&moves, state, r, c, piece)
			}
		}
	}

	return moves
}

func addPawnMoves(moves *[]Move, state GameState, r, c int, piece rune) {
	dir := -1
	startRow := 6
	promRow := 0
	if piece == 'p' {
		dir = 1
		startRow = 1
		promRow = 7
	}

	board := state.board
	nextR := r + dir
	if inBounds(nextR, c) && board[nextR][c] == 0 {
		addPawnAdvance(moves, r, c, nextR, c, piece, promRow)
		if r == startRow && board[r+2*dir][c] == 0 {
			*moves = append(*moves, Move{fromR: r, fromC: c, toR: r + 2*dir, toC: c})
		}
	}

	for _, dc := range []int{-1, 1} {
		cc := c + dc
		if !inBounds(nextR, cc) {
			continue
		}
		target := board[nextR][cc]
		if target != 0 && colorOf(target) == -colorOf(piece) {
			addPawnAdvance(moves, r, c, nextR, cc, piece, promRow)
		}
		if state.enPassantR == nextR && state.enPassantC == cc {
			*moves = append(*moves, Move{fromR: r, fromC: c, toR: nextR, toC: cc, isEnPassant: true})
		}
	}
}

func addPawnAdvance(moves *[]Move, fromR, fromC, toR, toC int, piece rune, promRow int) {
	if toR == promRow {
		for _, promo := range promotionPieces(colorOf(piece)) {
			*moves = append(*moves, Move{fromR: fromR, fromC: fromC, toR: toR, toC: toC, promotion: promo})
		}
		return
	}
	*moves = append(*moves, Move{fromR: fromR, fromC: fromC, toR: toR, toC: toC})
}

func addKnightMoves(moves *[]Move, state GameState, r, c int, piece rune) {
	board := state.board
	for _, d := range [][2]int{{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2}, {1, -2}, {1, 2}, {2, -1}, {2, 1}} {
		rr := r + d[0]
		cc := c + d[1]
		if !inBounds(rr, cc) {
			continue
		}
		target := board[rr][cc]
		if target == 0 || colorOf(target) != colorOf(piece) {
			*moves = append(*moves, Move{fromR: r, fromC: c, toR: rr, toC: cc})
		}
	}
}

func addSlidingMoves(moves *[]Move, state GameState, r, c int, piece rune, dirs [][2]int) {
	board := state.board
	for _, d := range dirs {
		rr := r + d[0]
		cc := c + d[1]
		for inBounds(rr, cc) {
			target := board[rr][cc]
			if target == 0 {
				*moves = append(*moves, Move{fromR: r, fromC: c, toR: rr, toC: cc})
			} else {
				if colorOf(target) != colorOf(piece) {
					*moves = append(*moves, Move{fromR: r, fromC: c, toR: rr, toC: cc})
				}
				break
			}
			rr += d[0]
			cc += d[1]
		}
	}
}

func addKingMoves(moves *[]Move, state GameState, r, c int, piece rune) {
	board := state.board
	for _, d := range [][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}} {
		rr := r + d[0]
		cc := c + d[1]
		if !inBounds(rr, cc) {
			continue
		}
		target := board[rr][cc]
		if target == 0 || colorOf(target) != colorOf(piece) {
			*moves = append(*moves, Move{fromR: r, fromC: c, toR: rr, toC: cc})
		}
	}

	color := colorOf(piece)
	if IsInCheck(state, color) {
		return
	}

	if color == White {
		if state.castling[0] && board[7][5] == 0 && board[7][6] == 0 && board[7][7] == 'R' {
			if !IsSquareAttacked(state, 7, 5, Black) && !IsSquareAttacked(state, 7, 6, Black) {
				*moves = append(*moves, Move{fromR: 7, fromC: 4, toR: 7, toC: 6, isCastle: true})
			}
		}
		if state.castling[1] && board[7][1] == 0 && board[7][2] == 0 && board[7][3] == 0 && board[7][0] == 'R' {
			if !IsSquareAttacked(state, 7, 3, Black) && !IsSquareAttacked(state, 7, 2, Black) {
				*moves = append(*moves, Move{fromR: 7, fromC: 4, toR: 7, toC: 2, isCastle: true})
			}
		}
	} else {
		if state.castling[2] && board[0][5] == 0 && board[0][6] == 0 && board[0][7] == 'r' {
			if !IsSquareAttacked(state, 0, 5, White) && !IsSquareAttacked(state, 0, 6, White) {
				*moves = append(*moves, Move{fromR: 0, fromC: 4, toR: 0, toC: 6, isCastle: true})
			}
		}
		if state.castling[3] && board[0][1] == 0 && board[0][2] == 0 && board[0][3] == 0 && board[0][0] == 'r' {
			if !IsSquareAttacked(state, 0, 3, White) && !IsSquareAttacked(state, 0, 2, White) {
				*moves = append(*moves, Move{fromR: 0, fromC: 4, toR: 0, toC: 2, isCastle: true})
			}
		}
	}
}
