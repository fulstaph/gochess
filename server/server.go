package server

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fulstaph/gochess/chess"
	"github.com/fulstaph/gochess/game"
)

type GameSession struct {
	mu          sync.Mutex
	state       chess.GameState
	aiMode      string // "white", "black", "both", "none"
	aiDepth     int
	repetitions map[string]int
	moveHistory []string
	gameOver    bool
	result      string
	lastMove    *chess.Move
}

func NewSession(aiMode string, aiDepth int) *GameSession {
	s := &GameSession{
		aiMode:      aiMode,
		aiDepth:     aiDepth,
		repetitions: make(map[string]int),
	}
	s.state = chess.InitialState()
	return s
}

func (s *GameSession) Reset(aiMode string, aiDepth int) {
	s.state = chess.InitialState()
	s.aiMode = aiMode
	s.aiDepth = aiDepth
	s.repetitions = make(map[string]int)
	s.moveHistory = nil
	s.gameOver = false
	s.result = ""
	s.lastMove = nil
}

func (s *GameSession) ApplyMove(from, to, promotion string) error {
	if s.gameOver {
		return fmt.Errorf("game is over")
	}

	input := from + to + promotion
	mv, err := chess.ParseMove(input, s.state)
	if err != nil {
		return err
	}

	s.applyAndRecord(mv)
	return nil
}

func (s *GameSession) ApplyAIMove() (bool, error) {
	if s.gameOver {
		return false, fmt.Errorf("game is over")
	}

	mv, ok := chess.BestMove(s.state, s.aiDepth)
	if !ok {
		return false, nil
	}

	s.applyAndRecord(mv)
	return true, nil
}

func (s *GameSession) applyAndRecord(mv chess.Move) {
	game.RecordMove(&s.moveHistory, s.state, mv)
	s.state = chess.ApplyMove(s.state, mv)
	s.lastMove = &mv

	key := chess.PositionKey(s.state)
	s.repetitions[key]++

	over, result := game.CheckGameOver(s.state, s.repetitions[key])
	if over {
		s.gameOver = true
		s.result = result
	}
}

func (s *GameSession) ShouldAIMove() bool {
	if s.gameOver {
		return false
	}
	turn := s.state.Turn()
	switch s.aiMode {
	case "white":
		return turn == chess.White
	case "black":
		return turn == chess.Black
	case "both":
		return true
	default:
		return false
	}
}

func (s *GameSession) Resign() {
	if s.gameOver {
		return
	}
	s.gameOver = true
	winner := "White"
	if s.state.Turn() == chess.White {
		winner = "Black"
	}
	s.result = fmt.Sprintf("%s resigns. %s wins.", chess.ColorName(s.state.Turn()), winner)
}

// buildBoard converts the engine board to the frontend [8][8]string format.
func buildBoard(state chess.GameState) [8][8]string {
	var board [8][8]string
	b := state.Board()
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			if b[r][c] != 0 {
				board[r][c] = string(b[r][c])
			}
		}
	}
	return board
}

func (s *GameSession) BuildStateMessage() StateMessage {
	board := buildBoard(s.state)

	turn := "white"
	if s.state.Turn() == chess.Black {
		turn = "black"
	}

	var legalMoves []LegalMove
	if !s.gameOver {
		legalMoves = buildLegalMoves(s.state)
	}

	var lastMove *LegalMove
	if s.lastMove != nil {
		lastMove = formatMoveToLegal(*s.lastMove, s.state)
	}

	return StateMessage{
		Type:        "state",
		V:           ProtocolVersion,
		Board:       board,
		Turn:        turn,
		MoveNumber:  s.state.FullmoveNumber(),
		IsCheck:     chess.IsInCheck(s.state, s.state.Turn()),
		IsGameOver:  s.gameOver,
		Result:      s.result,
		LegalMoves:  legalMoves,
		LastMove:    lastMove,
		MoveHistory: s.moveHistory,
	}
}

// buildLegalMoves converts engine moves to the frontend format.
func buildLegalMoves(state chess.GameState) []LegalMove {
	return formatLegalMoves(chess.GenerateLegalMoves(state), state)
}

func formatLegalMoves(moves []chess.Move, state chess.GameState) []LegalMove {
	result := make([]LegalMove, 0, len(moves))
	for _, mv := range moves {
		formatted := chess.FormatMove(mv)
		lm := parseMoveString(formatted, state)
		result = append(result, lm)
	}
	return result
}

// castleRank returns "1" for white and "8" for black.
func castleRank(isBlack bool) string {
	if isBlack {
		return "8"
	}
	return "1"
}

// parseMoveString converts a FormatMove string like "e2e4", "e7e8q", "O-O" into a LegalMove.
func parseMoveString(s string, state chess.GameState) LegalMove {
	rank := castleRank(state.Turn() == chess.Black)
	if s == "O-O" {
		return LegalMove{From: "e" + rank, To: "g" + rank}
	}
	if s == "O-O-O" {
		return LegalMove{From: "e" + rank, To: "c" + rank}
	}

	from := s[0:2]
	to := s[2:4]
	promo := ""
	if len(s) > 4 {
		promo = strings.ToLower(s[4:5])
	}
	return LegalMove{From: from, To: to, Promotion: promo}
}

// formatMoveToLegal converts a Move to LegalMove for the lastMove field.
func formatMoveToLegal(mv chess.Move, currentState chess.GameState) *LegalMove {
	formatted := chess.FormatMove(mv)
	// The move was made by the side opposite to the current turn.
	rank := castleRank(currentState.Turn() == chess.White)
	var lm LegalMove
	switch formatted {
	case "O-O":
		lm = LegalMove{From: "e" + rank, To: "g" + rank}
	case "O-O-O":
		lm = LegalMove{From: "e" + rank, To: "c" + rank}
	default:
		lm = LegalMove{From: formatted[0:2], To: formatted[2:4]}
	}
	return &lm
}
