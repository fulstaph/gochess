package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fulstaph/gochess/chess"
)

type model struct {
	state          chess.GameState
	aiMode         aiSide
	aiDepth        int
	lastMove       chess.Move
	hasLastMove    bool
	lastMoveSource string
	repetitions    map[string]int
	moveHistory    []string
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
		input:        input,
		history:      make([]string, 0, 32),
		historyIndex: -1,
		perspective:  playerPerspective(aiMode),
		useUnicode:   useUnicode,
		pieceScale:   pieceScale,
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
		if m.gameOver {
			return m, nil
		}
		if !msg.ok {
			m.applyGameOver()
			return m, nil
		}
		cmd := m.applyMove(msg.move, "AI")
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
	return renderBoard(m.state, lastPtr, statusLine, m.moveHistory, prompt, m.width, m.perspective, m.useUnicode, m.pieceScale)
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
		m.notice = "Examples: e2e4, e7e8q, O-O, ai [depth]"
		return nil
	case "resign":
		m.result = fmt.Sprintf("%s resigns. %s wins.", chess.ColorName(m.state.Turn()), chess.ColorName(-m.state.Turn()))
		m.gameOver = true
		m.input.Blur()
		return nil
	}

	if m.gameOver {
		return nil
	}

	fields := strings.Fields(lower)
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
		move, ok := chess.BestMove(m.state, depth)
		if !ok {
			m.applyGameOver()
			return nil
		}
		return m.applyMove(move, "AI")
	}

	move, err := chess.ParseMove(input, m.state)
	if err != nil {
		m.errMsg = err.Error()
		return nil
	}
	return m.applyMove(move, "You")
}

func (m *model) applyMove(move chess.Move, source string) tea.Cmd {
	m.errMsg = ""
	m.notice = ""
	recordMove(&m.moveHistory, m.state, move)
	m.state = chess.ApplyMove(m.state, move)
	m.lastMove = move
	m.hasLastMove = true
	m.lastMoveSource = source
	repCount := updateRepetition(m.repetitions, m.state)
	m.notice = claimableDrawNotice(m.state, repCount)

	gameOver, result := checkGameOver(m.state, repCount)
	if gameOver {
		m.result = result
		m.gameOver = true
		m.input.Blur()
		return nil
	}

	if aiControls(m.aiMode, m.state.Turn()) {
		return aiMoveCmd(m.state, m.aiDepth)
	}
	return nil
}

func (m *model) applyGameOver() {
	repCount := m.repetitions[chess.PositionKey(m.state)]
	gameOver, result := checkGameOver(m.state, repCount)
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
