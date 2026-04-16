package server

import "time"

// HubOptions configures optional Hub behaviour. A zero-value HubOptions is
// valid; NewHub fills in any missing rate-limit values with safe defaults.
type HubOptions struct {
	TrustProxy bool
	RateLimits RateLimitOptions
}

// RateLimitOptions holds the token-bucket parameters for each rate limiter.
type RateLimitOptions struct {
	// IPConn limits WebSocket upgrade attempts per client IP.
	IPConnRPS   float64
	IPConnBurst int
	// IPAuth limits login/register attempts per client IP (bcrypt guard).
	IPAuthRPS   float64
	IPAuthBurst int
	// PlayerAct limits room-creation and matchmaking requests per player.
	PlayerActRPS   float64
	PlayerActBurst int
	// Msg limits total WebSocket messages per player.
	MsgRPS   float64
	MsgBurst int
	// IdleTTL is how long a limiter bucket may be idle before eviction.
	IdleTTL time.Duration
}

// withDefaults fills in zero values with the compiled-in defaults. This keeps
// HubOptions zero-value-safe for tests that don't care about rate limiting.
func withDefaults(o RateLimitOptions) RateLimitOptions {
	if o.IPConnRPS == 0 {
		o.IPConnRPS = 5
	}
	if o.IPConnBurst == 0 {
		o.IPConnBurst = 10
	}
	if o.IPAuthRPS == 0 {
		o.IPAuthRPS = 0.2
	}
	if o.IPAuthBurst == 0 {
		o.IPAuthBurst = 5
	}
	if o.PlayerActRPS == 0 {
		o.PlayerActRPS = 2
	}
	if o.PlayerActBurst == 0 {
		o.PlayerActBurst = 10
	}
	if o.MsgRPS == 0 {
		o.MsgRPS = 20
	}
	if o.MsgBurst == 0 {
		o.MsgBurst = 40
	}
	if o.IdleTTL == 0 {
		o.IdleTTL = 10 * time.Minute
	}
	return o
}
