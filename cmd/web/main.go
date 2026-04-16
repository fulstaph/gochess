package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/fulstaph/gochess/server"
	"github.com/fulstaph/gochess/store"
)

func main() {
	port := flag.Int("port", 8080, "HTTP port")
	dbURL := flag.String("db", "", "PostgreSQL DSN (e.g. postgres://gochess:gochess@localhost/gochess). Falls back to DATABASE_URL env var. Omit for in-memory mode.")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}

	var db store.Store
	if *dbURL != "" {
		pg, err := store.Open(context.Background(), *dbURL)
		if err != nil {
			log.Fatalf("connect to database: %v", err)
		}
		db = pg
		log.Printf("connected to postgres")
		defer pg.Close()
	} else {
		log.Printf("no --db flag; running with in-memory storage (ratings and games will not persist)")
	}

	hub := server.NewHub(db)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", hub.HandleWebSocket)
	mux.HandleFunc("GET /api/rooms", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hub.ListRooms())
	})

	if pg, ok := db.(*store.Postgres); ok {
		mux.HandleFunc("GET /api/games", listGamesHandler(pg))
		mux.HandleFunc("GET /api/games/{id}", getGameHandler(pg))
		mux.HandleFunc("GET /api/players/{id}", getPlayerHandler(pg))
	}

	mux.Handle("/", http.FileServer(http.Dir("web/dist")))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("gochess web server listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func listGamesHandler(db *store.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID := r.URL.Query().Get("player")
		if playerID == "" {
			http.Error(w, "player query param required", http.StatusBadRequest)
			return
		}
		limit := queryInt(r, "limit", 20)
		offset := queryInt(r, "offset", 0)
		if limit > 100 {
			limit = 100
		}

		games, err := db.ListGamesByPlayer(r.Context(), playerID, limit, offset)
		if err != nil {
			log.Printf("ListGamesByPlayer: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if games == nil {
			games = []*store.Game{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(games)
	}
}

func getGameHandler(db *store.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		g, err := db.GetGame(r.Context(), id)
		if err == store.ErrNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g)
	}
}

func getPlayerHandler(db *store.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		p, err := db.GetPlayer(r.Context(), id)
		if err == store.ErrNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		stats, err := db.GetPlayerStats(r.Context(), id)
		if err != nil {
			log.Printf("GetPlayerStats: %v", err)
		}
		type profileResponse struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Rating      int    `json:"rating"`
			IsGuest     bool   `json:"isGuest"`
			Wins        int    `json:"wins"`
			Losses      int    `json:"losses"`
			Draws       int    `json:"draws"`
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profileResponse{
			ID:          p.ID,
			DisplayName: p.DisplayName,
			Rating:      p.Rating,
			IsGuest:     p.IsGuest,
			Wins:        stats.Wins,
			Losses:      stats.Losses,
			Draws:       stats.Draws,
		})
	}
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}
