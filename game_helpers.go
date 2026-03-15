package main

import (
	"fmt"
	"strings"

	"github.com/fulstaph/gochess/chess"
)

func aiControls(mode aiSide, color int) bool {
	switch mode {
	case aiWhite:
		return color == chess.White
	case aiBlack:
		return color == chess.Black
	case aiBoth:
		return true
	default:
		return false
	}
}

func recordMove(history *[]string, state chess.GameState, move chess.Move) {
	notation := chess.FormatMove(move)
	if state.Turn() == chess.White {
		entry := fmt.Sprintf("%d. %s", state.FullmoveNumber(), notation)
		*history = append(*history, entry)
		return
	}
	if len(*history) == 0 {
		entry := fmt.Sprintf("%d. %s", state.FullmoveNumber(), notation)
		*history = append(*history, entry)
		return
	}
	last := len(*history) - 1
	(*history)[last] = (*history)[last] + " " + notation
}

func updateRepetition(repetitions map[string]int, state chess.GameState) int {
	key := chess.PositionKey(state)
	repetitions[key]++
	return repetitions[key]
}

func buildStatus(state chess.GameState, hasLast bool, lastMove chess.Move, lastMoveSource string) string {
	status := ""
	if hasLast {
		if lastMoveSource != "" {
			status = fmt.Sprintf("Last move (%s): %s", lastMoveSource, chess.FormatMove(lastMove))
		} else {
			status = fmt.Sprintf("Last move: %s", chess.FormatMove(lastMove))
		}
	}
	if chess.IsInCheck(state, state.Turn()) {
		if status != "" {
			status += " | "
		}
		status += "Check!"
	}
	return status
}

func checkGameOver(state chess.GameState, repetitionCount int) (bool, string) {
	legal := chess.GenerateLegalMoves(state)
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

func claimableDrawNotice(state chess.GameState, repetitionCount int) string {
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
