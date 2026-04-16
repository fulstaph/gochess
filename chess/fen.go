package chess

import (
	"fmt"
	"strconv"
	"strings"
)

const validFENPieces = "PpNnBbRrQqKk"

// ToFEN serializes a GameState to a FEN string.
func ToFEN(state GameState) string {
	var b strings.Builder

	for r := 0; r < 8; r++ {
		if r > 0 {
			b.WriteByte('/')
		}
		empty := 0
		for c := 0; c < 8; c++ {
			p := state.board[r][c]
			if p == 0 {
				empty++
			} else {
				if empty > 0 {
					b.WriteByte(byte('0' + empty))
					empty = 0
				}
				b.WriteRune(p)
			}
		}
		if empty > 0 {
			b.WriteByte(byte('0' + empty))
		}
	}

	b.WriteByte(' ')
	if state.turn == White {
		b.WriteByte('w')
	} else {
		b.WriteByte('b')
	}

	b.WriteByte(' ')
	rights := castlingRights(state)
	if rights == "" {
		b.WriteByte('-')
	} else {
		b.WriteString(rights)
	}

	b.WriteByte(' ')
	if state.enPassantR == -1 || state.enPassantC == -1 {
		b.WriteByte('-')
	} else {
		b.WriteString(formatSquare(state.enPassantR, state.enPassantC))
	}

	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(state.halfmove))

	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(state.fullmoveNum))

	return b.String()
}

// ParseFEN constructs a GameState from a FEN string.
func ParseFEN(fen string) (GameState, error) {
	parts := strings.Fields(fen)
	if len(parts) != 6 {
		return GameState{}, fmt.Errorf("FEN must have 6 fields, got %d", len(parts))
	}

	var state GameState
	state.enPassantR = -1
	state.enPassantC = -1

	ranks := strings.Split(parts[0], "/")
	if len(ranks) != 8 {
		return GameState{}, fmt.Errorf("FEN board must have 8 ranks, got %d", len(ranks))
	}
	for r, rank := range ranks {
		c := 0
		for _, ch := range rank {
			if ch >= '1' && ch <= '8' {
				c += int(ch - '0')
			} else {
				if c >= 8 {
					return GameState{}, fmt.Errorf("FEN rank %d overflows", r+1)
				}
				if !strings.ContainsRune(validFENPieces, ch) {
					return GameState{}, fmt.Errorf("invalid piece %c in FEN rank %d", ch, r+1)
				}
				state.board[r][c] = ch
				c++
			}
		}
		if c != 8 {
			return GameState{}, fmt.Errorf("FEN rank %d has %d squares, expected 8", r+1, c)
		}
	}

	switch parts[1] {
	case "w":
		state.turn = White
	case "b":
		state.turn = Black
	default:
		return GameState{}, fmt.Errorf("invalid active color %q", parts[1])
	}

	if parts[2] != "-" {
		for _, ch := range parts[2] {
			switch ch {
			case 'K':
				state.castling[0] = true
			case 'Q':
				state.castling[1] = true
			case 'k':
				state.castling[2] = true
			case 'q':
				state.castling[3] = true
			default:
				return GameState{}, fmt.Errorf("invalid castling character %c", ch)
			}
		}
	}

	if parts[3] != "-" {
		r, c, ok := parseSquare(parts[3])
		if !ok {
			return GameState{}, fmt.Errorf("invalid en passant square %q", parts[3])
		}
		state.enPassantR = r
		state.enPassantC = c
	}

	halfmove, err := strconv.Atoi(parts[4])
	if err != nil {
		return GameState{}, fmt.Errorf("invalid halfmove clock: %v", err)
	}
	state.halfmove = halfmove

	fullmove, err := strconv.Atoi(parts[5])
	if err != nil {
		return GameState{}, fmt.Errorf("invalid fullmove number: %v", err)
	}
	state.fullmoveNum = fullmove

	return state, nil
}
