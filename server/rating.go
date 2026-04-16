package server

import (
	"math"
	"sync"
	"time"
)

const (
	initialRating = 1200
	kFactor       = 32
)

// Rater tracks Elo ratings in memory.
type Rater struct {
	mu          sync.Mutex
	ratings     map[string]int
	lastUpdated map[string]time.Time
}

func newRater() *Rater {
	return &Rater{
		ratings:     make(map[string]int),
		lastUpdated: make(map[string]time.Time),
	}
}

// Rating returns the current Elo rating for playerID (1200 if unseen).
func (r *Rater) Rating(playerID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ratingLocked(playerID)
}

// UpdateGame adjusts ratings after a completed game.
// Pass isDraw=true for draws; otherwise winnerID wins and loserID loses.
// AI player IDs ("") are ignored.
func (r *Rater) UpdateGame(winnerID, loserID string, isDraw bool) {
	if winnerID == "" || loserID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	wa := r.ratingLocked(winnerID)
	la := r.ratingLocked(loserID)

	ea := 1.0 / (1.0 + math.Pow(10, float64(la-wa)/400.0))
	eb := 1.0 - ea

	var sa, sb float64
	if isDraw {
		sa, sb = 0.5, 0.5
	} else {
		sa, sb = 1.0, 0.0
	}

	now := time.Now()
	r.ratings[winnerID] = wa + int(math.Round(float64(kFactor)*(sa-ea)))
	r.ratings[loserID] = la + int(math.Round(float64(kFactor)*(sb-eb)))
	r.lastUpdated[winnerID] = now
	r.lastUpdated[loserID] = now
}

func (r *Rater) ratingLocked(id string) int {
	if v, ok := r.ratings[id]; ok {
		return v
	}
	return initialRating
}

// cleanup removes rating entries that haven't been updated within maxAge
// and still have the default rating.
func (r *Rater) cleanup(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.lastUpdated {
		if t.Before(cutoff) && r.ratings[id] == initialRating {
			delete(r.ratings, id)
			delete(r.lastUpdated, id)
		}
	}
}
