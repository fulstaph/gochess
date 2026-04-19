package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/fulstaph/gochess/store"
)

// sessionManager handles player identity and session persistence.
// When a store is available, sessions are DB-backed; otherwise they fall back
// to the in-memory map (used in tests without a database).
type sessionManager struct {
	db store.Store // may be nil (no database configured)

	// fallback in-memory store used when db == nil
	mem *memSessionStore
}

func newSessionManager(db store.Store) *sessionManager {
	return &sessionManager{db: db, mem: newMemSessionStore()}
}

// resolve returns the player ID, display name, and session token for a request.
// Creates a new guest identity if the token is absent or unrecognised.
func (sm *sessionManager) resolve(r *http.Request) (playerID, displayName, token string, err error) {
	tok := r.Header.Get("X-Session-Token")
	if tok == "" {
		tok = r.URL.Query().Get("token")
	}
	return sm.resolveToken(r.Context(), tok)
}

func (sm *sessionManager) resolveToken(ctx context.Context, tok string) (playerID, displayName, token string, err error) {
	if sm.db != nil {
		return sm.resolveDB(ctx, tok)
	}
	return sm.mem.resolve(tok)
}

// resolveDB resolves a token via the database.
func (sm *sessionManager) resolveDB(ctx context.Context, tok string) (playerID, displayName, token string, err error) {
	if tok != "" {
		sess, serr := sm.db.GetSession(ctx, tok)
		if serr == nil {
			p, perr := sm.db.GetPlayer(ctx, sess.PlayerID)
			if perr == nil {
				return p.ID, p.DisplayName, tok, nil
			}
		}
	}
	return sm.createGuest(ctx)
}

func (sm *sessionManager) createGuest(ctx context.Context) (playerID, displayName, token string, err error) {
	id, err := newID()
	if err != nil {
		return "", "", "", fmt.Errorf("create guest ID: %w", err)
	}
	displayName = fmt.Sprintf("Guest-%s", id[:6])
	token, err = newToken()
	if err != nil {
		return "", "", "", fmt.Errorf("create guest token: %w", err)
	}

	p := &store.Player{
		ID:          id,
		DisplayName: displayName,
		Rating:      1200,
		IsGuest:     true,
		CreatedAt:   time.Now(),
	}
	sess := &store.Session{
		Token:     token,
		PlayerID:  id,
		ExpiresAt: time.Now().Add(store.SessionTTL),
	}

	if sm.db != nil {
		if err := sm.db.SavePlayer(ctx, p); err != nil {
			log.Printf("auth: SavePlayer guest: %v", err)
		}
		if err := sm.db.SaveSession(ctx, sess); err != nil {
			log.Printf("auth: SaveSession guest: %v", err)
		}
	} else {
		sm.mem.put(token, id, displayName)
	}
	return id, displayName, token, nil
}

// Register creates a permanent account from a guest session.
// Returns (newToken, displayName, error).
func (sm *sessionManager) Register(ctx context.Context, currentPlayerID, username, password string) (string, string, error) {
	if sm.db == nil {
		return "", "", errors.New("database not configured")
	}
	if len(username) < 3 || len(username) > 32 {
		return "", "", errors.New("username must be 3–32 characters")
	}
	if len(password) < 6 {
		return "", "", errors.New("password must be at least 6 characters")
	}

	// Check username not taken.
	if _, err := sm.db.GetPlayerByUsername(ctx, username); err == nil {
		return "", "", errors.New("username already taken")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash password: %w", err)
	}

	// Upgrade existing guest player in-place.
	p, err := sm.db.GetPlayer(ctx, currentPlayerID)
	if err != nil {
		return "", "", fmt.Errorf("get player: %w", err)
	}
	p.Username = username
	p.PasswordHash = string(hash)
	p.DisplayName = username
	p.IsGuest = false
	if err := sm.db.SavePlayer(ctx, p); err != nil {
		return "", "", fmt.Errorf("save player: %w", err)
	}

	// Issue a fresh long-lived session token.
	newTok, err := newToken()
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	if err := sm.db.SaveSession(ctx, &store.Session{
		Token:     newTok,
		PlayerID:  p.ID,
		ExpiresAt: time.Now().Add(store.SessionTTL),
	}); err != nil {
		return "", "", fmt.Errorf("save session: %w", err)
	}
	return newTok, p.DisplayName, nil
}

// Login authenticates a registered user.
// Returns (token, playerID, displayName, error).
func (sm *sessionManager) Login(ctx context.Context, username, password string) (string, string, string, error) {
	if sm.db == nil {
		return "", "", "", errors.New("database not configured")
	}
	p, err := sm.db.GetPlayerByUsername(ctx, username)
	if err != nil {
		return "", "", "", errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(password)); err != nil {
		return "", "", "", errors.New("invalid username or password")
	}

	tok, err := newToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate token: %w", err)
	}
	if err := sm.db.SaveSession(ctx, &store.Session{
		Token:     tok,
		PlayerID:  p.ID,
		ExpiresAt: time.Now().Add(store.SessionTTL),
	}); err != nil {
		return "", "", "", fmt.Errorf("save session: %w", err)
	}
	return tok, p.ID, p.DisplayName, nil
}

// Rating returns the persisted rating for a player (falls back to in-memory).
func (sm *sessionManager) RatingFor(ctx context.Context, playerID string) int {
	if sm.db == nil {
		return 1200
	}
	p, err := sm.db.GetPlayer(ctx, playerID)
	if err != nil {
		return 1200
	}
	return p.Rating
}

// ---- in-memory fallback (used without a database) ----

type memEntry struct {
	playerID   string
	lastAccess time.Time
}

type memSessionStore struct {
	mu     sync.RWMutex
	tokens map[string]memEntry // token → entry
	names  map[string]string   // playerID → displayName
}

func newMemSessionStore() *memSessionStore {
	return &memSessionStore{
		tokens: make(map[string]memEntry),
		names:  make(map[string]string),
	}
}

func (m *memSessionStore) put(token, playerID, displayName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = memEntry{playerID: playerID, lastAccess: time.Now()}
	m.names[playerID] = displayName
}

func (m *memSessionStore) resolve(tok string) (playerID, displayName, token string, err error) {
	// Fast path: check existing token under read lock.
	if tok != "" {
		m.mu.RLock()
		entry, ok := m.tokens[tok]
		name := m.names[entry.playerID]
		m.mu.RUnlock()
		if ok {
			// Update last access under write lock.
			m.mu.Lock()
			if e, exists := m.tokens[tok]; exists {
				e.lastAccess = time.Now()
				m.tokens[tok] = e
			}
			m.mu.Unlock()
			return entry.playerID, name, tok, nil
		}
	}

	// Slow path: create new guest under write lock.
	pid, err := newID()
	if err != nil {
		return "", "", "", err
	}
	tok, err = newToken()
	if err != nil {
		return "", "", "", err
	}
	name := fmt.Sprintf("Guest-%s", pid[:6])

	m.mu.Lock()
	m.tokens[tok] = memEntry{playerID: pid, lastAccess: time.Now()}
	m.names[pid] = name
	m.mu.Unlock()

	return pid, name, tok, nil
}

// cleanup removes entries that haven't been accessed within maxAge.
func (m *memSessionStore) cleanup(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok, entry := range m.tokens {
		if entry.lastAccess.Before(cutoff) {
			delete(m.names, entry.playerID)
			delete(m.tokens, tok)
		}
	}
}

// ---- crypto helpers ----

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}
