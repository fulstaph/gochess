package chess

const (
	mateScore = 100_000
	inf       = 10_000_000
)

// BestMove returns a simple minimax-selected move for the side to move.
// Depth is clamped to at least 1; higher values play stronger but slower.
func BestMove(state GameState, depth int) (Move, bool) {
	if depth < 1 {
		depth = 1
	}

	moves := GenerateLegalMoves(state)
	if len(moves) == 0 {
		return Move{}, false
	}

	bestScore := -inf
	bestMove := moves[0]
	alpha := -inf
	beta := inf

	// Simple move ordering: checks first, then captures
	// For now, we just use the generator's order, but a real engine would sort.

	for _, mv := range moves {
		next := ApplyMove(state, mv)
		score := -negamax(next, depth-1, -beta, -alpha)
		if score > bestScore {
			bestScore = score
			bestMove = mv
		}
		if score > alpha {
			alpha = score
		}
	}

	return bestMove, true
}

func negamax(state GameState, depth, alpha, beta int) int {
	if depth == 0 {
		if IsInCheck(state, state.turn) {
			moves := GenerateLegalMoves(state)
			if len(moves) == 0 {
				return -mateScore
			}
		}
		return quiescence(state, alpha, beta)
	}

	moves := GenerateLegalMoves(state)
	if len(moves) == 0 {
		return evaluateTerminal(state, moves)
	}

	best := -inf
	for _, mv := range moves {
		next := ApplyMove(state, mv)
		score := -negamax(next, depth-1, -beta, -alpha)
		if score > best {
			best = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break
		}
	}
	return best
}

func quiescence(state GameState, alpha, beta int) int {
	standPat := evaluate(state)
	if standPat >= beta {
		return beta
	}
	if standPat > alpha {
		alpha = standPat
	}

	moves := GenerateLegalMoves(state)
	// Filter for captures only
	// In a real engine, we'd have a separate generator for captures
	captures := make([]Move, 0, len(moves))
	for _, mv := range moves {
		if isCapture(state, mv) {
			captures = append(captures, mv)
		}
	}

	for _, mv := range captures {
		next := ApplyMove(state, mv)
		score := -quiescence(next, -beta, -alpha)
		if score >= beta {
			return beta
		}
		if score > alpha {
			alpha = score
		}
	}
	return alpha
}

func isCapture(state GameState, mv Move) bool {
	if mv.isEnPassant {
		return true
	}
	return state.board[mv.toR][mv.toC] != 0
}

func evaluateTerminal(state GameState, moves []Move) int {
	if len(moves) == 0 {
		if IsInCheck(state, state.turn) {
			return -mateScore
		}
		return 0
	}
	return evaluate(state)
}

func evaluate(state GameState) int {
	board := state.board
	total := 0

	// MG = Middlegame values
	mgPawn := 100
	mgKnight := 320
	mgBishop := 330
	mgRook := 500
	mgQueen := 900

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := board[r][c]
			if p == 0 {
				continue
			}
			
			val := 0
			pstVal := 0
			
			// Get piece value and PST bonus
			switch pieceType(p) {
			case 'p':
				val = mgPawn
				pstVal = pstPawn[mirrorRow(r, colorOf(p))][c]
			case 'n':
				val = mgKnight
				pstVal = pstKnight[mirrorRow(r, colorOf(p))][c]
			case 'b':
				val = mgBishop
				pstVal = pstBishop[mirrorRow(r, colorOf(p))][c]
			case 'r':
				val = mgRook
				pstVal = pstRook[mirrorRow(r, colorOf(p))][c]
			case 'q':
				val = mgQueen
				pstVal = pstQueen[mirrorRow(r, colorOf(p))][c]
			case 'k':
				pstVal = pstKingMid[mirrorRow(r, colorOf(p))][c]
			}

			if colorOf(p) == White {
				total += val + pstVal
			} else {
				total -= (val + pstVal)
			}
		}
	}

	return total * state.turn
}

func mirrorRow(r int, color int) int {
	if color == White {
		return r
	}
	return 7 - r
}

// Piece-Square Tables (from White's perspective)
// Flipped for Black by mirrorRow helper.
// Based on simplified PeSTO tables.

var pstPawn = [8][8]int{
	{0, 0, 0, 0, 0, 0, 0, 0},
	{50, 50, 50, 50, 50, 50, 50, 50},
	{10, 10, 20, 30, 30, 20, 10, 10},
	{5, 5, 10, 25, 25, 10, 5, 5},
	{0, 0, 0, 20, 20, 0, 0, 0},
	{5, -5, -10, 0, 0, -10, -5, 5},
	{5, 10, 10, -20, -20, 10, 10, 5},
	{0, 0, 0, 0, 0, 0, 0, 0},
}

var pstKnight = [8][8]int{
	{-50, -40, -30, -30, -30, -30, -40, -50},
	{-40, -20, 0, 0, 0, 0, -20, -40},
	{-30, 0, 10, 15, 15, 10, 0, -30},
	{-30, 5, 15, 20, 20, 15, 5, -30},
	{-30, 0, 15, 20, 20, 15, 0, -30},
	{-30, 5, 10, 15, 15, 10, 5, -30},
	{-40, -20, 0, 5, 5, 0, -20, -40},
	{-50, -40, -30, -30, -30, -30, -40, -50},
}

var pstBishop = [8][8]int{
	{-20, -10, -10, -10, -10, -10, -10, -20},
	{-10, 0, 0, 0, 0, 0, 0, -10},
	{-10, 0, 5, 10, 10, 5, 0, -10},
	{-10, 5, 5, 10, 10, 5, 5, -10},
	{-10, 0, 10, 10, 10, 10, 0, -10},
	{-10, 10, 10, 10, 10, 10, 10, -10},
	{-10, 5, 0, 0, 0, 0, 5, -10},
	{-20, -10, -10, -10, -10, -10, -10, -20},
}

var pstRook = [8][8]int{
	{0, 0, 0, 0, 0, 0, 0, 0},
	{5, 10, 10, 10, 10, 10, 10, 5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{0, 0, 0, 5, 5, 0, 0, 0},
}

var pstQueen = [8][8]int{
	{-20, -10, -10, -5, -5, -10, -10, -20},
	{-10, 0, 0, 0, 0, 0, 0, -10},
	{-10, 0, 5, 5, 5, 5, 0, -10},
	{-5, 0, 5, 5, 5, 5, 0, -5},
	{0, 0, 5, 5, 5, 5, 0, -5},
	{-10, 5, 5, 5, 5, 5, 0, -10},
	{-10, 0, 5, 0, 0, 0, 0, -10},
	{-20, -10, -10, -5, -5, -10, -10, -20},
}

var pstKingMid = [8][8]int{
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-20, -30, -30, -40, -40, -30, -30, -20},
	{-10, -20, -20, -20, -20, -20, -20, -10},
	{20, 20, 0, 0, 0, 0, 20, 20},
	{20, 30, 10, 0, 0, 10, 30, 20},
}
