package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulstaph/gochess/chess"
)

// ---- helpers ----

func testPlayer(id string) *Player {
	return &Player{
		ID:          id,
		DisplayName: id,
		send:        make(chan []byte, 64),
	}
}

func testRoom(vsAI bool) *Room {
	r := newRoom("test", nil, RoleWhite, vsAI, 1)
	return r
}

// drainType reads messages from the player's send channel and returns
// the types of messages received.
func drainTypes(p *Player) []string {
	var types []string
	for {
		select {
		case data := <-p.send:
			var msg map[string]json.RawMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			var t string
			_ = json.Unmarshal(msg["type"], &t)
			types = append(types, t)
		default:
			return types
		}
	}
}

// ---- Room.addPlayer ----

func TestAddPlayer_RolesAssigned(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	black := testPlayer("b")

	role1, started1 := r.addPlayer(white, "white")
	if role1 != RoleWhite {
		t.Fatalf("first player should be white, got %v", role1)
	}
	if started1 {
		t.Fatal("game should not start with only one player")
	}

	role2, started2 := r.addPlayer(black, "black")
	if role2 != RoleBlack {
		t.Fatalf("second player should be black, got %v", role2)
	}
	if !started2 {
		t.Fatal("game should start when both seats filled")
	}
	if r.status != RoomPlaying {
		t.Fatalf("room status should be Playing, got %v", r.status)
	}
}

func TestAddPlayer_ThirdPlayerIsSpectator(t *testing.T) {
	r := testRoom(false)
	r.addPlayer(testPlayer("w"), "white")
	r.addPlayer(testPlayer("b"), "black")

	spectator := testPlayer("s")
	role, started := r.addPlayer(spectator, "")
	if role != RoleSpectator {
		t.Fatalf("third player should be spectator, got %v", role)
	}
	if started {
		t.Fatal("started should be false for spectator")
	}
}

func TestAddPlayer_VSAIStartsImmediately(t *testing.T) {
	r := testRoom(true)
	p := testPlayer("p")
	_, started := r.addPlayer(p, "white")
	if !started {
		t.Fatal("vs-AI game should start with just one player")
	}
	if r.status != RoomPlaying {
		t.Fatalf("room status should be Playing for vs-AI, got %v", r.status)
	}
}

// ---- Room.ApplyMove ----

func TestRoomApplyMove_ValidMove(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	black := testPlayer("b")
	r.addPlayer(white, "white")
	r.addPlayer(black, "black")

	r.ApplyMove(white, "e2", "e4", "")

	if r.state.Turn() != chess.Black {
		t.Fatal("turn should be black after white plays")
	}
	if r.lastMove == nil {
		t.Fatal("lastMove should be set after a move")
	}
}

func TestRoomApplyMove_WrongTurn(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	black := testPlayer("b")
	r.addPlayer(white, "white")
	r.addPlayer(black, "black")

	// It's white's turn; black tries to move.
	r.ApplyMove(black, "e7", "e5", "")

	types := drainTypes(black)
	found := false
	for _, tt := range types {
		if tt == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected error message for wrong-turn move")
	}
	if r.state.Turn() != chess.White {
		t.Fatal("turn should still be white after rejected move")
	}
}

func TestRoomApplyMove_Spectator(t *testing.T) {
	r := testRoom(false)
	r.addPlayer(testPlayer("w"), "white")
	r.addPlayer(testPlayer("b"), "black")
	spectator := testPlayer("s")

	r.ApplyMove(spectator, "e2", "e4", "")

	types := drainTypes(spectator)
	found := false
	for _, tt := range types {
		if tt == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected error for spectator trying to move")
	}
}

func TestRoomApplyMove_GameNotStarted(t *testing.T) {
	r := testRoom(false) // waiting for second player
	white := testPlayer("w")
	r.addPlayer(white, "white")

	r.ApplyMove(white, "e2", "e4", "")

	types := drainTypes(white)
	found := false
	for _, tt := range types {
		if tt == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected error when game has not started")
	}
}

// ---- Room.Resign ----

func TestRoomResign_WhiteResigns(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	black := testPlayer("b")
	r.addPlayer(white, "white")
	r.addPlayer(black, "black")

	r.Resign(white) // white resigns → black wins

	if !r.gameOver {
		t.Fatal("game must be over after resign")
	}
	if !strings.Contains(r.result, "Black") {
		t.Fatalf("expected Black wins, got %q", r.result)
	}
	if r.status != RoomFinished {
		t.Fatal("room status should be Finished")
	}
}

func TestRoomResign_NoOpWhenOver(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	r.addPlayer(white, "white")
	r.gameOver = true
	r.result = "existing result"

	r.Resign(white)

	if r.result != "existing result" {
		t.Fatal("Resign should not overwrite existing result")
	}
}

// ---- Room.buildStateFor ----

func TestBuildStateFor_LegalMovesOnlyForCorrectSide(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	black := testPlayer("b")
	r.addPlayer(white, "white")
	r.addPlayer(black, "black")

	// It's white's turn: white should get legal moves, black should not.
	r.mu.Lock()
	whiteMsg := r.buildStateFor(RoleWhite)
	blackMsg := r.buildStateFor(RoleBlack)
	r.mu.Unlock()

	if len(whiteMsg.LegalMoves) == 0 {
		t.Fatal("white should have legal moves on their turn")
	}
	if len(blackMsg.LegalMoves) != 0 {
		t.Fatal("black should not receive legal moves when it is not their turn")
	}
}

func TestBuildStateFor_PlayerColorSet(t *testing.T) {
	r := testRoom(false)
	white := testPlayer("w")
	black := testPlayer("b")
	r.addPlayer(white, "white")
	r.addPlayer(black, "black")

	r.mu.Lock()
	whiteMsg := r.buildStateFor(RoleWhite)
	blackMsg := r.buildStateFor(RoleBlack)
	r.mu.Unlock()

	if whiteMsg.PlayerColor != "white" {
		t.Fatalf("expected playerColor 'white', got %q", whiteMsg.PlayerColor)
	}
	if blackMsg.PlayerColor != "black" {
		t.Fatalf("expected playerColor 'black', got %q", blackMsg.PlayerColor)
	}
}

// ---- sessionManager (in-memory fallback) ----

func TestSessionStore_NewGuest(t *testing.T) {
	sm := newSessionManager(nil)

	// No token → create new guest.
	id1, name1, tok1, err := sm.mem.resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id1 == "" || name1 == "" || tok1 == "" {
		t.Fatal("expected non-empty id, name, token for new guest")
	}
	if !strings.HasPrefix(name1, "Guest-") {
		t.Fatalf("expected Guest- prefix, got %q", name1)
	}

	// Same token → same player.
	id2, name2, _, err := sm.mem.resolve(tok1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same playerID on reconnect, got %q vs %q", id1, id2)
	}
	if name2 != name1 {
		t.Fatalf("expected same displayName on reconnect, got %q vs %q", name1, name2)
	}
}

func TestSessionStore_UnknownTokenCreatesNew(t *testing.T) {
	sm := newSessionManager(nil)

	id1, _, _, _ := sm.mem.resolve("")
	id2, _, _, _ := sm.mem.resolve("bogus-token-that-was-never-issued")

	if id1 == id2 {
		t.Fatal("unknown token should create a new player, not reuse existing")
	}
}

// ---- Draw mechanics ----

func TestRoom_DrawOffer_Accept(t *testing.T) {
	r := testRoom(false)
	w := testPlayer("white")
	b := testPlayer("black")
	r.addPlayer(w, "white")
	r.addPlayer(b, "black")

	// Make a move first so the game is started.
	r.ApplyMove(w, "e2", "e4", "")
	drainTypes(w)
	drainTypes(b)

	r.OfferDraw(w)
	bTypes := drainTypes(b)
	hasDraw := false
	for _, t := range bTypes {
		if t == "draw_offered" {
			hasDraw = true
		}
	}
	if !hasDraw {
		t.Fatal("expected black to receive draw_offered")
	}

	r.RespondDraw(b, true)

	r.mu.Lock()
	gameOver := r.gameOver
	result := r.result
	r.mu.Unlock()

	if !gameOver {
		t.Fatal("expected game to be over after draw accepted")
	}
	if !strings.Contains(result, "Draw") {
		t.Fatalf("expected draw result, got %q", result)
	}
}

func TestRoom_DrawOffer_Decline(t *testing.T) {
	r := testRoom(false)
	w := testPlayer("white")
	b := testPlayer("black")
	r.addPlayer(w, "white")
	r.addPlayer(b, "black")

	r.ApplyMove(w, "e2", "e4", "")
	drainTypes(w)
	drainTypes(b)

	r.OfferDraw(w)
	drainTypes(b)
	r.RespondDraw(b, false)

	r.mu.Lock()
	gameOver := r.gameOver
	drawOffer := r.drawOffer
	r.mu.Unlock()

	if gameOver {
		t.Fatal("expected game to continue after draw declined")
	}
	if drawOffer != nil {
		t.Fatal("expected draw offer to be cleared after decline")
	}
}

func TestRoom_DrawOffer_ClearedOnMove(t *testing.T) {
	r := testRoom(false)
	w := testPlayer("white")
	b := testPlayer("black")
	r.addPlayer(w, "white")
	r.addPlayer(b, "black")

	r.ApplyMove(w, "e2", "e4", "")
	drainTypes(w)
	drainTypes(b)

	r.OfferDraw(b)
	drainTypes(w)

	// Black makes a move — draw offer should be cleared.
	r.ApplyMove(b, "e7", "e5", "")

	r.mu.Lock()
	drawOffer := r.drawOffer
	r.mu.Unlock()

	if drawOffer != nil {
		t.Fatal("expected draw offer to be cleared after a move")
	}
}

// ---- Undo (vs-AI) ----

func TestRoom_Undo_VsAI(t *testing.T) {
	r := testRoom(true)
	w := testPlayer("white")
	r.addPlayer(w, "white")
	drainTypes(w)

	// Apply a move manually (simulate human move without triggering AI goroutine).
	r.mu.Lock()
	mv, err := chess.ParseMove("e2e4", r.state)
	if err != nil {
		r.mu.Unlock()
		t.Fatalf("ParseMove: %v", err)
	}
	r.applyAndRecord(mv)
	initialState := r.state
	r.mu.Unlock()
	drainTypes(w)

	// Undo — only 1 snapshot, so it pops 1.
	r.Undo(w)
	drainTypes(w)

	r.mu.Lock()
	stateAfterUndo := r.state
	moveCount := r.moveCount
	r.mu.Unlock()

	if moveCount != 0 {
		t.Fatalf("expected moveCount=0 after undo, got %d", moveCount)
	}
	if stateAfterUndo.Board() == initialState.Board() {
		t.Fatal("expected state to be different after undo")
	}
}

func TestRoom_Undo_NotAllowed_Multiplayer(t *testing.T) {
	r := testRoom(false)
	w := testPlayer("white")
	b := testPlayer("black")
	r.addPlayer(w, "white")
	r.addPlayer(b, "black")
	drainTypes(w)
	drainTypes(b)

	r.Undo(w)
	types := drainTypes(w)
	hasError := false
	for _, tp := range types {
		if tp == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected error when trying to undo in multiplayer")
	}
}
