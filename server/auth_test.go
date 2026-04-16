package server

import (
	"encoding/hex"
	"sync"
	"testing"
)

func TestNewID_Valid(t *testing.T) {
	id, err := newID()
	if err != nil {
		t.Fatalf("newID() error: %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("expected 32-char hex string, got %d chars: %q", len(id), id)
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("newID() returned invalid hex: %q", id)
	}
}

func TestNewToken_Valid(t *testing.T) {
	tok, err := newToken()
	if err != nil {
		t.Fatalf("newToken() error: %v", err)
	}
	if len(tok) != 64 {
		t.Fatalf("expected 64-char hex string, got %d chars: %q", len(tok), tok)
	}
	if _, err := hex.DecodeString(tok); err != nil {
		t.Fatalf("newToken() returned invalid hex: %q", tok)
	}
}

func TestNewID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := newID()
		if err != nil {
			t.Fatalf("newID() error on iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID on iteration %d: %q", i, id)
		}
		seen[id] = true
	}
}

func TestMemSessionStore_ConcurrentResolve(t *testing.T) {
	store := newMemSessionStore()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	ids := make([]string, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id, _, _, err := store.resolve("")
			ids[idx] = id
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: resolve error: %v", i, err)
		}
	}

	// All IDs should be unique (each call with empty token creates a new guest).
	seen := make(map[string]bool)
	for i, id := range ids {
		if seen[id] {
			t.Fatalf("goroutine %d: duplicate ID %q", i, id)
		}
		seen[id] = true
	}
}

func TestMemSessionStore_ConcurrentResolveExisting(t *testing.T) {
	store := newMemSessionStore()

	// Create a guest first.
	origID, origName, tok, err := store.resolve("")
	if err != nil {
		t.Fatalf("initial resolve error: %v", err)
	}

	// Concurrent lookups of the same token should all return the same player.
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ids := make([]string, goroutines)
	names := make([]string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id, name, _, _ := store.resolve(tok)
			ids[idx] = id
			names[idx] = name
		}(i)
	}
	wg.Wait()

	for i := range ids {
		if ids[i] != origID {
			t.Fatalf("goroutine %d: expected ID %q, got %q", i, origID, ids[i])
		}
		if names[i] != origName {
			t.Fatalf("goroutine %d: expected name %q, got %q", i, origName, names[i])
		}
	}
}
