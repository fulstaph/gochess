package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fulstaph/gochess/chess"
	"github.com/fulstaph/gochess/game"
)

// RoomStatus represents the lifecycle state of a room.
type RoomStatus int

const (
	RoomWaiting  RoomStatus = iota // waiting for second player
	RoomPlaying                    // both players present, game in progress
	RoomFinished                   // game over
)

func (s RoomStatus) String() string {
	switch s {
	case RoomWaiting:
		return "waiting"
	case RoomPlaying:
		return "playing"
	default:
		return "finished"
	}
}

const roomGCDelay = 5 * time.Minute
const disconnectGrace = 60 * time.Second

// Room manages one chess game between two players (or one player and the AI).
type Room struct {
	id  string
	mu  sync.Mutex
	hub *Hub

	status  RoomStatus
	white   *Player
	black   *Player
	vsAI    bool
	aiDepth int

	spectators []*Player

	state       chess.GameState
	repetitions map[string]int
	moveHistory []string
	moves       []chess.Move // ordered list for PGN generation
	gameOver    bool
	result      string
	lastMove    *chess.Move
	legalMoves  []chess.Move // legal moves for current position (nil = not yet computed)

	startedAt time.Time

	// Time control
	clock  *Clock // nil for untimed games
	tcName string // preset name, "" for untimed
	rated  bool

	// Draw offer: which role has an outstanding offer (nil = none)
	drawOffer *PlayerRole

	// Disconnection grace-period timers
	whiteTimer *time.Timer
	blackTimer *time.Timer
	finishedAt time.Time

	// Move count before first clock start (clock starts after first move)
	moveCount int

	// Undo stack (vs-AI games only)
	undoStack []roomSnapshot
}

type roomSnapshot struct {
	state       chess.GameState
	moveHistory []string
	moves       []chess.Move
	repetitions map[string]int
	lastMove    *chess.Move
	legalMoves  []chess.Move
	moveCount   int
}

func newRoom(id string, hub *Hub, creatorColor PlayerRole, vsAI bool, aiDepth int) *Room {
	state := chess.InitialState()
	return &Room{
		id:          id,
		hub:         hub,
		vsAI:        vsAI,
		aiDepth:     aiDepth,
		state:       state,
		repetitions: make(map[string]int),
		legalMoves:  chess.GenerateLegalMoves(state),
	}
}

func (r *Room) setTimeControl(tc *TimeControl, name string, rated bool) {
	r.tcName = name
	r.rated = rated && tc != nil
	if tc != nil {
		r.clock = NewClock(*tc, func(loser int) {
			r.flagLoss(loser)
		})
	}
}

// addPlayer seats a player in the room. Returns their colour and whether the
// game has started (both seats filled or vs AI).
func (r *Room) addPlayer(p *Player, desiredColor string) (PlayerRole, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var role PlayerRole
	switch desiredColor {
	case "black":
		if r.black == nil {
			r.black = p
			role = RoleBlack
		} else if r.white == nil {
			r.white = p
			role = RoleWhite
		} else {
			r.spectators = append(r.spectators, p)
			return RoleSpectator, false
		}
	default: // "white", "random", or ""
		if r.white == nil {
			r.white = p
			role = RoleWhite
		} else if r.black == nil {
			r.black = p
			role = RoleBlack
		} else {
			r.spectators = append(r.spectators, p)
			return RoleSpectator, false
		}
	}

	p.mu.Lock()
	p.roomID = r.id
	p.mu.Unlock()

	bothSeated := (r.white != nil || r.vsAI) && (r.black != nil || r.vsAI)
	if bothSeated && r.status == RoomWaiting {
		r.status = RoomPlaying
		r.startedAt = time.Now()
	}
	return role, bothSeated
}

// removePlayer removes a player and starts the grace-period timer.
func (r *Room) removePlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check spectators first.
	for i, sp := range r.spectators {
		if sp == p {
			r.spectators = append(r.spectators[:i], r.spectators[i+1:]...)
			p.mu.Lock()
			p.roomID = ""
			p.mu.Unlock()
			return
		}
	}

	var isWhite bool
	if r.white == p {
		r.white = nil
		isWhite = true
	} else if r.black == p {
		r.black = nil
	} else {
		return
	}

	p.mu.Lock()
	p.roomID = ""
	p.mu.Unlock()

	if r.gameOver || r.status == RoomFinished {
		return
	}

	r.broadcastExcept(p, OpponentDisconnectedMessage{Type: "opponent_disconnected", V: ProtocolVersion})

	timer := time.AfterFunc(disconnectGrace, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if (isWhite && r.white == nil) || (!isWhite && r.black == nil) {
			if !r.gameOver {
				winner := "Black"
				if !isWhite {
					winner = "White"
				}
				r.finishGame(fmt.Sprintf("Opponent disconnected. %s wins.", winner))
			}
		}
	})

	if isWhite {
		if r.whiteTimer != nil {
			r.whiteTimer.Stop()
		}
		r.whiteTimer = timer
	} else {
		if r.blackTimer != nil {
			r.blackTimer.Stop()
		}
		r.blackTimer = timer
	}
}

// ApplyMove validates and applies a player move.
func (r *Room) ApplyMove(p *Player, from, to, promotion string) {
	r.mu.Lock()

	role := r.roleOf(p)
	if role == RoleSpectator {
		r.mu.Unlock()
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "spectators cannot move"})
		return
	}
	if r.gameOver {
		r.mu.Unlock()
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "game is over"})
		return
	}
	if r.status != RoomPlaying {
		r.mu.Unlock()
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "game has not started yet"})
		return
	}
	turn := r.state.Turn()
	if (turn == chess.White && role != RoleWhite) || (turn == chess.Black && role != RoleBlack) {
		r.mu.Unlock()
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "not your turn"})
		return
	}

	mv, err := chess.ParseMove(from+to+promotion, r.state)
	if err != nil {
		r.mu.Unlock()
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: err.Error()})
		return
	}

	// Clear any pending draw offer on move.
	r.drawOffer = nil

	r.applyAndRecord(mv)
	r.mu.Unlock()

	r.broadcastAll()

	if !r.gameOver && r.vsAI && r.shouldAIMove() {
		go r.runAIMove(context.Background())
	}
}

// Resign ends the game with the given player forfeiting.
func (r *Room) Resign(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.gameOver {
		return
	}
	role := r.roleOf(p)
	resigner := "White"
	winner := "Black"
	if role == RoleBlack {
		resigner = "Black"
		winner = "White"
	}
	r.finishGame(fmt.Sprintf("%s resigns. %s wins.", resigner, winner))
}

// Undo reverts the last human + AI move pair in vs-AI games.
func (r *Room) Undo(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.vsAI {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "undo only available in AI games"})
		return
	}
	role := r.roleOf(p)
	if role == RoleSpectator {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "spectators cannot undo"})
		return
	}

	// Pop twice: AI move + human move.
	pops := 2
	if len(r.undoStack) < pops {
		pops = len(r.undoStack)
	}
	if pops == 0 {
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "nothing to undo"})
		return
	}

	snap := r.undoStack[len(r.undoStack)-pops]
	r.undoStack = r.undoStack[:len(r.undoStack)-pops]
	r.state = snap.state
	r.moveHistory = snap.moveHistory
	r.moves = snap.moves
	r.repetitions = snap.repetitions
	r.lastMove = snap.lastMove
	r.legalMoves = snap.legalMoves
	r.moveCount = snap.moveCount
	r.gameOver = false
	r.result = ""
	r.drawOffer = nil

	r.broadcastState()
}

// OfferDraw records a draw offer from p. The opponent is notified.
func (r *Room) OfferDraw(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.gameOver || r.status != RoomPlaying {
		return
	}
	role := r.roleOf(p)
	if role == RoleSpectator {
		return
	}
	if r.drawOffer != nil {
		// Offer already pending.
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "draw already offered"})
		return
	}
	r.drawOffer = &role
	r.broadcastExcept(p, DrawOfferedMessage{Type: "draw_offered", V: ProtocolVersion})
}

// RespondDraw handles the opponent's accept/decline of a draw offer.
func (r *Room) RespondDraw(p *Player, accept bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.drawOffer == nil || r.gameOver {
		return
	}
	role := r.roleOf(p)
	// Only the non-offering side can respond.
	if role == *r.drawOffer || role == RoleSpectator {
		return
	}
	r.drawOffer = nil
	if accept {
		r.finishGame("Draw by agreement.")
		return
	}
	// Declined: notify the offerer.
	r.broadcastExcept(p, ErrorMessage{Type: "error", V: ProtocolVersion, Message: "Draw offer declined."})
}

// flagLoss is called by the clock when a player runs out of time.
func (r *Room) flagLoss(loser int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gameOver {
		return
	}
	r.finishGame(fmt.Sprintf("%s ran out of time. %s wins.",
		chess.ColorName(loser), chess.ColorName(-loser)))
}

// NewGame resets the room to a fresh game, swapping colours.
func (r *Room) NewGame(aiMode string, aiDepth int) {
	r.mu.Lock()
	if r.clock != nil {
		r.clock.Stop()
	}
	r.state = chess.InitialState()
	r.repetitions = make(map[string]int)
	r.legalMoves = chess.GenerateLegalMoves(r.state)
	r.moveHistory = nil
	r.moves = nil
	r.gameOver = false
	r.result = ""
	r.lastMove = nil
	r.drawOffer = nil
	r.moveCount = 0
	r.startedAt = time.Now()
	r.vsAI = aiMode != "none"
	r.aiDepth = aiDepth
	r.status = RoomPlaying
	r.clock = nil
	r.mu.Unlock()

	r.broadcastAll()

	if r.shouldAIMove() {
		go r.runAIMove(context.Background())
	}
}

// Info returns a snapshot of the room for the lobby list.
func (r *Room) Info(rater *Rater) RoomInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	whiteName := "AI"
	blackName := "AI"
	var whiteRating, blackRating int
	if r.white != nil {
		whiteName = r.white.DisplayName
		if rater != nil {
			whiteRating = rater.Rating(r.white.ID)
		}
	}
	if r.black != nil {
		blackName = r.black.DisplayName
		if rater != nil {
			blackRating = rater.Rating(r.black.ID)
		}
	}
	return RoomInfo{
		RoomID:      r.id,
		Status:      r.status.String(),
		WhiteName:   whiteName,
		BlackName:   blackName,
		WhiteRating: whiteRating,
		BlackRating: blackRating,
		TimeControl: r.tcName,
		Spectators:  len(r.spectators),
	}
}

// opponentNameFor returns the display name of the opponent for the given role.
func (r *Room) opponentNameFor(role PlayerRole) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if role == RoleWhite {
		if r.black != nil {
			return r.black.DisplayName
		}
		return "AI"
	}
	if r.white != nil {
		return r.white.DisplayName
	}
	return "AI"
}

// IsExpired reports whether the room can be garbage-collected.
func (r *Room) IsExpired() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status != RoomFinished {
		return false
	}
	return time.Since(r.finishedAt) > roomGCDelay
}

// ---- internal helpers ----

func (r *Room) roleOf(p *Player) PlayerRole {
	if p == r.white {
		return RoleWhite
	}
	if p == r.black {
		return RoleBlack
	}
	return RoleSpectator
}

func (r *Room) shouldAIMove() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.vsAI && !r.gameOver
}

func (r *Room) runAIMove(ctx context.Context) {
	r.mu.Lock()
	stateCopy := r.state
	depth := r.aiDepth
	r.mu.Unlock()

	if data, err := marshalJSON(ThinkingMessage{Type: "thinking", V: ProtocolVersion}); err == nil {
		r.broadcastRaw(data)
	}

	mv, ok := chess.BestMove(stateCopy, depth)
	if !ok {
		return
	}

	r.mu.Lock()
	r.applyAndRecord(mv)
	r.mu.Unlock()

	r.broadcastAll()

	if r.shouldAIMove() {
		go r.runAIMove(ctx)
	}
}

func (r *Room) applyAndRecord(mv chess.Move) {
	// Push undo snapshot for vs-AI games.
	if r.vsAI {
		repsCopy := make(map[string]int, len(r.repetitions))
		for k, v := range r.repetitions {
			repsCopy[k] = v
		}
		r.undoStack = append(r.undoStack, roomSnapshot{
			state:       r.state,
			moveHistory: append([]string{}, r.moveHistory...),
			moves:       append([]chess.Move{}, r.moves...),
			repetitions: repsCopy,
			lastMove:    r.lastMove,
			legalMoves:  r.legalMoves,
			moveCount:   r.moveCount,
		})
	}

	movingTurn := r.state.Turn()

	game.RecordMove(&r.moveHistory, r.state, mv)
	r.moves = append(r.moves, mv)
	r.state = chess.ApplyMove(r.state, mv)
	r.lastMove = &mv
	r.moveCount++

	key := chess.PositionKey(r.state)
	r.repetitions[key]++

	// Start clock after first move (white's first move starts black's clock).
	if r.clock != nil {
		if r.moveCount == 1 {
			r.clock.Start(r.state.Turn())
		} else {
			r.clock.Punch(movingTurn)
		}
	}

	// Generate legal moves once; reused by buildStateFor to avoid duplicate work.
	r.legalMoves = chess.GenerateLegalMoves(r.state)
	repCount := r.repetitions[key]
	if over, result := game.CheckGameOverMoves(r.state, repCount, r.legalMoves); over {
		r.finishGame(result)
	}
}

// finishGame marks the game as over and stops the clock.
// Must be called with r.mu held.
func (r *Room) finishGame(result string) {
	r.gameOver = true
	r.result = result
	r.status = RoomFinished
	r.finishedAt = time.Now()
	if r.clock != nil {
		r.clock.Stop()
	}
	// Trigger rating update via hub (non-blocking, hub checks for rated flag).
	if r.hub != nil && r.rated && !r.vsAI {
		go r.hub.updateRatings(r)
	}
	r.broadcastState()
}

// broadcastAll sends the current state to white, black, and spectators.
func (r *Room) broadcastAll() {
	r.mu.Lock()
	white := r.white
	black := r.black
	spectators := make([]*Player, len(r.spectators))
	copy(spectators, r.spectators)
	whiteMsg := r.buildStateFor(RoleWhite)
	blackMsg := r.buildStateFor(RoleBlack)
	specMsg := r.buildStateFor(RoleSpectator)
	r.mu.Unlock()

	if white != nil {
		white.sendJSON(whiteMsg)
	}
	if black != nil {
		black.sendJSON(blackMsg)
	}
	for _, sp := range spectators {
		sp.sendJSON(specMsg)
	}
}

// broadcastState sends state to all players (called while mu is held).
func (r *Room) broadcastState() {
	if r.white != nil {
		r.white.sendJSON(r.buildStateFor(RoleWhite))
	}
	if r.black != nil {
		r.black.sendJSON(r.buildStateFor(RoleBlack))
	}
	for _, sp := range r.spectators {
		sp.sendJSON(r.buildStateFor(RoleSpectator))
	}
}

func (r *Room) broadcastExcept(except *Player, v any) {
	if r.white != nil && r.white != except {
		r.white.sendJSON(v)
	}
	if r.black != nil && r.black != except {
		r.black.sendJSON(v)
	}
	for _, sp := range r.spectators {
		if sp != except {
			sp.sendJSON(v)
		}
	}
}

func (r *Room) broadcastRaw(data []byte) {
	r.mu.Lock()
	white := r.white
	black := r.black
	spectators := make([]*Player, len(r.spectators))
	copy(spectators, r.spectators)
	r.mu.Unlock()

	send := func(p *Player) {
		if p == nil {
			return
		}
		select {
		case p.send <- data:
		default:
		}
	}
	send(white)
	send(black)
	for _, sp := range spectators {
		send(sp)
	}
}

func (r *Room) buildStateFor(role PlayerRole) StateMessage {
	board := buildBoard(r.state)
	turn := "white"
	if r.state.Turn() == chess.Black {
		turn = "black"
	}

	ownTurn := !r.gameOver &&
		((r.state.Turn() == chess.White && role == RoleWhite) ||
			(r.state.Turn() == chess.Black && role == RoleBlack))

	var legalMoves []LegalMove
	if ownTurn {
		legalMoves = formatLegalMoves(r.legalMoves, r.state)
	}

	var lastMove *LegalMove
	if r.lastMove != nil {
		lastMove = formatMoveToLegal(*r.lastMove, r.state)
	}

	var whiteMs, blackMs int64
	hasClock := r.clock != nil
	if hasClock {
		whiteMs, blackMs = r.clock.Snapshot()
	}

	// Draw offer is visible to the recipient (non-offering side).
	drawOffered := false
	if r.drawOffer != nil {
		offerer := *r.drawOffer
		drawOffered = (role == RoleWhite && offerer == RoleBlack) ||
			(role == RoleBlack && offerer == RoleWhite)
	}

	// Claimable draw notice only for the player whose turn it is.
	var claimableDraw string
	if ownTurn {
		repCount := r.repetitions[chess.PositionKey(r.state)]
		claimableDraw = game.ClaimableDraw(r.state, repCount)
	}

	return StateMessage{
		Type:          "state",
		V:             ProtocolVersion,
		Board:         board,
		Turn:          turn,
		MoveNumber:    r.state.FullmoveNumber(),
		IsCheck:       chess.IsInCheck(r.state, r.state.Turn()),
		IsGameOver:    r.gameOver,
		Result:        r.result,
		LegalMoves:    legalMoves,
		LastMove:      lastMove,
		MoveHistory:   r.moveHistory,
		PlayerColor:   role.String(),
		RoomID:        r.id,
		WhiteMs:       whiteMs,
		BlackMs:       blackMs,
		HasClock:      hasClock,
		DrawOffered:   drawOffered,
		ClaimableDraw: claimableDraw,
	}
}
