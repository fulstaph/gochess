package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type aiSide int

const (
	aiNone aiSide = iota
	aiWhite
	aiBlack
	aiBoth
)

func main() {
	aiMode, aiDepth, useUnicode, pieceScale := parseFlags()
	model := newModel(aiMode, aiDepth, useUnicode, pieceScale)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags() (aiSide, int, bool, int) {
	aiOpt := flag.String("ai", "black", "AI side: white, black, both, none")
	depth := flag.Int("depth", 2, "AI search depth (1-4)")
	pieces := flag.String("pieces", "unicode", "Piece style: unicode, ascii")
	bigpieces := flag.String("bigpieces", "2x2", "Piece size: off, 2x2, 3x3")
	flag.Parse()

	aiMode, ok := parseAISide(*aiOpt)
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid -ai value: %s (use white, black, both, none)\n", *aiOpt)
		os.Exit(2)
	}
	if *depth < 1 || *depth > 4 {
		fmt.Fprintln(os.Stderr, "AI depth must be between 1 and 4.")
		os.Exit(2)
	}
	useUnicode, ok := parsePieceStyle(*pieces)
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid -pieces value: %s (use unicode, ascii)\n", *pieces)
		os.Exit(2)
	}
	pieceScale, ok := parsePieceScale(*bigpieces)
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid -bigpieces value: %s (use off, 2x2, 3x3)\n", *bigpieces)
		os.Exit(2)
	}
	return aiMode, *depth, useUnicode, pieceScale
}

func parseAISide(s string) (aiSide, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none":
		return aiNone, true
	case "white":
		return aiWhite, true
	case "black":
		return aiBlack, true
	case "both":
		return aiBoth, true
	default:
		return aiNone, false
	}
}

func parsePieceStyle(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "unicode":
		return true, true
	case "ascii":
		return false, true
	default:
		return false, false
	}
}

func parsePieceScale(s string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "1x1", "1":
		return 1, true
	case "2x2", "2":
		return 2, true
	case "3x3", "3":
		return 3, true
	default:
		return 0, false
	}
}
