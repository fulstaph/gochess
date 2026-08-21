// Package chess implements the core chess engine: move generation, rules,
// evaluation, and AI search (minimax with alpha-beta pruning).
package chess

import (
	"context"
	"sort"
)

const (
	mateScore  = 100_000
	inf        = 10_000_000
	maxQSDepth = 8

	mgPawn   = 100
	mgKnight = 320
	mgBishop = 330
	mgRook   = 500
	mgQueen  = 900

	// Phase weights for tapered evaluation.
	phaseKnight = 1
	phaseBishop = 1
	phaseRook   = 2
	phaseQueen  = 4
	phaseTotal  = 4*phaseKnight + 4*phaseBishop + 4*phaseRook + 2*phaseQueen // 24

	// Transposition table size: 2^20 entries ≈ 1M entries.
	ttLogSize = 20
)

// searchContext holds per-search mutable state for move ordering heuristics.
type searchContext struct {
	killers  [64][2]Move // 2 killer moves per ply
	history  [64][64]int // history[fromSq][toSq] counters
	maxDepth int         // root search depth (used to compute ply)
	tt       *transTable // transposition table
	nodes    int         // node counter for periodic cancellation checks
	ctx      context.Context
}

func sqIndex(r, c int) int { return r*8 + c }

// BestMove uses iterative deepening with alpha-beta pruning.
// Each iteration seeds the next with the previous best move for better ordering.
func BestMove(state GameState, depth int) (Move, bool) {
	return BestMoveCtx(context.Background(), state, depth)
}

// BestMoveCtx is like BestMove but accepts a context for cancellation.
// When the context is cancelled, it returns the best move found so far.
func BestMoveCtx(ctx context.Context, state GameState, depth int) (Move, bool) {
	if depth < 1 {
		depth = 1
	}

	moves := GenerateLegalMoves(state)
	if len(moves) == 0 {
		return Move{}, false
	}

	bestMove := moves[0]
	sc := &searchContext{
		tt:  newTransTable(ttLogSize),
		ctx: ctx,
	}

	// Iterative deepening: search d=1,2,...,depth, using previous best for ordering.
	for d := 1; d <= depth; d++ {
		sc.maxDepth = d

		// Decay history heuristic between iterations to age stale entries.
		for i := range sc.history {
			for j := range sc.history[i] {
				sc.history[i][j] /= 2
			}
		}

		ordered := orderMoves(state, moves, bestMove, sc, d)
		alpha := -inf
		beta := inf
		iterBest := ordered[0]
		iterScore := -inf

		cancelled := false
		for _, mv := range ordered {
			next := ApplyMove(state, mv)
			score := -negamax(next, d-1, -beta, -alpha, sc)
			// Check for cancellation — keep best from previous complete iteration.
			if ctx.Err() != nil {
				cancelled = true
				break
			}
			if score > iterScore {
				iterScore = score
				iterBest = mv
			}
			if score > alpha {
				alpha = score
			}
		}
		if !cancelled {
			bestMove = iterBest
		}
		if cancelled {
			break
		}
	}

	return bestMove, true
}

func negamax(state GameState, depth, alpha, beta int, sc *searchContext) int {
	// Periodic cancellation check (every 4096 nodes).
	sc.nodes++
	if sc.nodes&4095 == 0 && sc.ctx.Err() != nil {
		return 0
	}

	moves := GenerateLegalMoves(state)
	if len(moves) == 0 {
		return evaluateTerminal(state, moves)
	}
	if depth == 0 {
		return quiescence(state, alpha, beta, maxQSDepth)
	}

	hash := ZobristHash(state)
	alphaOrig := alpha

	// Transposition table probe.
	pvMove := Move{}
	if entry, ok := sc.tt.probe(hash); ok {
		pvMove = entry.best
		if entry.depth >= depth {
			switch entry.flag {
			case ttExact:
				return entry.score
			case ttAlpha:
				if entry.score < beta {
					beta = entry.score
				}
			case ttBeta:
				if entry.score > alpha {
					alpha = entry.score
				}
			}
			if alpha >= beta {
				return entry.score
			}
		}
	}

	ply := sc.maxDepth - depth
	ordered := orderMoves(state, moves, pvMove, sc, depth)
	best := -inf
	bestMv := ordered[0]
	for _, mv := range ordered {
		next := ApplyMove(state, mv)
		score := -negamax(next, depth-1, -beta, -alpha, sc)
		if score > best {
			best = score
			bestMv = mv
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			// Record killer and history on beta cutoff.
			if !isCapture(state, mv) && ply < 64 {
				sc.killers[ply][1] = sc.killers[ply][0]
				sc.killers[ply][0] = mv
				from := sqIndex(mv.fromR, mv.fromC)
				to := sqIndex(mv.toR, mv.toC)
				sc.history[from][to] += depth * depth
			}
			break
		}
	}

	// Store in transposition table.
	var flag ttFlag
	if best <= alphaOrig {
		flag = ttAlpha
	} else if best >= beta {
		flag = ttBeta
	} else {
		flag = ttExact
	}
	sc.tt.store(hash, depth, best, flag, bestMv)

	return best
}

// orderMoves sorts moves: pvMove first, then captures (MVV-LVA), then killers, then quiet moves (by history).
func orderMoves(state GameState, moves []Move, pvMove Move, sc *searchContext, depth int) []Move {
	ordered := make([]Move, 0, len(moves))

	// PV move from previous iteration goes first.
	hasPV := pvMove != (Move{})
	if hasPV {
		for _, mv := range moves {
			if mv == pvMove {
				ordered = append(ordered, mv)
				break
			}
		}
	}

	captures := make([]Move, 0, 8)
	killerMoves := make([]Move, 0, 2)
	quiet := make([]Move, 0, len(moves))

	ply := 0
	if sc != nil {
		ply = sc.maxDepth - depth
		if ply < 0 {
			ply = 0
		}
	}

	for _, mv := range moves {
		if hasPV && mv == pvMove {
			continue
		}
		if isCapture(state, mv) {
			captures = append(captures, mv)
		} else if sc != nil && ply < 64 && (mv == sc.killers[ply][0] || mv == sc.killers[ply][1]) {
			killerMoves = append(killerMoves, mv)
		} else {
			quiet = append(quiet, mv)
		}
	}

	// Sort captures by MVV-LVA: highest victim value, lowest attacker value.
	sort.Slice(captures, func(i, j int) bool {
		return mvvLva(state, captures[i]) > mvvLva(state, captures[j])
	})

	// Sort quiet moves by history score (descending).
	if sc != nil {
		sort.Slice(quiet, func(i, j int) bool {
			si := sc.history[sqIndex(quiet[i].fromR, quiet[i].fromC)][sqIndex(quiet[i].toR, quiet[i].toC)]
			sj := sc.history[sqIndex(quiet[j].fromR, quiet[j].fromC)][sqIndex(quiet[j].toR, quiet[j].toC)]
			return si > sj
		})
	}

	ordered = append(ordered, captures...)
	ordered = append(ordered, killerMoves...)
	ordered = append(ordered, quiet...)
	return ordered
}

// mvvLva scores a capture: victim value * 10 - attacker value.
func mvvLva(state GameState, mv Move) int {
	var victim rune
	if mv.isEnPassant {
		victim = 'p' // pawn
	} else {
		victim = state.board[mv.toR][mv.toC]
	}
	attacker := state.board[mv.fromR][mv.fromC]
	return runeValue(victim)*10 - runeValue(attacker)
}

// runeValue returns the approximate centipawn value of a piece rune (case-insensitive).
func runeValue(p rune) int {
	switch pieceType(p) {
	case 'p':
		return mgPawn
	case 'n':
		return mgKnight
	case 'b':
		return mgBishop
	case 'r':
		return mgRook
	case 'q':
		return mgQueen
	case 'k':
		return mateScore
	}
	return 0
}

func quiescence(state GameState, alpha, beta, qsDepth int) int {
	if qsDepth <= 0 {
		return evaluate(state)
	}

	standPat := evaluate(state)
	if standPat >= beta {
		return beta
	}
	if standPat > alpha {
		alpha = standPat
	}

	for _, mv := range GenerateLegalMoves(state) {
		if !isCapture(state, mv) {
			continue
		}
		next := ApplyMove(state, mv)
		score := -quiescence(next, -beta, -alpha, qsDepth-1)
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
	mgTotal := 0
	egTotal := 0
	phase := 0

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := board[r][c]
			if p == 0 {
				continue
			}

			mgVal := 0
			egVal := 0
			mgPst := 0
			egPst := 0

			mr := mirrorRow(r, colorOf(p))

			switch pieceType(p) {
			case 'p':
				mgVal = mgPawn
				egVal = mgPawn
				mgPst = pstPawn[mr][c]
				egPst = pstPawn[mr][c]
			case 'n':
				mgVal = mgKnight
				egVal = mgKnight
				mgPst = pstKnight[mr][c]
				egPst = pstKnight[mr][c]
				phase += phaseKnight
			case 'b':
				mgVal = mgBishop
				egVal = mgBishop
				mgPst = pstBishop[mr][c]
				egPst = pstBishop[mr][c]
				phase += phaseBishop
			case 'r':
				mgVal = mgRook
				egVal = mgRook
				mgPst = pstRook[mr][c]
				egPst = pstRook[mr][c]
				phase += phaseRook
			case 'q':
				mgVal = mgQueen
				egVal = mgQueen
				mgPst = pstQueen[mr][c]
				egPst = pstQueen[mr][c]
				phase += phaseQueen
			case 'k':
				mgPst = pstKingMid[mr][c]
				egPst = pstKingEnd[mr][c]
			}

			if colorOf(p) == White {
				mgTotal += mgVal + mgPst
				egTotal += egVal + egPst
			} else {
				mgTotal -= (mgVal + mgPst)
				egTotal -= (egVal + egPst)
			}
		}
	}

	// Tapered evaluation: blend midgame and endgame scores by phase.
	if phase > phaseTotal {
		phase = phaseTotal
	}
	score := (mgTotal*phase + egTotal*(phaseTotal-phase)) / phaseTotal

	// Pawn structure evaluation.
	score += evaluatePawnStructure(board)

	return score * state.turn
}

// evaluatePawnStructure returns a score bonus/penalty from White's perspective
// based on doubled, isolated, and passed pawns.
func evaluatePawnStructure(board [8][8]rune) int {
	score := 0

	// Count pawns per file for each color.
	var whitePawns [8]int
	var blackPawns [8]int
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			switch board[r][c] {
			case 'P':
				whitePawns[c]++
			case 'p':
				blackPawns[c]++
			}
		}
	}

	// Doubled pawns penalty.
	for c := 0; c < 8; c++ {
		if whitePawns[c] > 1 {
			score -= 15 * (whitePawns[c] - 1)
		}
		if blackPawns[c] > 1 {
			score += 15 * (blackPawns[c] - 1)
		}
	}

	// Isolated and passed pawns.
	for c := 0; c < 8; c++ {
		hasLeft := c > 0
		hasRight := c < 7

		// White isolated pawns.
		if whitePawns[c] > 0 {
			friendly := false
			if hasLeft && whitePawns[c-1] > 0 {
				friendly = true
			}
			if hasRight && whitePawns[c+1] > 0 {
				friendly = true
			}
			if !friendly {
				score -= 10
			}
		}

		// Black isolated pawns.
		if blackPawns[c] > 0 {
			friendly := false
			if hasLeft && blackPawns[c-1] > 0 {
				friendly = true
			}
			if hasRight && blackPawns[c+1] > 0 {
				friendly = true
			}
			if !friendly {
				score += 10
			}
		}

		// Passed pawns: no opposing pawns on same or adjacent files ahead.
		if whitePawns[c] > 0 {
			passed := true
			// Find most advanced white pawn on this file.
			advRow := 7
			for r := 0; r < 8; r++ {
				if board[r][c] == 'P' && r < advRow {
					advRow = r
				}
			}
			for checkC := max(0, c-1); checkC <= min(7, c+1); checkC++ {
				for r := 0; r < advRow; r++ {
					if board[r][checkC] == 'p' {
						passed = false
						break
					}
				}
				if !passed {
					break
				}
			}
			if passed {
				score += 20 + (6-advRow)*5 // more bonus the further advanced
			}
		}

		if blackPawns[c] > 0 {
			passed := true
			advRow := 0
			for r := 7; r >= 0; r-- {
				if board[r][c] == 'p' && r > advRow {
					advRow = r
				}
			}
			for checkC := max(0, c-1); checkC <= min(7, c+1); checkC++ {
				for r := 7; r > advRow; r-- {
					if board[r][checkC] == 'P' {
						passed = false
						break
					}
				}
				if !passed {
					break
				}
			}
			if passed {
				score -= 20 + (advRow-1)*5
			}
		}
	}

	return score
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
	{-5, 0, 5, 5, 5, 5, 0, -5},
	{-10, 5, 5, 5, 5, 5, 5, -10},
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

// pstKingEnd encourages king centralization in the endgame.
var pstKingEnd = [8][8]int{
	{-50, -40, -30, -20, -20, -30, -40, -50},
	{-30, -20, -10, 0, 0, -10, -20, -30},
	{-30, -10, 20, 30, 30, 20, -10, -30},
	{-30, -10, 30, 40, 40, 30, -10, -30},
	{-30, -10, 30, 40, 40, 30, -10, -30},
	{-30, -10, 20, 30, 30, 20, -10, -30},
	{-30, -30, 0, 0, 0, 0, -30, -30},
	{-50, -30, -30, -30, -30, -30, -30, -50},
}
