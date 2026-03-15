# Gochess

A small terminal chess game written in Go. It includes full move validation (including check rules), draw rules, and a Bubble Tea TUI. The core rules live in a reusable `chess` package so you can embed the engine elsewhere.

## Features

- Legal move generation with check validation
- Castling (king/queen side), en passant, and promotion
- Check, checkmate, and stalemate detection
- Draw rules: fifty-move and threefold repetition
- Bubble Tea TUI with human-friendly input formats
- ANSI-colored board with last-move highlights and move list sidebar

## Quick Start

Requirements:
- Go 1.25+ (see `go.mod`)

Run the game:

```bash
go run .
```

Run tests:

```bash
go test ./...
```

## How To Play

The TUI renders the board and accepts commands from the prompt at the bottom.

Commands:
- `help` : show input formats and commands
- `resign` : resign the game
- `quit` / `exit` : quit immediately
- `ai [depth]` : let the computer pick a move (depth 1-4, default 2)
- `q` or `Ctrl+C` : quit immediately
- `Up/Down` : cycle recent command history
- `v` : flip board orientation
- `p` : toggle Unicode/ASCII pieces
- `b` : cycle piece size

### AI Mode (Executable Flags)

You can start the game with the AI controlling one or both sides:

```bash
go run . -ai=black -depth=2
```

Options:
- `-ai=white|black|both|none` (default `black`)
- `-depth=1..4` (default `2`)
- `-pieces=unicode|ascii` (default `unicode`)
- `-bigpieces=off|2x2|3x3` (default `2x2`)

### Move Input Formats

Supported formats:
- `e2e4`
- `e2 e4`
- `e2-e4`

Promotion:
- `e7e8q` or `e7e8=Q` (case-insensitive, accepts `q`, `r`, `b`, `n`)
- If no promotion piece is specified, it defaults to a queen.

Castling:
- `O-O` or `O-O-O`
- `0-0` or `0-0-0` also works

### Board Display

- Uppercase pieces are White: `K Q R B N P`
- Lowercase pieces are Black: `k q r b n p`
- `.` denotes an empty square
- Unicode chess pieces are used by default (`-pieces=ascii` to use letters)
- Piece size defaults to `2x2` (`-bigpieces=off` for normal size)
- The last move squares are highlighted
- A move list sidebar shows the current game in coordinate notation
- A help panel and input history make common commands easier to discover
- The board orients to the human player's side when AI controls the other

## Package Layout

- `main.go`
  - Program entry and flag parsing
- `tui.go`
  - Bubble Tea model, input handling, and AI turns
- `ui.go`
  - Board rendering and move list sidebar
- `game_helpers.go`
  - Shared helpers for move history, repetition tracking, and game-over checks
- `chess/state.go`
  - `GameState`, `Move`, and initial state setup
- `chess/movegen.go`
  - Pseudo-legal and legal move generation
- `chess/rules.go`
  - Move application and check/attack detection
- `chess/parse.go`
  - Input parsing and legal move matching
- `chess/helpers.go`
  - Utility helpers (piece types, bounds, color names)
- `chess/position.go`
  - Position key and draw rule helpers
- `chess/ai.go`
  - Simple AI search and evaluation
- `chess/format.go`
  - Move formatting helpers
- `chess/chess_test.go`
  - Tests for parsing, rules, and checkmate logic

## Engine Notes

- The engine enforces check rules; illegal moves that leave your king in check are rejected.
- Castling requires empty squares between king and rook and no traversal through check.
- En passant is available immediately after a qualifying pawn double-advance.
- Game ends are detected by checking for zero legal moves:
  - In check: checkmate
  - Not in check: stalemate
- Draws are declared automatically on fifty-move or threefold repetition.

## Limitations

This is a compact, terminal-focused chess engine. It does not currently include:
- PGN/FEN import/export
- Clocks or time controls
- Advanced AI evaluation (only material + search)
- Undo/redo or move takebacks

## Development Tips

- The `chess` package is designed to be embeddable; you can build alternate UIs on top of it.
- If you add UI code, keep game rules and state mutations inside `chess` to avoid rule drift.

## Roadmap Ideas

- PGN/FEN support
- Stronger AI (piece-square tables, time controls, deeper search)
- Save/load games
- Alternative UIs (web or graphical)
