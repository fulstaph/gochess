package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/fulstaph/gochess/chess"
	"github.com/fulstaph/gochess/store"
	"nhooyr.io/websocket"
)

// Hub manages all active rooms and connected players.
type Hub struct {
	mu          sync.Mutex
	rooms       map[string]*Room
	players     map[string]*Player // playerID → Player
	sessions    *sessionManager
	rater       *Rater
	matchmaker  *Matchmaker
	db          store.Store // may be nil

	gcTicker *time.Ticker
	done     chan struct{}
}

// NewHub creates and starts a Hub. db may be nil for an in-memory-only server.
func NewHub(db store.Store) *Hub {
	h := &Hub{
		rooms:      make(map[string]*Room),
		players:    make(map[string]*Player),
		sessions:   newSessionManager(db),
		rater:      newRater(),
		matchmaker: newMatchmaker(),
		db:         db,
		gcTicker:   time.NewTicker(2 * time.Minute),
		done:       make(chan struct{}),
	}
	if db != nil {
		go h.sessionCleanupLoop()
	} else {
		go h.memCleanupLoop()
	}
	go h.gcLoop()
	go h.raterCleanupLoop()
	return h
}

// Stop shuts down background goroutines.
func (h *Hub) Stop() {
	h.gcTicker.Stop()
	close(h.done)
}

// HandleWebSocket upgrades an HTTP connection and runs the player session.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}

	playerID, displayName, token, err := h.sessions.resolve(r)
	if err != nil {
		log.Printf("session resolve: %v", err)
		conn.Close(websocket.StatusInternalError, "internal error")
		return
	}

	h.mu.Lock()
	if old, ok := h.players[playerID]; ok {
		old.close()
	}
	p := newPlayer(playerID, displayName, conn, h)
	h.players[playerID] = p
	h.mu.Unlock()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	p.sendJSON(SessionMessage{
		Type:        "session",
		V:           ProtocolVersion,
		PlayerID:    playerID,
		DisplayName: displayName,
		Token:       token,
	})

	go p.writePump(ctx)
	p.readPump(ctx)
}

// dispatch routes a client message to the appropriate handler.
func (h *Hub) dispatch(p *Player, msg ClientMessage) {
	switch msg.Type {
	case "create_room":
		h.handleCreateRoom(p, msg)
	case "join_room":
		h.handleJoinRoom(p, msg)
	case "list_rooms":
		h.handleListRooms(p)
	case "move":
		h.handleMove(p, msg)
	case "new_game":
		h.handleNewGame(p, msg)
	case "resign":
		h.handleResign(p)
	case "draw_offer":
		h.handleDrawOffer(p)
	case "draw_response":
		h.handleDrawResponse(p, msg)
	case "register":
		h.handleRegister(p, msg)
	case "login":
		h.handleLogin(p, msg)
	case "find_game":
		h.handleFindGame(p, msg)
	case "undo":
		h.handleUndo(p)
	case "cancel_match":
		h.handleCancelMatch(p)
	default:
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "unknown message type: " + msg.Type})
	}
}

// disconnect is called when a player's read pump exits.
func (h *Hub) disconnect(p *Player) {
	h.matchmaker.Dequeue(p.ID)

	h.mu.Lock()
	delete(h.players, p.ID)
	roomID := p.roomID
	h.mu.Unlock()

	p.close()

	if roomID != "" {
		h.mu.Lock()
		room, ok := h.rooms[roomID]
		h.mu.Unlock()
		if ok {
			room.removePlayer(p)
		}
	}
}

// ListRooms returns a snapshot of all non-expired rooms.
func (h *Hub) ListRooms() []RoomInfo {
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()

	infos := make([]RoomInfo, 0, len(rooms))
	for _, r := range rooms {
		if !r.IsExpired() {
			infos = append(infos, r.Info(h.rater))
		}
	}
	return infos
}

// ---- message handlers ----

func (h *Hub) handleCreateRoom(p *Player, msg ClientMessage) {
	p.mu.Lock()
	if p.roomID != "" {
		p.mu.Unlock()
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "already in a room"})
		return
	}
	p.mu.Unlock()

	vsAI := msg.AIMode != "none" && msg.AIMode != ""
	aiDepth := msg.AIDepth
	if aiDepth < 1 || aiDepth > 4 {
		aiDepth = 2
	}
	color := msg.Color
	if color == "" {
		color = "white"
	}

	id, err := newID()
	if err != nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "internal error"})
		log.Printf("create room ID: %v", err)
		return
	}
	roomID := id[:8]
	room := newRoom(roomID, h, colorToRole(color), vsAI, aiDepth)

	if tc := ParseTimeControl(msg.TimeControl); tc != nil {
		rated := !vsAI // rated only for human vs human
		room.setTimeControl(tc, msg.TimeControl, rated)
	}

	h.mu.Lock()
	h.rooms[roomID] = room
	h.mu.Unlock()

	role, started := room.addPlayer(p, color)

	p.sendJSON(RoomCreatedMessage{
		Type:        "room_created",
		V:           ProtocolVersion,
		RoomID:      roomID,
		PlayerColor: role.String(),
	})

	if started {
		p.sendJSON(room.buildStateFor(role))
		if room.shouldAIMove() {
			go room.runAIMove(context.Background())
		}
	}
}

func (h *Hub) handleJoinRoom(p *Player, msg ClientMessage) {
	if msg.RoomID == "" {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "roomId required"})
		return
	}

	h.mu.Lock()
	room, ok := h.rooms[msg.RoomID]
	h.mu.Unlock()

	if !ok {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "room not found"})
		return
	}

	role, started := room.addPlayer(p, "")

	p.sendJSON(RoomJoinedMessage{
		Type:         "room_joined",
		V:            ProtocolVersion,
		RoomID:       room.id,
		PlayerColor:  role.String(),
		OpponentName: room.opponentNameFor(role),
	})

	if role == RoleSpectator {
		room.mu.Lock()
		p.sendJSON(room.buildStateFor(RoleSpectator))
		room.mu.Unlock()
		return
	}

	if started {
		room.mu.Lock()
		room.broadcastState()
		room.mu.Unlock()
	}
}

func (h *Hub) handleListRooms(p *Player) {
	p.sendJSON(RoomListMessage{
		Type:  "room_list",
		V:     ProtocolVersion,
		Rooms: h.ListRooms(),
	})
}

func (h *Hub) handleMove(p *Player, msg ClientMessage) {
	room := h.roomOf(p)
	if room == nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "not in a room"})
		return
	}
	room.ApplyMove(p, msg.From, msg.To, msg.Promotion)
}

func (h *Hub) handleNewGame(p *Player, msg ClientMessage) {
	room := h.roomOf(p)
	if room == nil {
		aiMode := msg.AIMode
		if aiMode == "" {
			aiMode = "black"
		}
		fakeMsg := ClientMessage{AIMode: aiMode, AIDepth: msg.AIDepth, Color: "white"}
		h.handleCreateRoom(p, fakeMsg)
		return
	}
	room.NewGame(msg.AIMode, msg.AIDepth)
}

func (h *Hub) handleUndo(p *Player) {
	room := h.roomOf(p)
	if room == nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "not in a room"})
		return
	}
	room.Undo(p)
}

func (h *Hub) handleResign(p *Player) {
	room := h.roomOf(p)
	if room == nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "not in a room"})
		return
	}
	room.Resign(p)
}

func (h *Hub) handleDrawOffer(p *Player) {
	room := h.roomOf(p)
	if room == nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "not in a room"})
		return
	}
	room.OfferDraw(p)
}

func (h *Hub) handleDrawResponse(p *Player, msg ClientMessage) {
	room := h.roomOf(p)
	if room == nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "not in a room"})
		return
	}
	room.RespondDraw(p, msg.Accept)
}

func (h *Hub) handleRegister(p *Player, msg ClientMessage) {
	ctx := context.Background()
	newTok, displayName, err := h.sessions.Register(ctx, p.ID, msg.Username, msg.Password)
	if err != nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: err.Error()})
		return
	}
	p.DisplayName = displayName
	p.sendJSON(AuthOKMessage{
		Type:        "auth_ok",
		V:           ProtocolVersion,
		Token:       newTok,
		PlayerID:    p.ID,
		DisplayName: displayName,
	})
}

func (h *Hub) handleLogin(p *Player, msg ClientMessage) {
	ctx := context.Background()
	tok, playerID, displayName, err := h.sessions.Login(ctx, msg.Username, msg.Password)
	if err != nil {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: err.Error()})
		return
	}
	// Migrate the connection to the registered player identity.
	h.mu.Lock()
	delete(h.players, p.ID)
	p.ID = playerID
	p.DisplayName = displayName
	if old, ok := h.players[playerID]; ok && old != p {
		old.close()
	}
	h.players[playerID] = p
	h.mu.Unlock()

	p.sendJSON(AuthOKMessage{
		Type:        "auth_ok",
		V:           ProtocolVersion,
		Token:       tok,
		PlayerID:    playerID,
		DisplayName: displayName,
	})
}

// updateRatings is called in a goroutine after a rated game completes.
func (h *Hub) updateRatings(r *Room) {
	r.mu.Lock()
	result := r.result
	whiteID := ""
	blackID := ""
	whiteName := "?"
	blackName := "?"
	if r.white != nil {
		whiteID = r.white.ID
		whiteName = r.white.DisplayName
	}
	if r.black != nil {
		blackID = r.black.ID
		blackName = r.black.DisplayName
	}
	moves := make([]chess.Move, len(r.moves))
	copy(moves, r.moves)
	tcName := r.tcName
	startedAt := r.startedAt
	finishedAt := r.finishedAt
	r.mu.Unlock()

	if whiteID == "" || blackID == "" {
		return
	}

	isDraw := false
	winnerID := whiteID
	loserID := blackID

	switch {
	case len(result) > 4 && result[:4] == "Draw":
		isDraw = true
	case len(result) > 5 && result[:5] == "Black":
		winnerID, loserID = blackID, whiteID
	}

	oldWhite := h.rater.Rating(whiteID)
	oldBlack := h.rater.Rating(blackID)
	h.rater.UpdateGame(winnerID, loserID, isDraw)
	newWhite := h.rater.Rating(whiteID)
	newBlack := h.rater.Rating(blackID)

	// Persist ratings and game record.
	if h.db != nil {
		ctx := context.Background()
		if err := h.db.UpdateRating(ctx, whiteID, newWhite); err != nil {
			log.Printf("UpdateRating white: %v", err)
		}
		if err := h.db.UpdateRating(ctx, blackID, newBlack); err != nil {
			log.Printf("UpdateRating black: %v", err)
		}

		pgnResult := resultToPGN(result)
		headers := map[string]string{
			"White":  whiteName,
			"Black":  blackName,
			"Result": pgnResult,
		}
		pgn := chess.FormatPGN(moves, chess.InitialState(), headers, pgnResult)
		gameID, err := newID()
		if err != nil {
			log.Printf("generate game ID: %v", err)
			return
		}
		g := &store.Game{
			ID:          gameID,
			WhiteID:     whiteID,
			BlackID:     blackID,
			PGN:         pgn,
			Result:      result,
			TimeControl: tcName,
			Rated:       true,
			StartedAt:   startedAt,
			FinishedAt:  finishedAt,
		}
		if err := h.db.SaveGame(ctx, g); err != nil {
			log.Printf("SaveGame: %v", err)
		}
	}

	r.mu.Lock()
	if r.white != nil {
		r.white.sendJSON(RatingUpdateMessage{
			Type: "rating_update", V: ProtocolVersion,
			OldRating: oldWhite, NewRating: newWhite, Delta: newWhite - oldWhite,
		})
	}
	if r.black != nil {
		r.black.sendJSON(RatingUpdateMessage{
			Type: "rating_update", V: ProtocolVersion,
			OldRating: oldBlack, NewRating: newBlack, Delta: newBlack - oldBlack,
		})
	}
	r.mu.Unlock()
}

// resultToPGN converts a game result string to PGN notation.
func resultToPGN(result string) string {
	switch {
	case len(result) > 4 && result[:4] == "Draw":
		return "1/2-1/2"
	case len(result) > 5 && result[:5] == "Black":
		return "0-1"
	case len(result) > 5 && result[:5] == "White":
		return "1-0"
	}
	return "*"
}

// roomOf returns the room the player is currently in, or nil.
func (h *Hub) roomOf(p *Player) *Room {
	p.mu.Lock()
	roomID := p.roomID
	p.mu.Unlock()
	if roomID == "" {
		return nil
	}
	h.mu.Lock()
	r := h.rooms[roomID]
	h.mu.Unlock()
	return r
}

// gcLoop periodically removes expired rooms.
func (h *Hub) gcLoop() {
	for {
		select {
		case <-h.gcTicker.C:
			h.mu.Lock()
			ids := make([]string, 0, len(h.rooms))
			for id := range h.rooms {
				ids = append(ids, id)
			}
			h.mu.Unlock()

			var expired []string
			for _, id := range ids {
				h.mu.Lock()
				r, ok := h.rooms[id]
				h.mu.Unlock()
				if ok && r.IsExpired() {
					expired = append(expired, id)
				}
			}

			if len(expired) > 0 {
				h.mu.Lock()
				for _, id := range expired {
					delete(h.rooms, id)
				}
				h.mu.Unlock()
			}
		case <-h.done:
			return
		}
	}
}

// sessionCleanupLoop removes expired DB sessions every hour.
func (h *Hub) sessionCleanupLoop() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := h.db.DeleteExpiredSessions(context.Background()); err != nil {
				log.Printf("session cleanup: %v", err)
			}
		case <-h.done:
			return
		}
	}
}

// memCleanupLoop removes stale in-memory sessions when no DB is configured.
func (h *Hub) memCleanupLoop() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			h.sessions.mem.cleanup(24 * time.Hour)
		case <-h.done:
			return
		}
	}
}

// raterCleanupLoop removes stale default-rating entries.
func (h *Hub) raterCleanupLoop() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			h.rater.cleanup(24 * time.Hour)
		case <-h.done:
			return
		}
	}
}

func colorToRole(color string) PlayerRole {
	if color == "black" {
		return RoleBlack
	}
	return RoleWhite
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
