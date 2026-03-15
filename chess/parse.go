package chess

import (
	"fmt"
	"strings"
)

func ParseMove(input string, state GameState) (Move, error) {
	raw := strings.ToLower(strings.TrimSpace(input))

	if side, ok := parseCastleInput(raw); ok {
		fromR, fromC := 0, 4
		toR, toC := 0, 6
		if state.turn == White {
			fromR = 7
			toR = 7
		}
		if side == "queen" {
			toC = 2
		}
		return matchMove(state, fromR, fromC, toR, toC, 0, false)
	}

	clean := strings.ReplaceAll(raw, " ", "")
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, "=", "")
	if len(clean) < 4 {
		return Move{}, fmt.Errorf("use formats like e2e4 or e7e8q")
	}

	fromR, fromC, ok := parseSquare(clean[0:2])
	if !ok {
		return Move{}, fmt.Errorf("invalid from-square")
	}
	toR, toC, ok := parseSquare(clean[2:4])
	if !ok {
		return Move{}, fmt.Errorf("invalid to-square")
	}

	var promo rune
	promoProvided := false
	if len(clean) >= 5 {
		promoProvided = true
		promo = rune(clean[4])
	}

	return matchMove(state, fromR, fromC, toR, toC, promo, promoProvided)
}

func matchMove(state GameState, fromR, fromC, toR, toC int, promo rune, promoProvided bool) (Move, error) {
	legal := GenerateLegalMoves(state)
	matches := make([]Move, 0, 4)
	for _, mv := range legal {
		if mv.fromR == fromR && mv.fromC == fromC && mv.toR == toR && mv.toC == toC {
			matches = append(matches, mv)
		}
	}

	if len(matches) == 0 {
		return Move{}, fmt.Errorf("no legal move for that input")
	}

	if promoProvided {
		desired := pieceType(promo)
		for _, mv := range matches {
			if mv.promotion != 0 && pieceType(mv.promotion) == desired {
				return mv, nil
			}
		}
		return Move{}, fmt.Errorf("promotion piece must be q, r, b, or n")
	}

	for _, mv := range matches {
		if mv.promotion != 0 && pieceType(mv.promotion) == 'q' {
			return mv, nil
		}
	}
	return matches[0], nil
}

func parseCastleInput(s string) (string, bool) {
	t := strings.ReplaceAll(s, "0", "o")
	t = strings.ReplaceAll(t, "-", "")
	t = strings.ReplaceAll(t, " ", "")
	if t == "oo" {
		return "king", true
	}
	if t == "ooo" {
		return "queen", true
	}
	return "", false
}

func parseSquare(s string) (int, int, bool) {
	if len(s) != 2 {
		return 0, 0, false
	}
	file := s[0]
	rank := s[1]
	if file < 'a' || file > 'h' {
		return 0, 0, false
	}
	if rank < '1' || rank > '8' {
		return 0, 0, false
	}
	c := int(file - 'a')
	r := 8 - int(rank-'0')
	return r, c, true
}
