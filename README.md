# Gochess

A chess engine written in Go with two frontends: a terminal UI (Bubble Tea) and a browser-based web UI (TypeScript). The core rules live in a reusable `chess` package.

## Features

- Legal move generation with full check validation
- Castling (king/queen side), en passant, and pawn promotion
- Check, checkmate, and stalemate detection
- Draw rules: fifty/seventy-five-move rule, threefold/fivefold repetition
- Minimax AI with alpha-beta pruning, quiescence search, and piece-square tables
- Terminal UI (Bubble Tea) with ANSI-colored board and move list sidebar
- Web UI (TypeScript) with click-to-move, legal move highlights, and promotion dialog
- WebSocket backend — designed for future PvP multiplayer

## Quick Start

Requirements: Go 1.22+, Node.js (for the web UI only)

If you have [Task](https://taskfile.dev) installed, most commands are one-liners:

```bash
task tui          # run terminal UI
task web          # build frontend + run web server
task test         # run all tests
task build        # build both binaries
task --list       # see all available tasks
```

Otherwise use the commands below directly.

### Terminal UI

```bash
go run .
```

### Web UI

Build the frontend (one-time):

```bash
cd web && npm install && npm run build
```

Run the server:

```bash
go run ./cmd/web
```

Then open `http://localhost:8080` in your browser.

### Tests

```bash
go test ./...
```

## Playing via Terminal

The TUI renders the board and accepts commands from the prompt at the bottom.

**Commands:**

- `help` — show input formats and commands
- `resign` — resign the game
- `quit` / `exit` — quit immediately
- `ai [depth]` — let the AI pick a move (depth 1–4, default 2)
- `q` or `Ctrl+C` — quit immediately
- `Up/Down` — cycle command history
- `v` — flip board orientation
- `p` — toggle Unicode/ASCII pieces
- `b` — cycle piece size

**Flags:**

```bash
go run . -ai=black -depth=2 -pieces=unicode -bigpieces=2x2
```

| Flag         | Default   | Description                               |
| ------------ | --------- | ----------------------------------------- |
| `-ai`        | `black`   | AI side: `white`, `black`, `both`, `none` |
| `-depth`     | `2`       | AI search depth (1–4)                     |
| `-pieces`    | `unicode` | Piece style: `unicode`, `ascii`           |
| `-bigpieces` | `2x2`     | Piece size: `off`, `2x2`, `3x3`           |

## Playing via Browser

The web frontend connects to the Go server via WebSocket. All game logic runs on the server; the browser only handles rendering and user input.

**How to play:**

1. Click a piece to select it — legal destination squares are highlighted
2. Click a highlighted square to move
3. For pawn promotion, a dialog appears to choose the piece
4. Use the toolbar to flip the board, start a new game, or resign

**Flags:**

```bash
go run ./cmd/web -port=8080 -ai=black -depth=2
```

| Flag     | Default | Description                               |
| -------- | ------- | ----------------------------------------- |
| `-port`  | `8080`  | HTTP port                                 |
| `-ai`    | `black` | AI side: `white`, `black`, `both`, `none` |
| `-depth` | `2`     | AI search depth (1–4)                     |

**Frontend development (watch mode):**

```bash
cd web && npm run watch
```

## Move Input Formats

Supported formats (terminal and WebSocket API):

- `e2e4`, `e2 e4`, `e2-e4`

Promotion:

- `e7e8q` or `e7e8=Q` (case-insensitive; `q`, `r`, `b`, `n`)
- Defaults to queen if no piece specified

Castling:

- `O-O` or `O-O-O` (also `0-0` / `0-0-0`)

## WebSocket Protocol

Connect to `ws://localhost:8080/ws`. All messages are JSON with a `type` field.

**Client → Server:**

| `type`     | Fields                     | Description      |
| ---------- | -------------------------- | ---------------- |
| `move`     | `from`, `to`, `promotion?` | Make a move      |
| `new_game` | `aiMode`, `aiDepth`        | Start a new game |
| `resign`   | —                          | Resign the game  |

**Server → Client:**

| `type`     | Description                                        |
| ---------- | -------------------------------------------------- |
| `state`    | Full game state (board, turn, legal moves, ...)    |
| `thinking` | AI is computing a move                             |
| `error`    | Invalid move or other error                        |

The `state` message is sent on every state change (after each move, new game, or resign).

## Package Layout

```text
chess/          Core engine (rules, move gen, AI) — importable as a library
  state.go      GameState, Move, initial position
  movegen.go    Legal and pseudo-legal move generation
  rules.go      Move application, check/attack detection
  parse.go      Input parsing and legal move matching
  ai.go         Minimax with alpha-beta pruning and quiescence search
  position.go   Position key and draw rule helpers
  helpers.go    Piece type utilities and color names
  format.go     Move formatting (coordinate notation, castling)
  chess_test.go Engine unit tests

server/         HTTP/WebSocket server
  server.go     GameSession: state management, move application, game-over logic
  handlers.go   WebSocket upgrade, message loop, async AI turns
  types.go      JSON message structs
  server_test.go Server unit tests

cmd/web/        Web server entry point
  main.go       Flag parsing, route registration, static file serving

web/            TypeScript frontend
  src/
    main.ts     App entry point, UI wiring
    socket.ts   WebSocket client with auto-reconnect
    board.ts    DOM board rendering, click-to-move, promotion dialog
    types.ts    TypeScript interfaces matching the WebSocket protocol
  dist/
    index.html  Page markup
    style.css   Dark-themed styles

main.go         Terminal UI entry point
tui.go          Bubble Tea model, input handling, AI turn management
ui.go           Board rendering, move list sidebar
game_helpers.go Move history, repetition tracking, game-over detection
```

## Engine Notes

- `ApplyMove` is immutable — it returns a new `GameState` without mutating the original.
- Check is enforced by generating pseudo-legal moves and filtering out those that leave the king in check.
- Castling requires: no pieces between king and rook, king not in check, king not passing through an attacked square.
- En passant is available only on the move immediately following a qualifying double-pawn advance.
- Game ends when there are zero legal moves (checkmate if in check, stalemate if not), or by the 75-move rule / fivefold repetition.

## Limitations

- No PGN/FEN import/export
- No clocks or time controls
- No undo/redo
- Single game session per server process (no lobbies or room management yet)

## Roadmap Ideas

- PGN/FEN support
- PvP multiplayer (the WebSocket architecture is already ready — add room management)
- Time controls
- Stronger AI (iterative deepening, transposition table, opening book)
- Save/load games
