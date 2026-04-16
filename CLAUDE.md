# GoChess

A chess engine and multiplayer platform written in Go, with a terminal UI (Bubble Tea) and a web UI (TypeScript + WebSocket).

## Project Structure

```
chess/       Core engine: move generation, rules, AI (minimax + alpha-beta), FEN, PGN
game/        Shared game-over detection and move recording logic
server/      WebSocket multiplayer: hub, rooms, matchmaking, rating, auth, clock
store/       Persistence layer: Store interface, PostgreSQL impl, migrations
cmd/web/     Web server entry point (serves static files + WebSocket)
web/         TypeScript frontend (esbuild build)
main.go      TUI entry point (Bubble Tea)
tui.go       TUI model, view, and update logic
```

## Commands

### Build & Run

```sh
task tui                  # Run terminal UI (AI plays black, depth 2)
task web                  # Build frontend + run web server on :8080
task dev                  # Hot-reload Go server (air) + watch TypeScript
task build                # Build both binaries (./gochess, ./gochess-web)
```

### Test & Lint

```sh
task test                 # go test ./...
task test:verbose         # go test -v ./...
task test:cover           # Tests with coverage report
task lint                 # Lint Go + TypeScript
task lint:go              # golangci-lint run ./...
task lint:ts              # npx tsc --noEmit (in web/)
task ci                   # Full CI pipeline locally (build, lint, test, frontend)
```

### Database (optional, for multiplayer persistence)

```sh
docker compose up -d      # Start PostgreSQL
# Set DATABASE_URL=postgres://gochess:gochess@localhost:5432/gochess
task dev:run db=postgres://gochess:gochess@localhost:5432/gochess
```

## Architecture Notes

- **Immutable game state**: `chess.GameState` is never mutated; `ApplyMove` returns a new state. This enables safe concurrent AI computation and natural undo/redo support.
- **Interface-driven persistence**: `store.Store` interface with PostgreSQL and in-memory implementations.
- **Message-driven server**: Clients send JSON over WebSocket, hub dispatches to rooms. Key message types: `move`, `new_game`, `resign`, `draw_offer`, `create_room`, `join_room`, `find_game`.

## Code Conventions

- Go 1.25, use `goimports` with local prefix `github.com/fulstaph/gochess`
- Linter config in `.golangci.yml` — run `task lint:go` before committing
- Frontend: TypeScript, built with esbuild, type-checked with `tsc --noEmit`
- Tests use standard `testing` package, no test frameworks
- CI runs on GitHub Actions (`.github/workflows/ci.yml`): build, lint, test for both Go and frontend

## Testing

- Run `task test` for all Go tests
- PostgreSQL integration tests in `store/postgres_test.go` are skipped unless `DATABASE_URL` is set
- When writing tests, follow existing patterns: table-driven tests, `t.Helper()` for helpers, subtests with `t.Run()`
