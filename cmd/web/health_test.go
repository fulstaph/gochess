package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fulstaph/gochess/store"
)

// fakeStore is a minimal store.Store implementation for testing health handlers.
// Only Ping has meaningful behaviour; all other methods return zero values.
type fakeStore struct {
	pingErr error
}

func (f *fakeStore) SavePlayer(_ context.Context, _ *store.Player) error { return nil }
func (f *fakeStore) GetPlayer(_ context.Context, _ string) (*store.Player, error) {
	return nil, nil
}
func (f *fakeStore) GetPlayerByUsername(_ context.Context, _ string) (*store.Player, error) {
	return nil, nil
}
func (f *fakeStore) UpdateRating(_ context.Context, _ string, _ int) error { return nil }
func (f *fakeStore) SaveGame(_ context.Context, _ *store.Game) error       { return nil }
func (f *fakeStore) GetGame(_ context.Context, _ string) (*store.Game, error) {
	return nil, nil
}
func (f *fakeStore) ListGamesByPlayer(_ context.Context, _ string, _, _ int) ([]*store.Game, error) {
	return nil, nil
}
func (f *fakeStore) SaveSession(_ context.Context, _ *store.Session) error { return nil }
func (f *fakeStore) GetSession(_ context.Context, _ string) (*store.Session, error) {
	return nil, nil
}
func (f *fakeStore) DeleteSession(_ context.Context, _ string) error { return nil }
func (f *fakeStore) DeleteExpiredSessions(_ context.Context) error   { return nil }
func (f *fakeStore) Ping(_ context.Context) error                    { return f.pingErr }
func (f *fakeStore) Close()                                          {}

func TestHealthz(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReadyz_NilDB(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", readyzHandler(nil))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (nil DB = in-memory, always ready)", resp.StatusCode)
	}
}

func TestReadyz_HealthyDB(t *testing.T) {
	db := &fakeStore{pingErr: nil}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", readyzHandler(db))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (healthy DB)", resp.StatusCode)
	}
}

func TestReadyz_FailingDB(t *testing.T) {
	db := &fakeStore{pingErr: errors.New("connection refused")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", readyzHandler(db))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (DB unreachable)", resp.StatusCode)
	}
}

// Compile-time check that fakeStore satisfies the full Store interface.
var _ store.Store = (*fakeStore)(nil)
