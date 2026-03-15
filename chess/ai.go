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
	moves := GenerateLegalMoves(state)
	if depth == 0 || len(moves) == 0 {
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

func evaluateTerminal(state GameState, moves []Move) int {
	if len(moves) == 0 {
		if IsInCheck(state, state.turn) {
			return -mateScore
		}
		return 0
	}
	return evaluateMaterial(state)
}

func evaluateMaterial(state GameState) int {
	board := state.board
	total := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := board[r][c]
			if p == 0 {
				continue
			}
			total += pieceValue(p) * colorOf(p)
		}
	}

	return total * state.turn
}

func pieceValue(piece rune) int {
	switch pieceType(piece) {
	case 'p':
		return 100
	case 'n':
		return 320
	case 'b':
		return 330
	case 'r':
		return 500
	case 'q':
		return 900
	default:
		return 0
	}
}
