package main

import (
	"fmt"
	"maps"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fulstaph/gochess/chess"
	gm "github.com/fulstaph/gochess/game"
)

const (
	moveSourceHuman = "You"
	moveSourceAI    = "AI"
)

type undoSnapshot struct {
	state    chess.GameState
	moveHist []string
	rawMoves []chess.Move
	reps     map[string]int
	lastMove chess.Move
	hasLast  bool
}

type model struct {
	state          chess.GameState
	aiMode         aiSide
	aiDepth        int
	lastMove       chess.Move
	hasLastMove    bool
	lastMoveSource string
	repetitions    map[string]int
	moveHistory    []string
	rawMoves       []chess.Move // for PGN export
	input          textinput.Model
	errMsg         string
	notice         string
	result         string
	gameOver       bool
	width          int
	height         int
	history        []string
	historyIndex   int
	historyStash   string
	perspective    int
	useUnicode     bool
	pieceScale     int
	thinking       bool
	undoStack      []undoSnapshot
}

type aiMoveMsg struct {
	move chess.Move
	ok   bool
}

func newModel(aiMode aiSide, aiDepth int, useUnicode bool, pieceScale int) model {
	state := chess.InitialState()
	input := textinput.New()
	input.Placeholder = "e2e4, help, resign"
	input.Prompt = "> "
	input.CharLimit = 32
	input.Width = 24
	input.PromptStyle = promptTextStyle
	input.TextStyle = promptTextStyle
	input.PlaceholderStyle = metaStyle
	input.Focus()
	return model{
		state:        state,
		aiMode:       aiMode,
		aiDepth:      aiDepth,
		repetitions:  map[string]int{chess.PositionKey(state): 1},
		moveHistory:  make([]string, 0, 64),
		rawMoves:     make([]chess.Move, 0, 64),
		input:        input,
		history:      make([]string, 0, 32),
		historyIndex: -1,
		perspective:  playerPerspective(aiMode),
		useUnicode:   useUnicode,
		pieceScale:   pieceScale,
		thinking:     aiControls(aiMode, state.Turn()),
	}
}

func (m model) Init() tea.Cmd {
	if aiControls(m.aiMode, m.state.Turn()) {
		return aiMoveCmd(m.state, m.aiDepth)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case aiMoveMsg:
		m.thinking = false
		if m.gameOver {
			return m, nil
		}
		if !msg.ok {
			m.applyGameOver()
			return m, nil
		}
		cmd := m.applyMove(msg.move, moveSourceAI)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "p":
			m.useUnicode = !m.useUnicode
			return m, nil
		case "b":
			m.cyclePieceScale()
			return m, nil
		case "v":
			m.togglePerspective()
			return m, nil
		case "up", "ctrl+p":
			m.historyPrev()
			return m, nil
		case "down", "ctrl+n":
			m.historyNext()
			return m, nil
		case "esc":
			m.input.SetValue("")
			return m, nil
		case "enter":
			inputValue := strings.TrimSpace(m.input.Value())
			m.pushHistory(inputValue)
			cmd := m.handleInput(inputValue)
			m.input.SetValue("")
			return m, cmd
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m model) View() string {
	statusParts := make([]string, 0, 4)
	if m.result != "" {
		statusParts = append(statusParts, m.result)
	}
	if m.errMsg != "" {
		statusParts = append(statusParts, "Error: "+m.errMsg)
	}
	if m.notice != "" {
		statusParts = append(statusParts, m.notice)
	}
	status := buildStatus(m.state, m.hasLastMove, m.lastMove, m.lastMoveSource)
	if status != "" {
		statusParts = append(statusParts, status)
	}

	statusLine := strings.Join(statusParts, " | ")
	prompt := ""
	if m.gameOver {
		prompt = "Game over. Press q to quit."
	} else {
		prompt = m.input.View()
	}

	var lastPtr *chess.Move
	if m.hasLastMove {
		lastPtr = &m.lastMove
	}
	return renderBoard(m.state, lastPtr, statusLine, m.moveHistory, prompt, m.width, m.perspective, m.useUnicode, m.pieceScale, m.thinking)
}

func (m *model) handleInput(input string) tea.Cmd {
	m.errMsg = ""
	m.notice = ""
	if input == "" {
		return nil
	}

	lower := strings.ToLower(input)
	switch lower {
	case "quit", "exit":
		return tea.Quit
	case "help":
		m.notice = "Examples: e2e4, e7e8q, O-O, ai [depth], undo, fen, pgn, new, load <file>"
		return nil
	case "resign":
		m.result = fmt.Sprintf("%s resigns. %s wins.", chess.ColorName(m.state.Turn()), chess.ColorName(-m.state.Turn()))
		m.gameOver = true
		m.input.Blur()
		return nil
	case "undo":
		return m.undoMove()
	case "fen":
		m.notice = chess.ToFEN(m.state)
		return nil
	case "pgn":
		result := m.result
		if result == "" {
			result = "*"
		}
		headers := map[string]string{"Result": result}
		m.notice = chess.FormatPGN(m.rawMoves, chess.InitialState(), headers, result)
		return nil
	case "new":
		return m.newGame()
	}

	fields := strings.Fields(lower)

	if len(fields) >= 2 && fields[0] == "load" {
		return m.loadPGN(strings.TrimSpace(input[len("load "):]))
	}

	if m.gameOver {
		return nil
	}
	if len(fields) > 0 && fields[0] == "ai" {
		depth := m.aiDepth
		if len(fields) > 1 {
			parsed, err := strconv.Atoi(fields[1])
			if err != nil || parsed < 1 || parsed > 4 {
				m.errMsg = "AI depth must be between 1 and 4."
				return nil
			}
			depth = parsed
		}
		m.thinking = true
		return aiMoveCmd(m.state, depth)
	}

	move, err := chess.ParseMove(input, m.state)
	if err != nil {
		m.errMsg = err.Error()
		return nil
	}
	return m.applyMove(move, moveSourceHuman)
}

// currentSnapshot captures a copy of all mutable state needed to restore the current position.
func (m *model) currentSnapshot() undoSnapshot {
	histCopy := make([]string, len(m.moveHistory))
	copy(histCopy, m.moveHistory)
	rawCopy := make([]chess.Move, len(m.rawMoves))
	copy(rawCopy, m.rawMoves)
	return undoSnapshot{
		state:    m.state,
		moveHist: histCopy,
		rawMoves: rawCopy,
		reps:     maps.Clone(m.repetitions),
		lastMove: m.lastMove,
		hasLast:  m.hasLastMove,
	}
}

func (m *model) applyMove(move chess.Move, source string) tea.Cmd {
	m.errMsg = ""
	m.notice = ""

	if source == moveSourceHuman {
		m.undoStack = append(m.undoStack, m.currentSnapshot())
	}

	gm.RecordMove(&m.moveHistory, m.state, move)
	m.rawMoves = append(m.rawMoves, move)
	m.state = chess.ApplyMove(m.state, move)
	m.lastMove = move
	m.hasLastMove = true
	m.lastMoveSource = source
	key := chess.PositionKey(m.state)
	m.repetitions[key]++
	repCount := m.repetitions[key]
	m.notice = gm.ClaimableDraw(m.state, repCount)

	gameOver, result := gm.CheckGameOver(m.state, repCount)
	if gameOver {
		m.result = result
		m.gameOver = true
		m.input.Blur()
		return nil
	}

	if aiControls(m.aiMode, m.state.Turn()) {
		m.thinking = true
		return aiMoveCmd(m.state, m.aiDepth)
	}
	return nil
}

func (m *model) applyGameOver() {
	repCount := m.repetitions[chess.PositionKey(m.state)]
	gameOver, result := gm.CheckGameOver(m.state, repCount)
	if gameOver {
		m.result = result
		m.gameOver = true
		m.input.Blur()
	}
}

func (m *model) pushHistory(entry string) {
	if entry == "" {
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != entry {
		m.history = append(m.history, entry)
	}
	m.historyIndex = -1
	m.historyStash = ""
}

func (m *model) historyPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIndex == -1 {
		m.historyStash = m.input.Value()
		m.historyIndex = len(m.history) - 1
	} else if m.historyIndex > 0 {
		m.historyIndex--
	}
	m.input.SetValue(m.history[m.historyIndex])
	m.input.CursorEnd()
}

func (m *model) historyNext() {
	if m.historyIndex == -1 {
		return
	}
	if m.historyIndex < len(m.history)-1 {
		m.historyIndex++
		m.input.SetValue(m.history[m.historyIndex])
		m.input.CursorEnd()
		return
	}
	m.historyIndex = -1
	m.input.SetValue(m.historyStash)
	m.input.CursorEnd()
	m.historyStash = ""
}

func aiMoveCmd(state chess.GameState, depth int) tea.Cmd {
	return func() tea.Msg {
		move, ok := chess.BestMove(state, depth)
		return aiMoveMsg{move: move, ok: ok}
	}
}

func playerPerspective(aiMode aiSide) int {
	switch aiMode {
	case aiWhite:
		return chess.Black
	case aiBlack:
		return chess.White
	default:
		return chess.White
	}
}

func (m *model) togglePerspective() {
	if m.perspective == chess.White {
		m.perspective = chess.Black
	} else {
		m.perspective = chess.White
	}
}

func (m *model) cyclePieceScale() {
	switch m.pieceScale {
	case 1:
		m.pieceScale = 2
	case 2:
		m.pieceScale = 3
	default:
		m.pieceScale = 1
	}
}

func (m *model) newGame() tea.Cmd {
	state := chess.InitialState()
	m.state = state
	m.repetitions = map[string]int{chess.PositionKey(state): 1}
	m.moveHistory = make([]string, 0, 64)
	m.rawMoves = make([]chess.Move, 0, 64)
	m.undoStack = nil
	m.hasLastMove = false
	m.gameOver = false
	m.result = ""
	m.notice = ""
	m.errMsg = ""
	m.input.Focus()
	if aiControls(m.aiMode, state.Turn()) {
		m.thinking = true
		return aiMoveCmd(state, m.aiDepth)
	}
	return nil
}

func (m *model) loadPGN(path string) tea.Cmd {
	data, err := os.ReadFile(path)
	if err != nil {
		m.errMsg = fmt.Sprintf("load: %v", err)
		return nil
	}
	moves, initialState, _, err := chess.ParsePGN(string(data))
	if err != nil {
		m.errMsg = fmt.Sprintf("load: %v", err)
		return nil
	}

	// Rebuild state and undo stack by replaying all moves.
	m.state = initialState
	m.moveHistory = nil
	m.rawMoves = nil
	m.repetitions = make(map[string]int)
	m.undoStack = nil
	m.gameOver = false
	m.result = ""
	m.hasLastMove = false

	for _, mv := range moves {
		m.undoStack = append(m.undoStack, m.currentSnapshot())

		gm.RecordMove(&m.moveHistory, m.state, mv)
		m.rawMoves = append(m.rawMoves, mv)
		m.state = chess.ApplyMove(m.state, mv)
		m.lastMove = mv
		m.hasLastMove = true

		key := chess.PositionKey(m.state)
		m.repetitions[key]++
	}

	m.notice = fmt.Sprintf("Loaded %d moves from %s. Use undo to step back.", len(moves), path)
	return nil
}

func (m *model) undoMove() tea.Cmd {
	if len(m.undoStack) == 0 {
		m.errMsg = "Nothing to undo."
		return nil
	}
	if m.thinking {
		m.errMsg = "Cannot undo while AI is thinking."
		return nil
	}
	last := len(m.undoStack) - 1
	snap := m.undoStack[last]
	m.undoStack = m.undoStack[:last]
	m.state = snap.state
	m.moveHistory = snap.moveHist
	m.rawMoves = snap.rawMoves
	m.repetitions = snap.reps
	m.lastMove = snap.lastMove
	m.hasLastMove = snap.hasLast
	m.gameOver = false
	m.result = ""
	m.notice = ""
	m.errMsg = ""
	m.input.Focus()
	return nil
}
