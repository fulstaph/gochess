package chess

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// FormatSAN returns the Standard Algebraic Notation for a move.
// state is the position before the move is applied.
func FormatSAN(mv Move, state GameState) string {
	preLegal := GenerateLegalMoves(state)
	next := ApplyMove(state, mv)
	nextLegal := GenerateLegalMoves(next)

	if mv.isCastle {
		san := "O-O"
		if mv.toC == 2 {
			san = "O-O-O"
		}
		return san + checkSuffixFromLegal(next, nextLegal)
	}

	piece := state.board[mv.fromR][mv.fromC]
	pt := pieceType(piece)
	isCapture := mv.isEnPassant || state.board[mv.toR][mv.toC] != 0

	var b strings.Builder

	if pt == 'p' {
		if isCapture {
			b.WriteByte(byte('a' + mv.fromC))
			b.WriteByte('x')
		}
		b.WriteString(formatSquare(mv.toR, mv.toC))
		if mv.promotion != 0 {
			b.WriteByte('=')
			b.WriteRune(pieceType(mv.promotion) - 'a' + 'A')
		}
	} else {
		b.WriteRune(pt - 'a' + 'A')
		b.WriteString(sanDisambiguation(mv, state, preLegal, pt))
		if isCapture {
			b.WriteByte('x')
		}
		b.WriteString(formatSquare(mv.toR, mv.toC))
	}

	b.WriteString(checkSuffixFromLegal(next, nextLegal))
	return b.String()
}

// FormatPGN generates a PGN string for a game.
// moves is the ordered list of moves applied from initialState.
// headers is an optional map of additional PGN tag pairs.
// result is the game result string (e.g. "1-0", "0-1", "1/2-1/2", "*").
func FormatPGN(moves []Move, initialState GameState, headers map[string]string, result string) string {
	tagOrder := []string{"Event", "Site", "Date", "Round", "White", "Black", "Result"}
	defaults := map[string]string{
		"Event":  "?",
		"Site":   "?",
		"Date":   "????.??.??",
		"Round":  "?",
		"White":  "?",
		"Black":  "?",
		"Result": result,
	}
	if result == "" {
		defaults["Result"] = "*"
	}

	var b strings.Builder

	for _, tag := range tagOrder {
		val := defaults[tag]
		if v, ok := headers[tag]; ok {
			val = v
		}
		fmt.Fprintf(&b, "[%s %q]\n", tag, val)
	}
	for tag, val := range headers {
		isDefault := false
		for _, dt := range tagOrder {
			if dt == tag {
				isDefault = true
				break
			}
		}
		if !isDefault {
			fmt.Fprintf(&b, "[%s %q]\n", tag, val)
		}
	}

	b.WriteByte('\n')

	tokens := make([]string, 0, len(moves)*2)
	state := initialState
	for i, mv := range moves {
		if state.turn == White {
			tokens = append(tokens, fmt.Sprintf("%d.", state.fullmoveNum))
		} else if i == 0 {
			// Game starts with black to move (unusual FEN position).
			tokens = append(tokens, fmt.Sprintf("%d...", state.fullmoveNum))
		}
		tokens = append(tokens, FormatSAN(mv, state))
		state = ApplyMove(state, mv)
	}
	if result != "" {
		tokens = append(tokens, result)
	}

	// Word-wrap at ~80 chars.
	line := ""
	for _, tok := range tokens {
		if line == "" {
			line = tok
		} else if len(line)+1+len(tok) > 80 {
			b.WriteString(line)
			b.WriteByte('\n')
			line = tok
		} else {
			line += " " + tok
		}
	}
	if line != "" {
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return b.String()
}

// sanDisambiguation returns the file, rank, or both needed to uniquely
// identify the moving piece among pieces of the same type.
func sanDisambiguation(mv Move, state GameState, legalMoves []Move, pt rune) string {
	var ambiguous []Move
	for _, lm := range legalMoves {
		if lm.toR == mv.toR && lm.toC == mv.toC &&
			(lm.fromR != mv.fromR || lm.fromC != mv.fromC) {
			p := state.board[lm.fromR][lm.fromC]
			if pieceType(p) == pt {
				ambiguous = append(ambiguous, lm)
			}
		}
	}
	if len(ambiguous) == 0 {
		return ""
	}

	sameFile := false
	for _, lm := range ambiguous {
		if lm.fromC == mv.fromC {
			sameFile = true
			break
		}
	}
	if !sameFile {
		return string(byte('a' + mv.fromC))
	}

	sameRank := false
	for _, lm := range ambiguous {
		if lm.fromR == mv.fromR {
			sameRank = true
			break
		}
	}
	if !sameRank {
		return string(byte('8' - mv.fromR))
	}

	return string(byte('a'+mv.fromC)) + string(byte('8'-mv.fromR))
}

// checkSuffixFromLegal returns "+" for check, "#" for checkmate, or "".
// legalMoves must be the pre-computed legal moves for state.
func checkSuffixFromLegal(state GameState, legalMoves []Move) string {
	if IsInCheck(state, state.turn) {
		if len(legalMoves) == 0 {
			return "#"
		}
		return "+"
	}
	return ""
}

// ---- PGN Import ----

// ParseSAN parses a SAN move string (e.g. "Nf3", "exd5", "O-O", "e8=Q+")
// and returns the matching legal Move for the given state.
func ParseSAN(san string, state GameState) (Move, error) {
	// Strip annotations: +, #, !, ?
	cleaned := strings.TrimRight(san, "+#!?")
	if cleaned == "" {
		return Move{}, fmt.Errorf("empty SAN string")
	}

	legal := GenerateLegalMoves(state)

	// Castling.
	if cleaned == "O-O" || cleaned == "0-0" {
		for _, mv := range legal {
			if mv.isCastle && mv.toC == 6 {
				return mv, nil
			}
		}
		return Move{}, fmt.Errorf("illegal castling O-O")
	}
	if cleaned == "O-O-O" || cleaned == "0-0-0" {
		for _, mv := range legal {
			if mv.isCastle && mv.toC == 2 {
				return mv, nil
			}
		}
		return Move{}, fmt.Errorf("illegal castling O-O-O")
	}

	// Pawn moves: e4, exd5, e8=Q, dxe8=Q
	if cleaned[0] >= 'a' && cleaned[0] <= 'h' {
		return parseSANPawn(cleaned, state, legal)
	}

	// Piece moves: Nf3, Nxe5, Rad1, R1d2, Qh4
	return parseSANPiece(cleaned, state, legal)
}

func parseSANPawn(san string, state GameState, legal []Move) (Move, error) {
	s := san
	var promoRune rune

	// Check for promotion: =Q, =R, =B, =N
	if idx := strings.IndexByte(s, '='); idx >= 0 {
		if idx+1 >= len(s) {
			return Move{}, fmt.Errorf("invalid promotion in %q", san)
		}
		promoRune = unicode.ToLower(rune(s[idx+1]))
		s = s[:idx]
	}

	// Remove capture indicator.
	s = strings.ReplaceAll(s, "x", "")

	// Now s is: "e4", "de5" (file + square), or just square "e4".
	var fromFile = -1
	var toR, toC int
	var ok bool

	if len(s) == 3 {
		// Disambiguation file + destination: dxe5 → "de5"
		fromFile = int(s[0] - 'a')
		toR, toC, ok = parseSquare(s[1:3])
		if !ok {
			return Move{}, fmt.Errorf("invalid square in %q", san)
		}
	} else if len(s) == 2 {
		toR, toC, ok = parseSquare(s[0:2])
		if !ok {
			return Move{}, fmt.Errorf("invalid square in %q", san)
		}
	} else {
		return Move{}, fmt.Errorf("invalid pawn move %q", san)
	}

	for _, mv := range legal {
		if mv.toR != toR || mv.toC != toC {
			continue
		}
		p := state.board[mv.fromR][mv.fromC]
		if pieceType(p) != 'p' {
			continue
		}
		if fromFile >= 0 && mv.fromC != fromFile {
			continue
		}
		if promoRune != 0 {
			if mv.promotion == 0 || pieceType(mv.promotion) != promoRune {
				continue
			}
		} else if mv.promotion != 0 {
			// Default to queen promotion if not specified.
			if pieceType(mv.promotion) != 'q' {
				continue
			}
		}
		return mv, nil
	}
	return Move{}, fmt.Errorf("no legal pawn move matching %q", san)
}

func parseSANPiece(san string, state GameState, legal []Move) (Move, error) {
	if len(san) < 2 {
		return Move{}, fmt.Errorf("invalid piece move %q", san)
	}

	pt := unicode.ToLower(rune(san[0])) // N, B, R, Q, K → n, b, r, q, k
	s := san[1:]

	// Remove capture indicator.
	s = strings.ReplaceAll(s, "x", "")

	// Destination is always the last two characters.
	if len(s) < 2 {
		return Move{}, fmt.Errorf("invalid piece move %q", san)
	}
	destStr := s[len(s)-2:]
	toR, toC, ok := parseSquare(destStr)
	if !ok {
		return Move{}, fmt.Errorf("invalid destination square in %q", san)
	}

	// Disambiguation: everything before the destination square.
	disambig := s[:len(s)-2]
	var disambigFile = -1
	var disambigRank = -1

	for _, ch := range disambig {
		if ch >= 'a' && ch <= 'h' {
			disambigFile = int(ch - 'a')
		} else if ch >= '1' && ch <= '8' {
			disambigRank = 8 - int(ch-'0')
		}
	}

	for _, mv := range legal {
		if mv.toR != toR || mv.toC != toC {
			continue
		}
		p := state.board[mv.fromR][mv.fromC]
		if pieceType(p) != pt {
			continue
		}
		if disambigFile >= 0 && mv.fromC != disambigFile {
			continue
		}
		if disambigRank >= 0 && mv.fromR != disambigRank {
			continue
		}
		return mv, nil
	}
	return Move{}, fmt.Errorf("no legal piece move matching %q", san)
}

var tagPairRegex = regexp.MustCompile(`\[(\w+)\s+"([^"]*)"\]`)

// ParsePGN parses a PGN string and returns the moves, initial state, and tag pairs.
func ParsePGN(pgn string) ([]Move, GameState, map[string]string, error) {
	headers := make(map[string]string)
	lines := strings.Split(pgn, "\n")

	// Parse tag pairs and find where move text begins.
	moveTextStart := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			moveTextStart = i + 1
			continue
		}
		if matches := tagPairRegex.FindStringSubmatch(trimmed); matches != nil {
			headers[matches[1]] = matches[2]
			moveTextStart = i + 1
		} else if trimmed[0] != '[' {
			// First non-tag, non-empty line is move text.
			moveTextStart = i
			break
		}
	}

	// Determine initial state.
	var state GameState
	if fen, ok := headers["FEN"]; ok {
		var err error
		state, err = ParseFEN(fen)
		if err != nil {
			return nil, GameState{}, nil, fmt.Errorf("invalid FEN tag: %w", err)
		}
	} else {
		state = InitialState()
	}

	// Collect move text.
	moveText := strings.Join(lines[moveTextStart:], " ")

	// Strip comments { ... } and variations ( ... ).
	moveText = stripBraces(moveText)
	moveText = stripParens(moveText)

	// Tokenize and parse moves.
	tokens := strings.Fields(moveText)
	resultTokens := map[string]bool{"1-0": true, "0-1": true, "1/2-1/2": true, "*": true}

	var moves []Move
	current := state
	for _, tok := range tokens {
		// Skip move numbers: "1.", "1...", "23."
		if isMoveNumber(tok) {
			continue
		}
		// Skip result tokens.
		if resultTokens[tok] {
			continue
		}

		mv, err := ParseSAN(tok, current)
		if err != nil {
			return nil, GameState{}, nil, fmt.Errorf("move %d (%q): %w", len(moves)+1, tok, err)
		}
		moves = append(moves, mv)
		current = ApplyMove(current, mv)
	}

	return moves, state, headers, nil
}

func stripBraces(s string) string {
	var b strings.Builder
	depth := 0
	for _, ch := range s {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			if depth > 0 {
				depth--
			}
		} else if depth == 0 {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func stripParens(s string) string {
	var b strings.Builder
	depth := 0
	for _, ch := range s {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			if depth > 0 {
				depth--
			}
		} else if depth == 0 {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func isMoveNumber(tok string) bool {
	// Matches "1.", "23.", "1...", etc.
	s := strings.TrimRight(tok, ".")
	if s == tok {
		return false // no trailing dot
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
