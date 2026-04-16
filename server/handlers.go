package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/fulstaph/gochess/chess"
	"nhooyr.io/websocket"
)

// HandleWebSocket is kept for the legacy single-session web server (cmd/web/main.go).
// New code should use Hub.HandleWebSocket instead.
func (s *GameSession) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := r.Context()

	s.mu.Lock()
	msg := s.BuildStateMessage()
	s.mu.Unlock()

	if err := writeJSON(ctx, conn, msg); err != nil {
		return
	}

	s.mu.Lock()
	aiShouldMove := s.ShouldAIMove()
	s.mu.Unlock()

	if aiShouldMove {
		s.handleAITurn(ctx, conn)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(data, &clientMsg); err != nil {
			writeJSON(ctx, conn, ErrorMessage{Type: "error", V: ProtocolVersion, Message: "invalid message format"})
			continue
		}

		switch clientMsg.Type {
		case "move":
			s.handleMove(ctx, conn, clientMsg)
		case "new_game":
			s.handleNewGame(ctx, conn, clientMsg)
		case "resign":
			s.handleResign(ctx, conn)
		default:
			writeJSON(ctx, conn, ErrorMessage{Type: "error", V: ProtocolVersion, Message: "unknown message type"})
		}
	}
}

func (s *GameSession) handleMove(ctx context.Context, conn *websocket.Conn, msg ClientMessage) {
	s.mu.Lock()
	err := s.ApplyMove(msg.From, msg.To, msg.Promotion)
	if err != nil {
		s.mu.Unlock()
		writeJSON(ctx, conn, ErrorMessage{Type: "error", V: ProtocolVersion, Message: err.Error()})
		return
	}
	state := s.BuildStateMessage()
	aiShouldMove := s.ShouldAIMove()
	s.mu.Unlock()

	writeJSON(ctx, conn, state)

	if aiShouldMove {
		s.handleAITurn(ctx, conn)
	}
}

func (s *GameSession) handleNewGame(ctx context.Context, conn *websocket.Conn, msg ClientMessage) {
	aiMode := msg.AIMode
	if aiMode == "" {
		aiMode = "black"
	}
	aiDepth := msg.AIDepth
	if aiDepth < 1 || aiDepth > 4 {
		aiDepth = 2
	}

	s.mu.Lock()
	s.Reset(aiMode, aiDepth)
	state := s.BuildStateMessage()
	aiShouldMove := s.ShouldAIMove()
	s.mu.Unlock()

	writeJSON(ctx, conn, state)

	if aiShouldMove {
		s.handleAITurn(ctx, conn)
	}
}

func (s *GameSession) handleResign(ctx context.Context, conn *websocket.Conn) {
	s.mu.Lock()
	s.Resign()
	state := s.BuildStateMessage()
	s.mu.Unlock()

	writeJSON(ctx, conn, state)
}

func (s *GameSession) handleAITurn(ctx context.Context, conn *websocket.Conn) {
	writeJSON(ctx, conn, ThinkingMessage{Type: "thinking", V: ProtocolVersion})

	// Copy state before releasing the lock so AI computation is lock-free.
	s.mu.Lock()
	stateCopy := s.state
	depth := s.aiDepth
	s.mu.Unlock()

	mv, ok := chess.BestMove(stateCopy, depth)
	if !ok {
		return
	}

	time.Sleep(100 * time.Millisecond) // keep "thinking" visible briefly

	s.mu.Lock()
	s.applyAndRecord(mv)
	state := s.BuildStateMessage()
	aiShouldMove := s.ShouldAIMove()
	s.mu.Unlock()

	writeJSON(ctx, conn, state)

	if aiShouldMove {
		s.handleAITurn(ctx, conn)
	}
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
