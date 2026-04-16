package server

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	matchRatingWindow = 200 // Elo points; widen to any after grace period
	matchGracePeriod  = 30 * time.Second
)

type queueEntry struct {
	player *Player
	tc     string // time control preset name
	rating int
	queued time.Time
}

// Matchmaker pairs players with compatible time controls and close Elo ratings.
type Matchmaker struct {
	mu    sync.Mutex
	queue []*queueEntry
}

func newMatchmaker() *Matchmaker {
	return &Matchmaker{}
}

// Enqueue adds a player to the queue and returns a matched pair if one is found.
// Returns (white, black) or (nil, nil) when no match yet.
func (mm *Matchmaker) Enqueue(p *Player, tc string, rating int) (*queueEntry, *queueEntry) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Remove any stale entry for this player.
	mm.removePlayer(p.ID)

	entry := &queueEntry{player: p, tc: tc, rating: rating, queued: time.Now()}

	// Try to find a waiting opponent.
	for i, candidate := range mm.queue {
		if candidate.tc != tc {
			continue
		}
		elapsed := time.Since(candidate.queued)
		ratingDiff := entry.rating - candidate.rating
		if ratingDiff < 0 {
			ratingDiff = -ratingDiff
		}
		// Accept the match if ratings are close enough, or the candidate has been waiting long.
		if ratingDiff <= matchRatingWindow || elapsed >= matchGracePeriod {
			// Remove candidate from queue and return the pair.
			mm.queue = append(mm.queue[:i], mm.queue[i+1:]...)
			return candidate, entry
		}
	}

	mm.queue = append(mm.queue, entry)
	return nil, nil
}

// Dequeue removes a player from the queue. Returns true if they were queued.
func (mm *Matchmaker) Dequeue(playerID string) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return mm.removePlayer(playerID)
}

// removePlayer is the unlocked helper — caller must hold mm.mu.
func (mm *Matchmaker) removePlayer(playerID string) bool {
	for i, e := range mm.queue {
		if e.player.ID == playerID {
			mm.queue = append(mm.queue[:i], mm.queue[i+1:]...)
			return true
		}
	}
	return false
}

// handleFindGame is called from Hub.dispatch.
func (h *Hub) handleFindGame(p *Player, msg ClientMessage) {
	tc := msg.TimeControl
	if tc == "" {
		tc = "blitz5"
	}

	rating := h.rater.Rating(p.ID)
	white, black := h.matchmaker.Enqueue(p, tc, rating)

	if white == nil {
		// Still waiting.
		p.sendJSON(MatchWaitingMessage{
			Type:        "match_waiting",
			V:           ProtocolVersion,
			TimeControl: tc,
		})
		return
	}

	// Pair found — create a room and notify both players.
	id, err := newID()
	if err != nil {
		log.Printf("matchmaking room ID: %v", err)
		p.sendJSON(ErrorMessage{Type: "error", V: ProtocolVersion, Message: "internal error"})
		return
	}
	roomID := id[:8]
	room := newRoom(roomID, h, RoleWhite, false, 0)
	if tcCfg := ParseTimeControl(tc); tcCfg != nil {
		room.setTimeControl(tcCfg, tc, true) // rated
	}

	h.mu.Lock()
	h.rooms[roomID] = room
	h.mu.Unlock()

	room.addPlayer(white.player, "white")
	room.addPlayer(black.player, "black")

	// Send match-found before the initial state so the client knows the room/color.
	white.player.sendJSON(MatchFoundMessage{
		Type: "match_found", V: ProtocolVersion,
		RoomID: roomID, PlayerColor: "white",
		OpponentName: black.player.DisplayName,
	})
	black.player.sendJSON(MatchFoundMessage{
		Type: "match_found", V: ProtocolVersion,
		RoomID: roomID, PlayerColor: "black",
		OpponentName: white.player.DisplayName,
	})

	room.mu.Lock()
	room.broadcastState()
	room.mu.Unlock()

	if room.shouldAIMove() {
		go room.runAIMove(context.Background())
	}
}

// handleCancelMatch removes the player from the matchmaking queue.
func (h *Hub) handleCancelMatch(p *Player) {
	h.matchmaker.Dequeue(p.ID)
	p.sendJSON(MatchCancelledMessage{Type: "match_cancelled", V: ProtocolVersion})
}
