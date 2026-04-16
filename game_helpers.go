package main

import (
	"fmt"

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
