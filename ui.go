package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fulstaph/gochess/chess"
	"github.com/mattn/go-runewidth"
)

var (
	colorLightSquare = lipgloss.Color("#EEEED2")
	colorDarkSquare  = lipgloss.Color("#769656")
	colorHighlight   = lipgloss.Color("#C4D26C")
	colorWhitePiece  = lipgloss.Color("#FAFAFA")
	colorBlackPiece  = lipgloss.Color("#1E1E1E")
	colorEmptyPiece  = lipgloss.Color("#5B5B5B")
	colorTextMuted   = lipgloss.Color("#6C6C6C")
	colorTextAccent  = lipgloss.Color("#1D3557")
	colorTitleBg     = lipgloss.Color("#1D3557")
	colorTitleFg     = lipgloss.Color("#F1FAEE")
	colorPromptBg    = lipgloss.Color("#F1F5F9")
	colorPromptFg    = lipgloss.Color("#0F172A")
)

var (
	titleStyle      = lipgloss.NewStyle().Foreground(colorTitleFg).Background(colorTitleBg).Padding(0, 1).Bold(true)
	metaStyle       = lipgloss.NewStyle().Foreground(colorTextMuted)
	labelStyle      = lipgloss.NewStyle().Foreground(colorTextMuted)
	panelStyle      = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorTextMuted).Padding(0, 1)
	promptBox       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorTextMuted).Background(colorPromptBg).Foreground(colorPromptFg).Padding(0, 1)
	statusStyle     = lipgloss.NewStyle().Foreground(colorTextAccent).Bold(true)
	warnStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#B00020")).Bold(true)
	promptTextStyle = lipgloss.NewStyle().Foreground(colorPromptFg)
)

const (
	squareWidth  = 5
	squareHeight = 3
)

func renderBoard(state chess.GameState, lastMove *chess.Move, status string, moveHistory []string, prompt string, width int, perspective int, useUnicode bool, pieceScale int) string {
	header := buildHeader(state, status, perspective)
	boardLines, boardHeight := buildBoardLines(state, lastMove, perspective, useUnicode, pieceScale)
	boardBlock := panelStyle.Render(strings.Join(boardLines, "\n"))

	promptLine := promptBox.Render(prompt)
	controls := metaStyle.Render("Commands: help, resign, quit, exit, ai [depth] | Flags: -ai=white|black|both|none, -depth=1..4, -pieces=unicode|ascii, -bigpieces=off|2x2|3x3")
	left := lipgloss.JoinVertical(lipgloss.Left, header, boardBlock, promptLine, controls)

	sidebar := panelStyle.Render(buildMoveList(moveHistory, boardHeight))
	helpPanel := panelStyle.Render(buildHelpPanel())
	right := lipgloss.JoinVertical(lipgloss.Left, sidebar, helpPanel)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	if width > 0 && lipgloss.Width(joined) > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(joined)
	}
	return joined
}

func buildHeader(state chess.GameState, status string, perspective int) string {
	title := titleStyle.Render("Gochess")
	view := chess.ColorName(perspective)
	meta := metaStyle.Render(fmt.Sprintf("Move %d | %s to move | Halfmove %d | View: %s", state.FullmoveNumber(), chess.ColorName(state.Turn()), state.HalfmoveClock(), view))
	lines := []string{title, meta}
	if status != "" {
		style := statusStyle
		if strings.Contains(status, "Error:") {
			style = warnStyle
		}
		lines = append(lines, style.Render(status))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func buildBoardLines(state chess.GameState, lastMove *chess.Move, perspective int, useUnicode bool, pieceScale int) ([]string, int) {
	board := state.Board()
	lines := make([]string, 0, 10)

	filesLine := "   "
	for c := 0; c < 8; c++ {
		file := fileLabel(c, perspective)
		filesLine += padSquare(string(file))
	}
	lines = append(lines, labelStyle.Render(filesLine))

	var fromR, fromC, toR, toC int
	hasLast := false
	if lastMove != nil {
		fromR, fromC = lastMove.From()
		toR, toC = lastMove.To()
		hasLast = true
	}

	mid := squareHeight / 2
	for dr := 0; dr < 8; dr++ {
		rank := rankLabel(dr, perspective)
		rowLines := make([]string, squareHeight)
		leftLabel := labelStyle.Render(fmt.Sprintf(" %d ", rank))
		blankLabel := labelStyle.Render("   ")
		for i := 0; i < squareHeight; i++ {
			if i == mid {
				rowLines[i] = leftLabel
			} else {
				rowLines[i] = blankLabel
			}
		}

		for dc := 0; dc < 8; dc++ {
			br, bc := boardCoords(dr, dc, perspective)
			highlight := hasLast && ((br == fromR && bc == fromC) || (br == toR && bc == toC))
			cellLines := renderSquare(board[br][bc], (br+bc)%2 == 0, highlight, useUnicode, pieceScale)
			for i := 0; i < squareHeight; i++ {
				rowLines[i] += cellLines[i]
			}
		}

		for i := 0; i < squareHeight; i++ {
			if i == mid {
				rowLines[i] += leftLabel
			} else {
				rowLines[i] += blankLabel
			}
		}
		lines = append(lines, rowLines...)
	}

	lines = append(lines, labelStyle.Render(filesLine))
	return lines, len(lines)
}

func renderSquare(piece rune, light bool, highlight bool, useUnicode bool, pieceScale int) []string {
	bg := colorDarkSquare
	if light {
		bg = colorLightSquare
	}
	if highlight {
		bg = colorHighlight
	}

	fg := colorEmptyPiece
	style := lipgloss.NewStyle().Background(bg).Foreground(fg)
	if piece != 0 {
		if piece >= 'A' && piece <= 'Z' {
			fg = colorWhitePiece
			style = style.Foreground(fg).Bold(true)
		} else {
			fg = colorBlackPiece
			style = style.Foreground(fg)
		}
	}
	sprite, spriteHeight := pieceSprite(piece, useUnicode, pieceScale)
	if spriteHeight < 1 {
		spriteHeight = 1
	}
	if spriteHeight > squareHeight {
		spriteHeight = squareHeight
	}
	startRow := (squareHeight - spriteHeight) / 2

	lines := make([]string, 0, squareHeight)
	for i := 0; i < squareHeight; i++ {
		if i >= startRow && i < startRow+spriteHeight {
			lines = append(lines, style.Render(padSquare(sprite[i-startRow])))
		} else {
			lines = append(lines, style.Render(strings.Repeat(" ", squareWidth)))
		}
	}
	return lines
}

func buildMoveList(history []string, height int) string {
	lines := make([]string, 0, height)
	lines = append(lines, titleStyle.Render("Moves"))
	lines = append(lines, "")

	available := height - len(lines)
	if available < 1 {
		return strings.Join(lines, "\n")
	}

	start := 0
	if len(history) > available {
		if available == 1 {
			lines = append(lines, "...")
			available = 0
		} else {
			lines = append(lines, "...")
			available--
			start = len(history) - available
		}
	}

	for i := start; i < len(history) && available > 0; i++ {
		lines = append(lines, history[i])
		available--
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func buildHelpPanel() string {
	lines := []string{
		titleStyle.Render("Help"),
		"",
		"Moves: e2e4, e7e8q",
		"Castle: O-O / O-O-O",
		"AI: ai 2  (depth 1-4)",
		"View: v to flip",
		"Pieces: p to toggle",
		"Size: b to cycle",
		"Style: -pieces=ascii",
		"Quit: q / Ctrl+C",
		"Resign: resign",
	}
	return strings.Join(lines, "\n")
}

func pieceGlyph(piece rune, useUnicode bool) string {
	if piece == 0 {
		return "."
	}
	if !useUnicode {
		return string(piece)
	}
	switch piece {
	case 'K':
		return "♔"
	case 'Q':
		return "♕"
	case 'R':
		return "♖"
	case 'B':
		return "♗"
	case 'N':
		return "♘"
	case 'P':
		return "♙"
	case 'k':
		return "♚"
	case 'q':
		return "♛"
	case 'r':
		return "♜"
	case 'b':
		return "♝"
	case 'n':
		return "♞"
	case 'p':
		return "♟"
	default:
		return string(piece)
	}
}

func pieceSprite(piece rune, useUnicode bool, pieceScale int) ([]string, int) {
	if piece == 0 || pieceScale <= 1 {
		return []string{pieceGlyph(piece, useUnicode)}, 1
	}

	glyph := pieceGlyph(piece, useUnicode)
	fill := "#"
	if useUnicode {
		fill = "█"
	}
	top := strings.Repeat(fill, 3)
	mid := fill + glyph + fill
	if pieceScale == 2 {
		return []string{top, mid}, 2
	}
	return []string{top, mid, top}, 3
}

func padSquare(text string) string {
	if text == "" {
		return strings.Repeat(" ", squareWidth)
	}
	width := runewidth.StringWidth(text)
	if width >= squareWidth {
		return runewidth.Truncate(text, squareWidth, "")
	}
	left := (squareWidth - width) / 2
	right := squareWidth - width - left
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}

func boardCoords(displayR, displayC int, perspective int) (int, int) {
	if perspective == chess.Black {
		return 7 - displayR, 7 - displayC
	}
	return displayR, displayC
}

func rankLabel(displayR int, perspective int) int {
	if perspective == chess.Black {
		return 1 + displayR
	}
	return 8 - displayR
}

func fileLabel(displayC int, perspective int) byte {
	if perspective == chess.Black {
		return byte('h' - displayC)
	}
	return byte('a' + displayC)
}
