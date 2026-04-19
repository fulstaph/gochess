package server

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"nhooyr.io/websocket"
)

// PlayerRole identifies a player's role inside a room.
type PlayerRole int

const (
	RoleWhite PlayerRole = iota
	RoleBlack
	RoleSpectator
)

func (r PlayerRole) String() string {
	switch r {
	case RoleWhite:
		return "white"
	case RoleBlack:
		return "black"
	default:
		return "spectator"
	}
}

// Player represents a connected WebSocket client.
type Player struct {
	ID          string
	DisplayName string
	remoteIP    string // set by Hub at accept time; used for per-IP rate limiting
	conn        *websocket.Conn
	send        chan []byte
	hub         *Hub
	roomID      string // current room, "" if in lobby
	mu          sync.Mutex
	closeOnce   sync.Once
}

func newPlayer(id, displayName string, conn *websocket.Conn, hub *Hub) *Player {
	return &Player{
		ID:          id,
		DisplayName: displayName,
		conn:        conn,
		send:        make(chan []byte, 64),
		hub:         hub,
	}
}

// writePump drains the send channel to the WebSocket connection.
func (p *Player) writePump(ctx context.Context) {
	for {
		select {
		case msg, ok := <-p.send:
			if !ok {
				return
			}
			if err := p.conn.Write(ctx, websocket.MessageText, msg); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// readPump reads messages from the WebSocket and dispatches them to the hub.
func (p *Player) readPump(ctx context.Context) {
	defer func() {
		p.hub.disconnect(p)
	}()
	for {
		_, data, err := p.conn.Read(ctx)
		if err != nil {
			return
		}
		// Per-player message rate limit — checked before JSON parsing so a
		// spamming client cannot burn CPU on unmarshalling.
		if !p.hub.msgLim.Allow(p.ID) {
			sendRateLimited(p, "message")
			continue
		}
		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "invalid message format"})
			continue
		}
		p.hub.dispatch(p, msg)
	}
}

// sendJSON marshals v and enqueues it on the send channel.
func (p *Player) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("player %s sendJSON marshal: %v", p.ID, err)
		return
	}
	select {
	case p.send <- data:
	default:
		log.Printf("player %s send buffer full, dropping message", p.ID)
	}
}

// close closes the send channel. Safe to call more than once.
func (p *Player) close() {
	p.closeOnce.Do(func() { close(p.send) })
}
