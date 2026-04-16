package server

import (
	"sync"
	"time"

	"github.com/fulstaph/gochess/chess"
)

// TimeControl defines initial time and per-move increment for a timed game.
type TimeControl struct {
	Initial   time.Duration
	Increment time.Duration
}

// ParseTimeControl converts a preset name to a TimeControl.
// Returns nil for "none" or unrecognised names (untimed game).
func ParseTimeControl(name string) *TimeControl {
	switch name {
	case "bullet1":
		return &TimeControl{time.Minute, 0}
	case "bullet2":
		return &TimeControl{2 * time.Minute, time.Second}
	case "blitz3":
		return &TimeControl{3 * time.Minute, 0}
	case "blitz5":
		return &TimeControl{5 * time.Minute, 3 * time.Second}
	case "rapid10":
		return &TimeControl{10 * time.Minute, 0}
	case "rapid15":
		return &TimeControl{15 * time.Minute, 10 * time.Second}
	}
	return nil
}

const clockStopped = 0 // sentinel for Clock.active when not running

// Clock tracks remaining time per player. The server is the sole authority;
// clients receive snapshots via StateMessage and display a local countdown.
type Clock struct {
	mu        sync.Mutex
	white     time.Duration
	black     time.Duration
	increment time.Duration
	active    int // chess.White, chess.Black, or clockStopped
	startedAt time.Time
	flagTimer *time.Timer
	onFlag    func(loser int) // called from a goroutine when time expires
}

// NewClock creates a Clock for a new game.
// onFlag is called (in a new goroutine) when a player runs out of time.
func NewClock(tc TimeControl, onFlag func(loser int)) *Clock {
	return &Clock{
		white:     tc.Initial,
		black:     tc.Initial,
		increment: tc.Increment,
		active:    clockStopped,
		onFlag:    onFlag,
	}
}

// Start begins the clock for turn (called after the first move, for the side now to move).
func (c *Clock) Start(turn int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = turn
	c.startedAt = time.Now()
	c.armTimer(turn, c.timeFor(turn))
}

// Punch records that movingTurn has made their move.
// It stops their clock (adding the increment) then starts the opponent's.
func (c *Clock) Punch(movingTurn int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != movingTurn {
		return
	}
	c.stopTimer()

	elapsed := time.Since(c.startedAt)
	if movingTurn == chess.White {
		c.white = max0(c.white-elapsed) + c.increment
	} else {
		c.black = max0(c.black-elapsed) + c.increment
	}

	other := chess.Black
	if movingTurn == chess.Black {
		other = chess.White
	}
	c.active = other
	c.startedAt = time.Now()
	c.armTimer(other, c.timeFor(other))
}

// Stop halts the clock permanently (game over).
func (c *Clock) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopTimer()
	c.active = clockStopped
}

// Snapshot returns current remaining milliseconds for each side,
// accounting for elapsed time in the active player's turn.
func (c *Clock) Snapshot() (whiteMs, blackMs int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return snapshotMs(c.timeFor(chess.White)), snapshotMs(c.timeFor(chess.Black))
}

// timeFor returns remaining time for turn, subtracting elapsed if it is their turn.
func (c *Clock) timeFor(turn int) time.Duration {
	if turn == chess.White {
		if c.active == chess.White {
			return c.white - time.Since(c.startedAt)
		}
		return c.white
	}
	if c.active == chess.Black {
		return c.black - time.Since(c.startedAt)
	}
	return c.black
}

func (c *Clock) stopTimer() {
	if c.flagTimer != nil {
		c.flagTimer.Stop()
		c.flagTimer = nil
	}
}

func (c *Clock) armTimer(turn int, remaining time.Duration) {
	if remaining <= 0 {
		go c.onFlag(turn)
		return
	}
	c.flagTimer = time.AfterFunc(remaining, func() { c.onFlag(turn) })
}

func max0(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

func snapshotMs(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}
