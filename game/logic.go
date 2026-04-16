// Package game provides shared game logic used by both the TUI and web server.
package game

import (
	"fmt"
	"strings"

	"github.com/fulstaph/gochess/chess"
)

// RecordMove appends a move to the history slice in "1. e4 e5" format.
// White's moves start a new entry; black's moves append to the last entry.
func RecordMove(history *[]string, state chess.GameState, move chess.Move) {
	notation := chess.FormatMove(move)
	if state.Turn() == chess.White || len(*history) == 0 {
		*history = append(*history, fmt.Sprintf("%d. %s", state.FullmoveNumber(), notation))
		return
	}
	last := len(*history) - 1
	(*history)[last] = (*history)[last] + " " + notation
}

// CheckGameOver returns (true, resultString) when the game is over.
// repetitionCount is the number of times the current position has been seen.
func CheckGameOver(state chess.GameState, repetitionCount int) (bool, string) {
	return checkGameOverWithMoves(state, repetitionCount, chess.GenerateLegalMoves(state))
}

// CheckGameOverMoves is like CheckGameOver but accepts pre-generated legal
// moves to avoid a redundant generation when the caller already has them.
func CheckGameOverMoves(state chess.GameState, repetitionCount int, legal []chess.Move) (bool, string) {
	return checkGameOverWithMoves(state, repetitionCount, legal)
}

func checkGameOverWithMoves(state chess.GameState, repetitionCount int, legal []chess.Move) (bool, string) {
	if len(legal) == 0 {
		if chess.IsInCheck(state, state.Turn()) {
			return true, fmt.Sprintf("Checkmate. %s wins.", chess.ColorName(-state.Turn()))
		}
		return true, "Stalemate. Draw."
	}
	if chess.IsSeventyFiveMoveDraw(state) {
		return true, "Draw by seventy-five-move rule."
	}
	if repetitionCount >= 5 {
		return true, "Draw by fivefold repetition."
	}
	return false, ""
}

// ClaimableDraw returns a notice string if a draw can be claimed under the
// 50-move rule or threefold repetition, or an empty string otherwise.
func ClaimableDraw(state chess.GameState, repetitionCount int) string {
	reasons := make([]string, 0, 2)
	if chess.IsFiftyMoveDraw(state) {
		reasons = append(reasons, "50-move rule")
	}
	if repetitionCount >= 3 {
		reasons = append(reasons, "threefold repetition")
	}
	if len(reasons) == 0 {
		return ""
	}
	return "Draw claim available: " + strings.Join(reasons, " or ")
}
