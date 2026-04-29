# GoChess

A chess engine and multiplayer platform written in Go, with a terminal UI (Bubble Tea) and a web UI (TypeScript + WebSocket).

## Project Structure

```
chess/           Core engine: move generation, rules, AI (minimax + alpha-beta), FEN, PGN
game/            Shared game-over detection and move recording logic
server/          WebSocket multiplayer: hub, rooms, matchmaking, rating, auth, clock, rate limiting
store/           Persistence layer: Store interface, PostgreSQL impl, in-memory impl, migrations
cmd/web/         Web server entry point (HTTP routes, config, health checks)
web/             TypeScript frontend (src/ → esbuild → dist/)
main.go          TUI entry point (Bubble Tea, flag parsing)
tui.go           TUI Bubble Tea model: Update, View, AI integration
ui.go            TUI board rendering with lipgloss, move list sidebar, help panel
game_helpers.go  TUI game state helpers (game-over detection wrappers)
```

## Commands

### Build & Run

```sh
task tui                  # Run terminal UI (AI plays black, depth 2)
task web                  # Build frontend + run web server on :8080
task web:server           # Run web server without rebuilding frontend
task web:build            # Build TypeScript frontend only
task web:watch            # Watch TypeScript (hot-reload frontend)
task dev                  # Hot-reload Go server (air) + watch TypeScript
task dev:run db=postgres://gochess:gochess@localhost:5432/gochess  # Run with DB
task build                # Build both binaries (./gochess, ./gochess-web)
task build:tui            # Build TUI binary only
task build:web            # Build web server binary only
```

### Test & Lint

```sh
task test                 # go test ./...
task test:verbose         # go test -v ./...
task test:cover           # Tests with coverage report
task test:api             # Bruno API tests
task lint                 # Lint Go + TypeScript
task lint:go              # golangci-lint run ./...
task lint:ts              # npx tsc --noEmit (in web/)
task ci                   # Full CI pipeline locally (build, lint, test, frontend)
```

### Database (optional, for multiplayer persistence)

```sh
docker compose up -d      # Start PostgreSQL 16
# Set DATABASE_URL=postgres://gochess:gochess@localhost:5432/gochess
task dev:run db=postgres://gochess:gochess@localhost:5432/gochess
```

## Key Types

### chess package

```go
// Immutable game state — never mutated, ApplyMove returns a new copy
type GameState struct {
    board          [8][8]rune  // pieces as Unicode runes (e.g. '♔', '♟')
    turn           int          // chess.White (1) or chess.Black (-1)
    castling       [4]bool      // [wKingside, wQueenside, bKingside, bQueenside]
    enPassantR     int          // en-passant target row (-1 if none)
    enPassantC     int          // en-passant target col (-1 if none)
    halfmove       int          // for 50/75-move rule
    fullmoveNum    int          // for PGN
}

type Move struct {
    fromR, fromC   int
    toR, toC       int
    promotion      rune  // non-zero for pawn promotion
    isCastle       bool
    isEnPassant    bool
}
```

Key functions: `InitialState()`, `ApplyMove(state, move)`, `GenerateLegalMoves(state)`,
`IsInCheck(state, color)`, `BestMove(state, depth)`, `ToFEN(state)`, `ParseFEN(fen)`,
`ParseMove(state, input)`, `FormatMove(move)`, `PositionKey(state)`.

### store package

```go
type Store interface {
    SavePlayer(ctx, *Player) error
    GetPlayer(ctx, id) (*Player, error)
    GetPlayerByUsername(ctx, username) (*Player, error)
    SaveGame(ctx, *Game) error
    GetGame(ctx, id) (*Game, error)
    ListGamesByPlayer(ctx, playerID) ([]*Game, error)
    SaveSession(ctx, *Session) error
    GetSession(ctx, token) (*Session, error)
    DeleteSession(ctx, token) error
    DeleteExpiredSessions(ctx) error
    Ping(ctx) error
    Close() error
}
```

When `DATABASE_URL` is unset, the server uses a thread-safe in-memory implementation.

## Chess Engine

**AI algorithm**: Negamax (minimax) with alpha-beta pruning, iterative deepening, and quiescence search.

- Piece values: Pawn=100, Knight=320, Bishop=330, Rook=500, Queen=900, Mate=100,000
- Evaluation: material + piece-square tables (PST) with tapered eval (blends opening/endgame)
- Move ordering: killer moves + history heuristic for better alpha-beta cutoffs
- Quiescence search: up to depth 8 beyond nominal depth to avoid horizon effect
- Iterative deepening: seeds best move from shallower search for ordering in deeper passes

**Draw detection** (in `game/logic.go` + `chess/position.go`):
- 50-move rule (halfmove ≥ 100), 75-move automatic draw (halfmove ≥ 150)
- Threefold repetition (claimable), fivefold repetition (automatic)
- Stalemate (no legal moves, not in check)

## Server Architecture

**WebSocket connection flow**:
1. HTTP upgrade at `/ws` (rate-limited per IP)
2. Session token resolved (from message or new guest created)
3. `Player` struct created with a buffered send channel
4. Read pump dispatches messages → `Hub` → `Room`; write pump drains send channel

**Room lifecycle**: `RoomWaiting` → `RoomPlaying` → `RoomFinished`

**Key client→server message types**: `move`, `new_game`, `resign`, `draw_offer`, `draw_accept`,
`draw_decline`, `undo_request`, `undo_accept`, `create_room`, `join_room`, `find_game`, `leave_room`

**Key server→client message types**: `state` (full game state after every action), `session`,
`room_created`, `match_found`, `rating_update`, `error`, `room_list`

All messages carry a `v` (version) field for protocol versioning.

**Rate limiting** (token-bucket, idle eviction after 10 min):
| Limiter | Rate | Burst |
|---|---|---|
| Per-IP connections | 5/s | 10 |
| Per-IP auth actions | 0.2/s | 5 |
| Per-player actions | 2/s | 10 |
| Per-player messages | 20/s | 40 |

**HTTP endpoints**:
- `GET /healthz` — liveness probe (always 200)
- `GET /readyz` — readiness probe (200 when DB pingable, 503 otherwise)
- `GET /api/rooms` — list active rooms (JSON)
- `GET /api/games` — list finished games
- `GET /api/players` — list players

## Configuration

Layered config (later layers override earlier):
1. Compiled-in defaults (port 8080, no DB)
2. `config.json` file (if present)
3. `DATABASE_URL` environment variable (legacy, sets DB URL)
4. `GOCHESS__*` environment variables (e.g. `GOCHESS__HTTP__PORT=9090`, `GOCHESS__DB__URL=postgres://...`)
5. CLI flags (`--port`, `--db.url`)

Config is implemented in `cmd/web/config.go` using `knadh/koanf/v2`.

## Frontend (web/)

Built with esbuild; type-checked with `tsc --noEmit`. Source in `web/src/`, output to `web/dist/`.

| File | Responsibility |
|---|---|
| `main.ts` | Entry point, DOM wiring, WebSocket message dispatch |
| `socket.ts` | `ChessSocket` class, auto-reconnect (500ms→8s exponential backoff), localStorage token |
| `board.ts` | DOM board rendering, click-to-move, legal move highlights, promotion dialog |
| `lobby.ts` | Room list rendering, room creation/joining, filtering by status |
| `clock.ts` | Clock display and client-side countdown |
| `history.ts` | Move history panel |
| `settings.ts` | Board theme (brown/green/blue/grey), sound toggle, auto-flip |
| `sounds.ts` | Move/capture sound effects |
| `types.ts` | TypeScript interfaces mirroring Go server message types |

## Architecture Notes

- **Immutable game state**: `chess.GameState` is never mutated; `ApplyMove` returns a new state. Enables safe concurrent AI computation and natural undo/redo.
- **Interface-driven persistence**: `store.Store` interface with PostgreSQL and in-memory implementations; swap with no server changes.
- **Message-driven server**: Clients send JSON over WebSocket; `Hub` dispatches to `Room` via channels. Protocol-versioned with `v` field.
- **Rate limiting**: Token-bucket per IP and per player with idle eviction; protects auth, connections, and game actions independently.
- **Layered configuration**: 12-factor-compatible; env vars override file; CLI flags are highest priority.
- **Health/readiness split**: `/healthz` is always up; `/readyz` gates on DB availability.
- **WebSocket auto-reconnect**: Client retries with exponential backoff; session token in localStorage survives page refresh.
- **In-memory store fallback**: No database required for single-session play; PostgreSQL is opt-in.

## Code Conventions

- Go 1.25.3; use `goimports` with local prefix `github.com/fulstaph/gochess`
- Linter config in `.golangci.yml` — run `task lint:go` before committing
  - SA1019 excluded (nhooyr.io/websocket deprecation notice)
- Frontend: TypeScript 5.8, built with esbuild, type-checked with `tsc --noEmit`
- Tests use standard `testing` package, no test frameworks

## Key Dependencies

| Package | Purpose |
|---|---|
| `charmbracelet/bubbletea` | TUI framework |
| `charmbracelet/lipgloss` | TUI styling |
| `nhooyr.io/websocket` | WebSocket server/client |
| `jackc/pgx/v5` | PostgreSQL driver with connection pooling |
| `knadh/koanf/v2` | Layered configuration |
| `golang.org/x/crypto` | bcrypt password hashing |
| `golang.org/x/time/rate` | Token-bucket rate limiting |

## Testing

- Run `task test` for all Go tests
- PostgreSQL integration tests in `store/postgres_test.go` are skipped unless `DATABASE_URL` is set
- Write tests following existing patterns: table-driven, `t.Helper()` for helpers, subtests with `t.Run()`
- 14 test files covering chess engine, server (hub/room/auth/ratelimit/matchmaking/rating/clock), store, config, and health endpoints
- CI runs both Go and frontend jobs in parallel (`.github/workflows/ci.yml`)
