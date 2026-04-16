package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// TestRateLimiter_Allow_Burst verifies that a burst of requests is allowed up
// to the configured burst size, then denied.
func TestRateLimiter_Allow_Burst(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1), 3, time.Minute)

	results := make([]bool, 4)
	for i := range results {
		results[i] = rl.Allow("key")
	}

	// First three should be allowed (burst=3), fourth denied.
	for i := 0; i < 3; i++ {
		if !results[i] {
			t.Errorf("call %d: expected allowed, got denied", i)
		}
	}
	if results[3] {
		t.Error("call 3: expected denied (burst exhausted), got allowed")
	}
}

// TestRateLimiter_Allow_Refill verifies that tokens refill over time.
func TestRateLimiter_Allow_Refill(t *testing.T) {
	// rps=100 → one token every 10ms, burst=1
	rl := NewRateLimiter(rate.Limit(100), 1, time.Minute)

	if !rl.Allow("x") {
		t.Fatal("first call should be allowed")
	}
	if rl.Allow("x") {
		t.Fatal("second immediate call should be denied")
	}
	// Wait for one token to refill (>10ms)
	time.Sleep(20 * time.Millisecond)
	if !rl.Allow("x") {
		t.Fatal("call after refill sleep should be allowed")
	}
}

// TestRateLimiter_Allow_PerKey verifies that different keys have independent buckets.
func TestRateLimiter_Allow_PerKey(t *testing.T) {
	// burst=1, so each key gets exactly one token.
	rl := NewRateLimiter(rate.Limit(1), 1, time.Minute)

	if !rl.Allow("alice") {
		t.Error("alice first call: expected allowed")
	}
	if rl.Allow("alice") {
		t.Error("alice second call: expected denied")
	}
	// bob has his own bucket, unaffected by alice.
	if !rl.Allow("bob") {
		t.Error("bob first call: expected allowed (independent bucket)")
	}
}

// TestRateLimiter_Cleanup_RemovesIdle verifies that cleanup evicts idle buckets.
func TestRateLimiter_Cleanup_RemovesIdle(t *testing.T) {
	idleTTL := time.Minute
	rl := NewRateLimiter(rate.Limit(1), 1, idleTTL)

	rl.Allow("a")
	rl.Allow("b")

	rl.mu.Lock()
	count := len(rl.buckets)
	rl.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 buckets before cleanup, got %d", count)
	}

	// Simulate time past idleTTL.
	rl.cleanup(time.Now().Add(2 * idleTTL))

	rl.mu.Lock()
	count = len(rl.buckets)
	rl.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 buckets after cleanup, got %d", count)
	}
}

// TestClientIP covers the trustProxy=false and trustProxy=true paths,
// including spoofed comma-separated X-Forwarded-For values.
func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		trustProxy bool
		expectedIP string
	}{
		{
			name:       "direct connection, no XFF",
			remoteAddr: "1.2.3.4:5678",
			trustProxy: false,
			expectedIP: "1.2.3.4",
		},
		{
			name:       "trustProxy=false ignores XFF",
			remoteAddr: "1.2.3.4:5678",
			xff:        "9.9.9.9",
			trustProxy: false,
			expectedIP: "1.2.3.4",
		},
		{
			name:       "trustProxy=true honours XFF",
			remoteAddr: "1.2.3.4:5678",
			xff:        "9.9.9.9",
			trustProxy: true,
			expectedIP: "9.9.9.9",
		},
		{
			name:       "trustProxy=true, XFF comma-separated (spoofed chain)",
			remoteAddr: "1.2.3.4:5678",
			xff:        "evil.ip.0.0, 9.9.9.9",
			trustProxy: true,
			expectedIP: "evil.ip.0.0",
		},
		{
			name:       "trustProxy=true, empty XFF falls back to RemoteAddr",
			remoteAddr: "1.2.3.4:5678",
			xff:        "",
			trustProxy: true,
			expectedIP: "1.2.3.4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := clientIP(req, tc.trustProxy)
			if got != tc.expectedIP {
				t.Errorf("clientIP() = %q, want %q", got, tc.expectedIP)
			}
		})
	}
}
